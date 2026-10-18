package util

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"strings"

	"github.com/deposist/s-ui-x-extended/util/common"
	"github.com/sagernet/sing-box/parser/clash"
)

func subscriptionURL(raw string) string {
	raw = strings.TrimSpace(raw)
	for _, prefix := range []string{"happ://add/", "hiddify://import/"} {
		if strings.HasPrefix(raw, prefix) {
			return strings.TrimPrefix(raw, prefix)
		}
	}
	return raw
}

func decodeSubscriptionBody(data string) string {
	for _, encoding := range []*base64.Encoding{base64.StdEncoding, base64.RawStdEncoding, base64.URLEncoding, base64.RawURLEncoding} {
		if decoded, err := encoding.DecodeString(data); err == nil {
			return string(decoded)
		}
	}
	return data
}

func structuredSubscription(data string) ([]map[string]interface{}, bool, error) {
	var candidates []map[string]interface{}
	if (strings.HasPrefix(data, "{") || strings.HasPrefix(data, "[")) && !strings.Contains(data, "\"proxies\"") {
		var value any
		if err := json.Unmarshal([]byte(data), &value); err != nil {
			return nil, true, common.NewError("invalid subscription JSON")
		}
		var collect func(any)
		collect = func(v any) {
			switch v := v.(type) {
			case []any:
				for _, item := range v {
					collect(item)
				}
			case map[string]any:
				if nodes, ok := v["outbounds"]; ok {
					collect(nodes)
				} else if typ, _ := v["type"].(string); typ != "" {
					candidates = append(candidates, v)
				}
			}
		}
		collect(value)
	} else if strings.Contains(data, "proxies:") || strings.Contains(data, "\"proxies\"") {
		outbounds, err := clash.ParseClashSubscription(context.Background(), data)
		if err != nil {
			return nil, true, common.NewError("invalid Clash/Mihomo subscription")
		}
		for _, outbound := range outbounds {
			raw, err := outbound.MarshalJSONContext(context.Background())
			if err != nil {
				return nil, true, err
			}
			var node map[string]interface{}
			if err = json.Unmarshal(raw, &node); err != nil {
				return nil, true, err
			}
			candidates = append(candidates, node)
		}
	} else {
		return nil, false, nil
	}
	nodes := make([]map[string]interface{}, 0, len(candidates))
	for _, node := range candidates {
		typ, _ := node["type"].(string)
		switch typ {
		case "", "direct", "block", "dns", "selector", "urltest", "fallback", "failover", "bond":
			continue
		}
		nodes = append(nodes, node)
	}
	if len(nodes) == 0 {
		return nil, true, common.NewError("subscription contains no supported proxy servers")
	}
	return nodes, true, nil
}
