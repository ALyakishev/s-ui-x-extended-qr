package util

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSubscriptionHWIDRedirectOrigin(t *testing.T) {
	t.Setenv("SUI_ALLOW_PRIVATE_SUB_URLS", "true")
	headers := map[string]string{
		"X-HWID": "device-redirect-test", "X-Device-OS": "Android",
		"X-Device-Model": "Samsung Galaxy S24", "X-Ver-OS": "14",
	}
	received := make(chan http.Header, 2)
	foreign := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received <- r.Header.Clone()
		if r.URL.Path == "/step" {
			http.Redirect(w, r, "/final", http.StatusFound)
			return
		}
		_, _ = w.Write([]byte("ok"))
	}))
	defer foreign.Close()
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/same":
			http.Redirect(w, r, "/final", http.StatusFound)
		case "/foreign":
			http.Redirect(w, r, foreign.URL+"/step", http.StatusFound)
		default:
			received <- r.Header.Clone()
			_, _ = w.Write([]byte("ok"))
		}
	}))
	defer source.Close()
	for _, tc := range []struct {
		name string
		path string
		keep bool
		hops int
	}{
		{"same origin", "/same", true, 1},
		{"different port and subsequent redirect", "/foreign", false, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req, err := http.NewRequest(http.MethodGet, source.URL+tc.path, nil)
			if err != nil {
				t.Fatal(err)
			}
			for key, value := range headers {
				req.Header.Set(key, value)
			}
			response, err := getExternalHTTPClient().Do(req)
			if err != nil {
				t.Fatal(err)
			}
			defer response.Body.Close()
			if _, err := io.Copy(io.Discard, response.Body); err != nil {
				t.Fatal(err)
			}
			for range tc.hops {
				got := <-received
				for key, value := range headers {
					want := ""
					if tc.keep {
						want = value
					}
					if got.Get(key) != want {
						t.Fatalf("redirect disclosed or dropped %s: got %q, want %q", key, got.Get(key), want)
					}
				}
			}
		})
	}
}

func TestSubscriptionHWIDCanBeEditedAndDisabled(t *testing.T) {
	t.Setenv("SUI_ALLOW_PRIVATE_SUB_URLS", "true")
	received := make(chan http.Header, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received <- r.Header.Clone()
		_, _ = w.Write([]byte(`{"outbounds":[{"type":"socks","tag":"A","server":"one.example","server_port":1080}]}`))
	}))
	defer srv.Close()
	for _, hwid := range []string{"provider-specific-device-id", ""} {
		nodes, err := GetExternalSubContext(context.Background(), srv.URL, "Happ", SubscriptionDevice{HWID: hwid})
		if err != nil {
			t.Fatal(err)
		}
		if nodes[0]["tag"] != "A" {
			t.Fatalf("unexpected protected subscription: %#v", nodes)
		}
		headers := <-received
		for _, key := range []string{"X-Device-OS", "X-Device-Model", "X-Ver-OS"} {
			if len(headers.Values(key)) != 0 {
				t.Fatalf("empty device metadata still sent %s", key)
			}
		}
		values := headers.Values("X-HWID")
		if hwid == "" {
			if len(values) != 0 {
				t.Fatalf("empty HWID still sent a header: %#v", values)
			}
		} else if len(values) != 1 || values[0] != hwid {
			t.Fatalf("manual HWID not applied: %#v", values)
		}
	}
}
