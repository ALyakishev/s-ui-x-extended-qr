package service

import (
	"encoding/json"
	"fmt"
	"reflect"
	"testing"

	"github.com/deposist/s-ui-x-extended/database"
	"github.com/deposist/s-ui-x-extended/database/model"
)

func TestTrustTunnelClientLinksBackfill(t *testing.T) {
	initSettingTestDB(t)
	db := database.GetDB()
	inbound := model.Inbound{Type: "trusttunnel", Tag: "trusttunnel-test",
		Addrs:   json.RawMessage(`[{"server":"192.0.2.1","server_port":8443},{"server":"2001:db8::1","server_port":443}]`),
		Options: json.RawMessage(`{"listen_port":8443}`),
	}
	if err := db.Create(&inbound).Error; err != nil {
		t.Fatal(err)
	}
	client := model.Client{Enable: true, Name: "alice", Inbounds: json.RawMessage(fmt.Sprintf("[%d]", inbound.Id)),
		Config: json.RawMessage(`{"trusttunnel":{"name":"alice","password":"existing-password"}}`),
		Links:  json.RawMessage(`[{"type":"remote","remark":"manual","uri":"https://example.com/sub"}]`),
	}
	if err := db.Create(&client).Error; err != nil {
		t.Fatal(err)
	}
	originalConfig := string(client.Config)
	svc := &ClientService{}
	for attempt := 0; attempt < 2; attempt++ {
		if err := svc.RegenerateMissingLocalLinks("fallback.example"); err != nil {
			t.Fatal(err)
		}
		links := clientLinks(t, client.Id)
		if len(links) != 3 || !hasLocalLinkWithPrefix(links, "tt://?") {
			t.Fatal("TrustTunnel links missing from persisted client delivery")
		}
		count := 0
		for _, link := range links {
			if link["type"] == "local" && link["remark"] == inbound.Tag {
				count++
			}
		}
		if count != 2 {
			t.Fatal("expected a local link per address")
		}
		if !reflect.DeepEqual(links[2], map[string]string{"type": "remote", "remark": "manual", "uri": "https://example.com/sub"}) {
			t.Fatal("manual link changed")
		}
		var stored model.Client
		if err := db.First(&stored, client.Id).Error; err != nil {
			t.Fatal(err)
		}
		if string(stored.Config) != originalConfig {
			t.Fatal("existing credentials changed")
		}
	}
}
