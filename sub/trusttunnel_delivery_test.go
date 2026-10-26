package sub

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/deposist/s-ui-x-extended/database"
	"github.com/deposist/s-ui-x-extended/database/model"
	"github.com/deposist/s-ui-x-extended/service"
	"github.com/deposist/s-ui-x-extended/util"
)

func TestTrustTunnelJSONAndNativeSubscriptions(t *testing.T) {
	initSubTestDB(t)
	ClearSubscriptionOutputCache()
	if _, err := (&service.SettingService{}).GetAllSetting(); err != nil {
		t.Fatal(err)
	}
	db := database.GetDB()
	setSubSetting(t, "subSecretRequired", "false") // Also exercise enabled legacy name URLs.
	tls := model.Tls{Server: json.RawMessage(`{"enabled":true,"server_name":"vpn.example"}`), Client: json.RawMessage(`{"enabled":true,"server_name":"vpn.example"}`)}
	if err := db.Create(&tls).Error; err != nil {
		t.Fatal(err)
	}
	in := model.Inbound{Type: "trusttunnel", Tag: "tt-dual", TlsId: tls.Id, Tls: &tls,
		Options: json.RawMessage(`{"listen_port":8443,"network":["tcp","udp"],"quic":true,"congestion_controller":"bbr"}`),
		Addrs:   json.RawMessage(`[{"server":"192.0.2.1","server_port":8443}]`), OutJson: json.RawMessage("null")}
	if err := util.FillOutJson(&in, "vpn.example"); err != nil {
		t.Fatal(err)
	}
	originalJSON := string(in.OutJson)
	if err := db.Create(&in).Error; err != nil {
		t.Fatal(err)
	}
	config := json.RawMessage(`{"trusttunnel":{"name":"alice","password":"synthetic-password"}}`)
	links := util.LinkGenerator(config, &in, "vpn.example")
	if len(links) != 1 || !strings.HasPrefix(links[0], "tt://?") {
		t.Fatal("native link missing")
	}
	if string(in.OutJson) != originalJSON {
		t.Fatal("native generation changed JSON template")
	}
	linkJSON, err := json.Marshal([]Link{{Type: "local", Remark: in.Tag, Uri: links[0]}})
	if err != nil {
		t.Fatal(err)
	}
	client := model.Client{Enable: true, Name: "alice", SubSecret: "tt-dual-secret", Config: config,
		Inbounds: json.RawMessage(fmt.Sprintf("[%d]", in.Id)), Links: linkJSON}
	if err := db.Create(&client).Error; err != nil {
		t.Fatal(err)
	}

	// Check both legacy name lookup and secret URLs, including the API's format.
	for _, id := range []string{client.SubSecret, client.Name} {
		for _, format := range []string{"", "json"} {
			body, _, err := (&JsonService{}).GetJson(id, format)
			if err != nil {
				t.Fatal(err)
			}
			var cfg struct {
				Outbounds []map[string]interface{} `json:"outbounds"`
			}
			if err := json.Unmarshal([]byte(*body), &cfg); err != nil {
				t.Fatal(err)
			}
			out := findOutbound(cfg.Outbounds, "trusttunnel")
			if out == nil {
				t.Fatal("existing JSON outbound disappeared")
			}
			if out["username"] != "alice" || out["password"] != "synthetic-password" || out["server"] != "192.0.2.1" || out["server_port"] != float64(8443) || out["quic"] != true || out["congestion_controller"] != "bbr" {
				t.Fatal("JSON delivery lost credentials, address or transport")
			}
			tls, ok := out["tls"].(map[string]interface{})
			if !ok || tls["enabled"] != true || tls["server_name"] != "vpn.example" {
				t.Fatal("JSON delivery lost TLS")
			}
		}
	}
	// Display suffixes must not corrupt the native binary payload, in either
	// plaintext or Base64 subscription output.
	for _, encoded := range []bool{false, true} {
		ClearSubscriptionOutputCache()
		setSubSetting(t, "subEncode", fmt.Sprint(encoded))
		setSubSetting(t, "subNameInRemark", "true")
		setSubSetting(t, "subShowInfo", "true")
		body, _, err := (&SubService{}).GetSubs(client.SubSecret)
		if err != nil {
			t.Fatal(err)
		}
		got := *body
		if encoded {
			decoded, err := base64.StdEncoding.DecodeString(got)
			if err != nil {
				t.Fatal(err)
			}
			got = string(decoded)
		}
		if got != links[0] {
			t.Fatal("native subscription payload changed by display options")
		}
	}
	var stored model.Client
	if err := db.First(&stored, client.Id).Error; err != nil {
		t.Fatal(err)
	}
	if string(stored.Config) != string(config) {
		t.Fatal("delivery changed existing client configuration")
	}
}
