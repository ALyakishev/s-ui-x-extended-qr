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
