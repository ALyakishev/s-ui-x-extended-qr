package util

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestExternalSubscriptionFormats(t *testing.T) {
	t.Setenv("SUI_ALLOW_PRIVATE_SUB_URLS", "true")
	for name, body := range map[string]string{
		"links":      "\ufeff  vless://11111111-1111-1111-1111-111111111111@one.example:443?security=tls#Primary\r\n",
		"base64":     base64.StdEncoding.EncodeToString([]byte("vless://11111111-1111-1111-1111-111111111111@one.example:443?security=tls#Primary")),
		"base64-raw": base64.RawStdEncoding.EncodeToString([]byte("vless://11111111-1111-1111-1111-111111111111@one.example:443?security=tls#Primary\n")),
		"base64-url": base64.RawURLEncoding.EncodeToString([]byte("vless://11111111-1111-1111-1111-111111111111@one.example:443?security=tls#Primary\n#\uffff")),
		"singbox":    `{"outbounds":[{"type":"direct","tag":"direct"},{"type":"socks","tag":"Primary","server":"one.example","server_port":1080}]}`,
		"profiles":   `[{"remarks":"Profile","outbounds":[{"type":"socks","tag":"Primary","server":"one.example","server_port":1080}]}]`,
		"clash":      "proxies:\n  - name: Primary\n    type: socks5\n    server: one.example\n    port: 1080\n",
		"clash-json": `{"proxies":[{"name":"Primary","type":"socks5","server":"one.example","port":1080}]}`,
	} {
		t.Run(name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(body)) }))
			defer srv.Close()
			nodes, err := GetExternalSub(srv.URL)
			if err != nil {
				t.Fatal(err)
			}
			if len(nodes) != 1 || nodes[0]["tag"] != "Primary" || nodes[0]["server"] != "one.example" {
				t.Fatalf("nodes=%#v", nodes)
			}
		})
	}
}

func TestExternalSubscriptionUserAgentAndWrappers(t *testing.T) {
	t.Setenv("SUI_ALLOW_PRIVATE_SUB_URLS", "true")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.UserAgent() != "Hiddify" {
			t.Errorf("user-agent=%q", r.UserAgent())
		}
		_, _ = w.Write([]byte(`{"outbounds":[{"type":"socks","tag":"one","server":"one.example","server_port":1080}]}`))
	}))
	defer srv.Close()
	for _, prefix := range []string{"happ://add/", "hiddify://import/"} {
		if _, err := GetExternalSubWithUserAgent(prefix+srv.URL, "Hiddify"); err != nil {
			t.Fatal(err)
		}
	}
}
