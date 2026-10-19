package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/deposist/s-ui-x-extended/core"
	"github.com/deposist/s-ui-x-extended/database"
	"github.com/deposist/s-ui-x-extended/database/model"
)

func TestSubscriptionSelectionAndRefresh(t *testing.T) {
	initSettingTestDB(t)
	t.Setenv("SUI_ALLOW_PRIVATE_SUB_URLS", "true")
	body := `{"outbounds":[{"type":"socks","tag":"B","server":"127.0.0.1","server_port":1082},{"type":"socks","tag":"A","server":"127.0.0.1","server_port":1081}]}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(body)) }))
	defer srv.Close()
	raw, _ := json.Marshal(map[string]any{"type": "subscription", "tag": "selected", "url": srv.URL, "primary": "A", "backup": "B", "nodes": []any{map[string]any{"type": "direct", "tag": "forged"}}})
	var row model.Outbound
	if err := row.UnmarshalJSON(raw); err != nil {
		t.Fatal(err)
	}
	if err := prepareSubscription(context.Background(), &row); err != nil {
		t.Fatal(err)
	}
	var opts subscriptionOptions
	if err := json.Unmarshal(row.Options, &opts); err != nil {
		t.Fatal(err)
	}
	if len(opts.Nodes) != 2 || opts.Nodes[0]["tag"] != "A" {
		t.Fatalf("selected=%#v", opts.Nodes)
	}
	db := database.GetDB()
	if err := db.Create(&row).Error; err != nil {
		t.Fatal(err)
	}
	configs, err := (&OutboundService{}).GetAllConfig(db)
	if err != nil {
		t.Fatal(err)
	}
	assembled, _ := json.Marshal(map[string]any{"log": map[string]any{"disabled": true}, "outbounds": configs})
	if strings.Contains(string(assembled), srv.URL) || strings.Contains(string(assembled), `"type":"subscription"`) {
		t.Fatalf("metadata leaked: %s", assembled)
	}
	if err := core.ValidateConfig(assembled); err != nil {
		t.Fatal(err)
	}
	groups, err := LoadFailoverGroups(db)
	if err != nil || len(groups) != 1 || len(groups[0].Members) != 2 {
		t.Fatalf("groups=%#v err=%v", groups, err)
	}
	body = `{"outbounds":[{"type":"socks","tag":"A","server":"127.0.0.1","server_port":2081},{"type":"socks","tag":"B","server":"127.0.0.1","server_port":2082}]}`
	if err := (&ConfigService{}).RefreshSubscriptions(context.Background(), true); err != nil {
		t.Fatal(err)
	}
	var changed model.Outbound
	if err := db.First(&changed, row.Id).Error; err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(changed.Options), "2081") {
		t.Fatalf("not refreshed: %s", changed.Options)
	}
	body = `{"outbounds":[{"type":"socks","tag":"C","server":"127.0.0.1","server_port":3081}]}`
	if err := (&ConfigService{}).RefreshSubscriptions(context.Background(), true); err != nil {
		t.Fatal(err)
	}
	if err := db.First(&changed, row.Id).Error; err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(changed.Options), "2081") || !strings.Contains(string(changed.Options), "last_error") {
		t.Fatalf("lost last known config: %s", changed.Options)
	}
}

func TestSubscriptionRejectsAmbiguousAndUnsafeNodes(t *testing.T) {
	t.Setenv("SUI_ALLOW_PRIVATE_SUB_URLS", "true")
	for name, body := range map[string]string{
		"duplicate":  `{"outbounds":[{"type":"socks","tag":"A","server":"one.example","server_port":1080},{"type":"socks","tag":"A","server":"two.example","server_port":1080}]}`,
		"detour":     `{"outbounds":[{"type":"socks","tag":"A","server":"one.example","server_port":1080,"detour":"direct"}]}`,
		"local file": `{"outbounds":[{"type":"trojan","tag":"A","server":"one.example","server_port":443,"password":"x","tls":{"enabled":true,"certificate_path":"secrets.txt"}}]}`,
	} {
		t.Run(name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(body)) }))
			defer srv.Close()
			options, _ := json.Marshal(map[string]any{"url": srv.URL, "primary": "A"})
			row := model.Outbound{Type: "subscription", Tag: "s", Options: options}
			if err := prepareSubscription(context.Background(), &row); err == nil {
				t.Fatal("unsafe selection accepted")
			}
		})
	}
}

func TestSubscriptionHWIDPersistsAcrossRefresh(t *testing.T) {
	initSettingTestDB(t)
	t.Setenv("SUI_ALLOW_PRIVATE_SUB_URLS", "true")
	const hwid = "f8d88d1c-3020-4e5a-bf5c-390335104958"
	requests := make(chan string, 3)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		value := r.Header.Get("X-HWID")
		requests <- value
		if value != hwid {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_, _ = w.Write([]byte(`{"outbounds":[{"type":"socks","tag":"A","server":"one.example","server_port":1080}]}`))
	}))
	defer srv.Close()
	options, err := json.Marshal(map[string]any{"url": srv.URL, "primary": "A", "hwid": hwid})
	if err != nil {
		t.Fatal(err)
	}
	row := model.Outbound{Type: SubscriptionType, Tag: "hwid-subscription", Options: options}
	if err := prepareSubscription(context.Background(), &row); err != nil {
		t.Fatalf("HWID-protected subscription rejected: %v; received HWID %q", err, <-requests)
	}
	if got := <-requests; got != hwid {
		t.Fatalf("initial HWID = %q", got)
	}
	db := database.GetDB()
	if err := db.Create(&row).Error; err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := (&ConfigService{}).RefreshSubscriptions(context.Background(), true); err != nil {
			t.Fatal(err)
		}
		if got := <-requests; got != hwid {
			t.Fatalf("refresh HWID = %q", got)
		}
		var saved model.Outbound
		if err := db.First(&saved, row.Id).Error; err != nil {
			t.Fatal(err)
		}
		var stored map[string]any
		if err := json.Unmarshal(saved.Options, &stored); err != nil {
			t.Fatal(err)
		}
		if stored["hwid"] != hwid || stored["last_error"] != nil {
			t.Fatalf("HWID or refresh state lost: %#v", stored)
		}
	}
	configs, err := (&OutboundService{}).GetAllConfig(db)
	if err != nil {
		t.Fatal(err)
	}
	assembled, err := json.Marshal(configs)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(assembled), hwid) {
		t.Fatal("subscription HWID leaked into core configuration")
	}
}
