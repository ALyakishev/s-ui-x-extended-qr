package util

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/deposist/s-ui-x-extended/logger"
	"github.com/deposist/s-ui-x-extended/util/common"
	"github.com/deposist/s-ui-x-extended/util/ssrf"
)

const maxExternalSubBytes = 4 << 20

var (
	externalHTTPClientOnce sync.Once
	externalHTTPClient     *http.Client
)

// errBlockedExternalAddress is returned by the dialer hook when the resolved
// IP address points at a private/loopback/etc. range while
// SUI_ALLOW_PRIVATE_SUB_URLS is not enabled.
var errBlockedExternalAddress = common.NewError("private url host is not allowed")

// allowPrivateExternalURLs reports whether SUI_ALLOW_PRIVATE_SUB_URLS opts the
// process out of private-address filtering for external subscription URLs.
func allowPrivateExternalURLs() bool {
	return os.Getenv("SUI_ALLOW_PRIVATE_SUB_URLS") == "true"
}

// getExternalHTTPClient returns a process-wide HTTP client that re-validates
// every dialed address against isBlockedExternalAddr. Re-validating at dial
// time prevents DNS-rebinding attacks where validateExternalURL sees a public
// address but the subsequent connection is steered to a private one.
func getExternalHTTPClient() *http.Client {
	externalHTTPClientOnce.Do(func() {
		dialer := &net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 30 * time.Second,
		}
		transport := &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				if allowPrivateExternalURLs() {
					return dialer.DialContext(ctx, network, addr)
				}
				host, port, err := net.SplitHostPort(addr)
				if err != nil {
					return nil, err
				}
				if addr, err := netip.ParseAddr(host); err == nil {
					if isBlockedExternalAddr(addr) {
						return nil, errBlockedExternalAddress
					}
					return dialer.DialContext(ctx, network, net.JoinHostPort(addr.String(), port))
				}
				ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
				if err != nil {
					return nil, err
				}
				var lastErr error
				for _, ip := range ips {
					addr, ok := netip.AddrFromSlice(ip.IP)
					if !ok || isBlockedExternalAddr(addr) {
						lastErr = errBlockedExternalAddress
						continue
					}
					conn, err := dialer.DialContext(ctx, network, net.JoinHostPort(addr.String(), port))
					if err == nil {
						return conn, nil
					}
					lastErr = err
				}
				if lastErr == nil {
					lastErr = errBlockedExternalAddress
				}
				return nil, lastErr
			},
			TLSHandshakeTimeout:   10 * time.Second,
			ResponseHeaderTimeout: 10 * time.Second,
			IdleConnTimeout:       90 * time.Second,
			MaxIdleConns:          10,
			MaxIdleConnsPerHost:   2,
		}
		externalHTTPClient = &http.Client{
			Timeout:   15 * time.Second,
			Transport: transport,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 10 {
					return common.NewError("too many redirects")
				}
				return validateExternalURL(req.URL.String())
			},
		}
	})
	return externalHTTPClient
}

func GetExternalLink(rawURL string) (string, error) {
	return getExternalLink(context.Background(), rawURL, "")
}

func getExternalLink(ctx context.Context, rawURL, userAgent string) (string, error) {
	rawURL = subscriptionURL(rawURL)
	if err := validateExternalURL(rawURL); err != nil {
		return "", common.NewError("invalid or blocked subscription URL")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", common.NewError("invalid subscription request")
	}
	if userAgent != "" {
		req.Header.Set("User-Agent", userAgent)
	}
	response, err := getExternalHTTPClient().Do(req)
	if err != nil {
		if errors.Is(err, errBlockedExternalAddress) {
			return "", errBlockedExternalAddress
		}
		return "", common.NewError("external subscription request failed")
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return "", common.NewErrorf("unexpected status code: %d", response.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(response.Body, maxExternalSubBytes+1))
	if err != nil {
		logger.Warning("sub: Error reading response body:", err)
		return "", err
	}
	if len(body) > maxExternalSubBytes {
		return "", common.NewError("response is too large")
	}

	data := strings.TrimSpace(strings.TrimPrefix(string(body), "\ufeff"))
	return strings.TrimSpace(strings.TrimPrefix(decodeSubscriptionBody(data), "\ufeff")), nil
}

func GetExternalSub(url string) ([]map[string]interface{}, error) {
	return GetExternalSubWithUserAgent(url, "")
}

func GetExternalSubWithUserAgent(rawURL, userAgent string) ([]map[string]interface{}, error) {
	return GetExternalSubContext(context.Background(), rawURL, userAgent)
}

func GetExternalSubContext(ctx context.Context, url, userAgent string) ([]map[string]interface{}, error) {
	var err error
	var result []map[string]interface{}

	if len(url) == 0 {
		return nil, common.NewError("no url")
	}

	data, err := getExternalLink(ctx, url, userAgent)
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return nil, common.NewError("no result")
	}

	if nodes, structured, err := structuredSubscription(data); structured {
		return nodes, err
	}
	// if data is a text
	links := strings.Split(data, "\n")
	skipReasons := make(map[string]struct{})
	skipped := 0
	for _, link := range links {
		link = strings.TrimSpace(link)
		if link == "" || strings.HasPrefix(link, "#") {
			continue
		}
		linkToJson, _, err := GetOutbound(link, 0)
		if err != nil {
			// Report why a link was dropped. A link the operator pasted and that
			// vanished from the subscription without a word is indistinguishable
			// from one the panel never received.
			skipped++
			skipReasons[err.Error()] = struct{}{}
			continue
		}
		result = append(result, *linkToJson)
	}
	if len(result) == 0 {
		if skipped > 0 {
			return nil, common.NewErrorf("no result: %s", strings.Join(sortedKeys(skipReasons), "; "))
		}
		return nil, common.NewError("no result")
	}
	if skipped > 0 {
		logger.Warningf("sub: skipped %d external link(s): %s", skipped, strings.Join(sortedKeys(skipReasons), "; "))
	}
	return result, nil
}

// sortedKeys returns the map keys in a stable order so the reported reasons (and
// the tests asserting them) do not depend on map iteration order.
func sortedKeys(m map[string]struct{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func validateExternalURL(rawURL string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return err
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return common.NewError("unsupported url scheme")
	}
	host := parsed.Hostname()
	if host == "" {
		return common.NewError("missing url host")
	}
	if strings.EqualFold(host, "localhost") {
		return common.NewError("localhost url is not allowed")
	}
	if allowPrivateExternalURLs() {
		return nil
	}
	if addr, err := netip.ParseAddr(host); err == nil {
		if isBlockedExternalAddr(addr) {
			return common.NewError("private url host is not allowed")
		}
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	addrs, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return err
	}
	if len(addrs) == 0 {
		return common.NewError("url host did not resolve")
	}
	for _, ipAddr := range addrs {
		addr, ok := netip.AddrFromSlice(ipAddr.IP)
		if !ok || isBlockedExternalAddr(addr) {
			return common.NewError("private url host is not allowed")
		}
	}
	return nil
}

func isBlockedExternalAddr(addr netip.Addr) bool {
	// Delegate to the central SSRF validator so the external-subscription fetch
	// path enforces the SAME policy as every other outbound integration —
	// including the CGNAT 100.64.0.0/10, 192.0.0.0/24, 198.18.0.0/15, 240.0.0.0/4
	// and IPv6 special ranges this local list previously omitted, plus the
	// IsGlobalUnicast requirement.
	return ssrf.IsBlockedAddr(addr)
}
