package util

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/deposist/s-ui-x-extended/database/model"
)

// Independent reader: do not call encoder helpers to interpret lengths or tags.
func decodeTT(t *testing.T, uri string) map[uint64]string {
	t.Helper()
	if !strings.HasPrefix(uri, "tt://?") || strings.ContainsAny(strings.TrimPrefix(uri, "tt://?"), "=+/") {
		t.Fatal("invalid native link encoding")
	}
	data, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(uri, "tt://?"))
	if err != nil {
		t.Fatal(err)
	}
	read := func() uint64 {
		if len(data) == 0 {
			t.Fatal("truncated integer")
		}
		size := 1 << (data[0] >> 6)
		if size > 4 || len(data) < size {
			t.Fatal("invalid integer size")
		}
		n := uint64(data[0] & 63)
		for _, b := range data[1:size] {
			n = n*256 + uint64(b)
		}
		data = data[size:]
		return n
	}
	fields := map[uint64]string{}
	var order []uint64
	for len(data) > 0 {
		tag := read()
		length := read()
		if length > uint64(len(data)) {
			t.Fatal("truncated TLV value")
		}
		if _, exists := fields[tag]; exists {
			t.Fatal("duplicate TLV tag")
		}
		fields[tag] = string(data[:length])
		data = data[length:]
		order = append(order, tag)
	}
	if !reflect.DeepEqual(order, []uint64{0, 1, 2, 5, 6, 9, 12}) {
		t.Fatalf("unexpected TLV order: %v", order)
	}
	return fields
}

func TestTrustTunnelReferenceVector(t *testing.T) {
	link, err := trustTunnelURI("vpn.example", "192.0.2.1:8443", "alice", "p@ss", "Тест")
	if err != nil {
		t.Fatal(err)
	}
	// Produced by the independent Python reference algorithm in the feature spec.
	const want = "tt://?AAEBAQt2cG4uZXhhbXBsZQIOMTkyLjAuMi4xOjg0NDMFBWFsaWNlBgRwQHNzCQEBDAjQotC10YHRgg"
	if link != want {
		t.Fatal("reference vector mismatch")
	}
	fields := decodeTT(t, link)
	expected := map[uint64]string{0: "\x01", 1: "vpn.example", 2: "192.0.2.1:8443", 5: "alice", 6: "p@ss", 9: "\x01", 12: "Тест"}
	if !reflect.DeepEqual(fields, expected) {
		t.Fatal("decoded TLV differs from input")
	}
}

func TestTrustTunnelVarintBoundaries(t *testing.T) {
	for _, tc := range []struct {
		n   uint64
		hex string
	}{
		{0, "00"}, {1, "01"}, {63, "3f"}, {64, "4040"}, {16383, "7fff"}, {16384, "80004000"}, {1073741823, "bfffffff"},
	} {
		got, err := appendTrustTunnelVarint(nil, tc.n)
		if err != nil || hex.EncodeToString(got) != tc.hex {
			t.Fatalf("varint boundary %d", tc.n)
		}
	}
	for _, n := range []uint64{1 << 30, 1 << 32, ^uint64(0)} {
		if _, err := appendTrustTunnelVarint(nil, n); err == nil {
			t.Fatalf("accepted unsupported varint %d", n)
		}
	}
}

func TestTrustTunnelLongUTF8RoundTrip(t *testing.T) {
	for _, n := range []int{0, 63, 64, 16383, 16384} {
		value := strings.Repeat("я", n)
		uri, err := trustTunnelURI("vpn.example", "[2001:db8::1]:443", "用户", " p?#&=+/ ", value)
		if err != nil {
			t.Fatal(err)
		}
		fields := decodeTT(t, uri)
		if fields[12] != value || fields[5] != "用户" || fields[6] != " p?#&=+/ " {
			t.Fatal("UTF-8 round trip mismatch")
		}
	}
}

func ttInbound() *model.Inbound {
	return &model.Inbound{Type: "trusttunnel", Tag: "TT", Options: json.RawMessage(`{"listen_port":9443}`), Addrs: json.RawMessage(`[]`), TlsId: 1, Tls: &model.Tls{
		Server: json.RawMessage(`{"enabled":true,"server_name":"vpn.example"}`), Client: json.RawMessage(`{}`),
	}}
}

func TestTrustTunnelLinkGenerator(t *testing.T) {
	in := ttInbound()
	in.Addrs = json.RawMessage(`[
 {"server":"192.0.2.1","server_port":443,"remark":" v4"},
 {"server":"2001:db8::1","server_port":8443,"remark":" v6","tls":{"server_name":"override.example"}},
 {"server":"[2001:db8::2]","server_port":10443},
 {"server":"edge.example","server_port":9443}
 ]`)
	before, _ := json.Marshal(in)
	links := LinkGenerator(json.RawMessage(`{"trusttunnel":{"name":"alice","password":"p?#ä"}}`), in, "unused.example")
	if len(links) != 4 {
		t.Fatalf("expected four links, got %d", len(links))
	}
	for idx, address := range []string{"192.0.2.1:443", "[2001:db8::1]:8443", "[2001:db8::2]:10443", "edge.example:9443"} {
		fields := decodeTT(t, links[idx])
		hostname := "vpn.example"
		if idx == 1 {
			hostname = "override.example"
		}
		if fields[1] != hostname || fields[2] != address || fields[5] != "alice" || fields[6] != "p?#ä" {
			t.Fatalf("incorrect address/credential mapping at index %d", idx)
		}
	}
	if decodeTT(t, links[0])[12] != "TT v4" {
		t.Fatal("inbound/address remark lost")
	}
	after, _ := json.Marshal(in)
	if !bytes.Equal(before, after) {
		t.Fatal("link generation mutated inbound")
	}
}

func TestTrustTunnelFallbackAndMissingConfiguration(t *testing.T) {
	cfg := json.RawMessage(`{"trusttunnel":{"name":"alice","password":"pw"}}`)
	for _, host := range []string{"192.0.2.1", "2001:db8::1", "[2001:db8::1]", "edge.example"} {
		in := ttInbound()
		in.TlsId = 0
		in.Tls = nil
		links := LinkGenerator(cfg, in, host)
		if len(links) != 1 {
			t.Fatal("fallback host produced no link")
		}
		fields := decodeTT(t, links[0])
		if fields[1] != shareHost(host) || fields[12] != "TT" {
			t.Fatal("incorrect fallback hostname or remark")
		}
	}
	for _, cfg := range []string{`{}`, `{"trusttunnel":{"name":"alice"}}`, `{"trusttunnel":{"password":"p"}}`, `{"trusttunnel":null}`, `invalid`} {
		if len(LinkGenerator(json.RawMessage(cfg), ttInbound(), "edge.example")) != 0 {
			t.Fatal("accepted missing credentials")
		}
	}
	for _, addr := range []string{
		`{"server":"","server_port":443}`, `{"server":"edge.example","server_port":0}`,
		`{"server":"edge.example","server_port":65536}`, `{"server":"edge.example","server_port":-1}`,
		`{"server":"edge.example","server_port":443.5}`, `{"server":"edge.example","server_port":"443"}`,
		`{"server":"edge.example:443","server_port":443}`, `{"server":"https://edge.example","server_port":443}`,
	} {
		in := ttInbound()
		in.Addrs = json.RawMessage("[" + addr + "]")
		if len(LinkGenerator(cfg, in, "fallback.example")) != 0 {
			t.Fatalf("accepted malformed address %s", addr)
		}
	}
}

// Exercise a separately selected TLS row, not a preassembled addr.tls map.
// prepareTls historically drops the row when Client is absent, or overwrites
// a client-side SNI with an empty server-side value.
func TestTrustTunnelSelectedTLSHostname(t *testing.T) {
	for _, tc := range []struct {
		name, serverTLS, clientTLS, addrs, hostname, address string
	}{
		{"server SNI", `{"enabled":true,"server_name":"tls.example"}`, `{}`, `[{"server":"192.0.2.1","server_port":8443}]`, "tls.example", "192.0.2.1:8443"},
		{"absent client blob", `{"enabled":true,"server_name":"tls.example"}`, ``, `[{"server":"192.0.2.1","server_port":8443}]`, "tls.example", "192.0.2.1:8443"},
		{"null client blob", `{"enabled":true,"server_name":"tls.example"}`, `null`, `[{"server":"192.0.2.1","server_port":8443}]`, "tls.example", "192.0.2.1:8443"},
		{"empty server SNI", `{"enabled":true,"server_name":""}`, `{"server_name":"client.example"}`, `[{"server":"192.0.2.1","server_port":8443}]`, "client.example", "192.0.2.1:8443"},
		{"server SNI priority", `{"enabled":true,"server_name":"tls.example"}`, `{"server_name":"client.example"}`, `[{"server":"192.0.2.1","server_port":8443}]`, "tls.example", "192.0.2.1:8443"},
		{"address override", `{"enabled":true,"server_name":"tls.example"}`, ``, `[{"server":"2001:db8::1","server_port":10443,"tls":{"server_name":"override.example"}}]`, "override.example", "[2001:db8::1]:10443"},
		{"explicit empty override", `{"enabled":true,"server_name":"tls.example"}`, `{}`, `[{"server":"192.0.2.1","server_port":8443,"tls":{"server_name":""}}]`, "192.0.2.1", "192.0.2.1:8443"},
		{"unrelated address override", `{"enabled":true,"server_name":"tls.example"}`, ``, `[{"server":"192.0.2.1","server_port":8443,"tls":{"insecure":true}}]`, "tls.example", "192.0.2.1:8443"},
		{"no hostname", `{"enabled":true}`, `{}`, `[{"server":"192.0.2.1","server_port":8443}]`, "192.0.2.1", "192.0.2.1:8443"},
		{"implicit address", `{"enabled":true,"server_name":"tls.example"}`, ``, `[]`, "tls.example", "192.0.2.1:9443"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in := ttInbound()
			in.Tls = &model.Tls{Id: 7, Name: "display label is not SNI", Server: json.RawMessage(tc.serverTLS), Client: json.RawMessage(tc.clientTLS)}
			in.TlsId = in.Tls.Id
			in.Addrs = json.RawMessage(tc.addrs)
			beforeServer, beforeClient, beforeAddrs := string(in.Tls.Server), string(in.Tls.Client), string(in.Addrs)
			links := LinkGenerator(json.RawMessage(`{"trusttunnel":{"name":"alice","password":"synthetic-password"}}`), in, "192.0.2.1")
			if len(links) != 1 {
				t.Fatalf("expected one link, got %d", len(links))
			}
			fields := decodeTT(t, links[0])
			if fields[1] != tc.hostname {
				t.Errorf("TLS hostname = %q, want %q", fields[1], tc.hostname)
			}
			if fields[2] != tc.address {
				t.Errorf("connection address = %q, want %q", fields[2], tc.address)
			}
			if string(in.Tls.Server) != beforeServer || string(in.Tls.Client) != beforeClient || string(in.Addrs) != beforeAddrs {
				t.Fatal("source TLS/address data mutated")
			}
		})
	}
}
