package util

import (
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"net"
	"strconv"
	"strings"

	"github.com/deposist/s-ui-x-extended/database/model"
)

// prepareTrustTunnelTLS preserves an explicit SNI from the selected TLS row.
// The shared prepareTls may discard it when the optional Client JSON is absent,
// or replace a client SNI with an empty server value. Keep that legacy behavior
// for other protocols; only repair the native TrustTunnel delivery map here.
// Run before merging address TLS so explicit per-address overrides still win,
// including an empty server_name that requests the advertised-host fallback.
func prepareTrustTunnelTLS(t *model.Tls, merged map[string]interface{}) map[string]interface{} {
	if t == nil {
		return merged
	}
	for _, raw := range []json.RawMessage{t.Server, t.Client} {
		var config struct {
			ServerName string `json:"server_name"`
		}
		if json.Unmarshal(raw, &config) != nil || strings.TrimSpace(config.ServerName) == "" {
			continue
		}
		if merged == nil {
			merged = make(map[string]interface{})
		}
		merged["server_name"] = config.ServerName
		break
	}
	return merged
}

// appendTrustTunnelVarint implements the 1/2/4-byte QUIC-style integers used
// by TrustTunnel links. The format does not support the 8-byte QUIC form.
func appendTrustTunnelVarint(dst []byte, n uint64) ([]byte, error) {
	switch {
	case n < 1<<6:
		return append(dst, byte(n)), nil
	case n < 1<<14:
		return binary.BigEndian.AppendUint16(dst, uint16(n)|0x4000), nil
	case n < 1<<30:
		return binary.BigEndian.AppendUint32(dst, uint32(n)|0x80000000), nil
	default:
		return nil, fmt.Errorf("unsupported TrustTunnel varint")
	}
}

func trustTunnelURI(hostname, address, username, password, remark string) (string, error) {
	fields := []struct {
		tag   uint64
		value string
	}{
		{0, "\x01"}, // format version
		{1, hostname},
		{2, address},
		{5, username},
		{6, password},
		{9, "\x01"},
		{12, remark},
	}
	var payload []byte
	for _, field := range fields {
		var err error
		payload, err = appendTrustTunnelVarint(payload, field.tag)
		if err != nil {
			return "", err
		}
		payload, err = appendTrustTunnelVarint(payload, uint64(len(field.value)))
		if err != nil {
			return "", err
		}
		payload = append(payload, field.value...)
	}
	return "tt://?" + base64.RawURLEncoding.EncodeToString(payload), nil
}

// trustTunnelLink emits one native link per advertised server, using the same
// merged TLS/address and per-client credentials as other delivery formats.
// Fail closed for incomplete configurations; never log credentials or URIs.
func trustTunnelLink(user map[string]interface{}, addrs []map[string]interface{}) []string {
	username, password := mapString(user, "name"), mapString(user, "password")
	if username == "" || password == "" {
		return []string{}
	}
	links := []string{}
	for _, addr := range addrs {
		server := shareHost(strings.TrimSpace(mapString(addr, "server")))
		port, ok := asInt(addr["server_port"])
		if !ok || port < 1 || port > 65535 || server == "" {
			continue
		}
		if raw, ok := addr["server_port"].(float64); ok && (math.IsNaN(raw) || math.IsInf(raw, 0) || raw != float64(port)) {
			continue
		}
		// A server field is a host, not a URL or a host:port. IPv6 is
		// accepted both bare and bracketed; JoinHostPort adds brackets once.
		if strings.ContainsAny(server, "/?#@[] \t\r\n") || (strings.Contains(server, ":") && net.ParseIP(server) == nil) {
			continue
		}
		hostname := server
		if tls, ok := addr["tls"].(map[string]interface{}); ok {
			if sni := mapString(tls, "server_name"); sni != "" {
				hostname = sni
			}
		}
		uri, err := trustTunnelURI(hostname, net.JoinHostPort(server, strconv.Itoa(port)), username, password, mapString(addr, "remark"))
		if err == nil {
			links = append(links, uri)
		}
	}
	return links
}
