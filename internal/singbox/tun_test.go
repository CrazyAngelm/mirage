package singbox

import (
	"encoding/json"
	"testing"
)

func TestTunConfigHasTUNInbound(t *testing.T) {
	cfg, err := TunConfig("hysteria2", []map[string]any{
		{"type": "hysteria2", "tag": "hysteria2", "server": "46.225.150.255", "server_port": 443, "password": "secret", "tls": map[string]any{"enabled": true, "insecure": true}},
	})
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal([]byte(cfg), &decoded); err != nil {
		t.Fatal(err)
	}
	inbounds := decoded["inbounds"].([]any)
	if len(inbounds) != 1 {
		t.Fatalf("expected 1 inbound, got %d", len(inbounds))
	}
	inb := inbounds[0].(map[string]any)
	if inb["type"] != "tun" {
		t.Fatalf("inbound type = %v, want tun", inb["type"])
	}
	if inb["auto_route"] != true {
		t.Fatal("expected auto_route=true")
	}
	if inb["strict_route"] != true {
		t.Fatal("expected strict_route=true")
	}
	if _, ok := inb["sniff"]; ok {
		t.Fatalf("sniff should be absent for sing-box 1.13: %+v", inb)
	}
	if _, ok := inb["sniff_override_destination"]; ok {
		t.Fatalf("sniff_override_destination should be absent for sing-box 1.13: %+v", inb)
	}
	dns := decoded["dns"].(map[string]any)
	server := dns["servers"].([]any)[0].(map[string]any)
	if server["type"] != "udp" {
		t.Fatalf("dns server = %+v", server)
	}
	rules := dns["rules"].([]any)
	if len(rules) != 1 {
		t.Fatalf("dns rules = %#v, want one RU-direct rule", rules)
	}
	outbounds := decoded["outbounds"].([]any)
	if len(outbounds) != 3 {
		t.Fatalf("expected 3 outbounds, got %d", len(outbounds))
	}
}

func TestTunConfigRoutesRussianIPDirect(t *testing.T) {
	cfg, err := TunConfig("xray_reality_xhttp", []map[string]any{
		{"type": "socks", "tag": "xray_reality_xhttp", "server": "127.0.0.1", "server_port": 2081},
	})
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal([]byte(cfg), &decoded); err != nil {
		t.Fatal(err)
	}

	route := decoded["route"].(map[string]any)
	assertRussianDirectRuleSet(t, route, "xray_reality_xhttp")
	assertRussianDomainDirectRule(t, route)
	assertRussianDirectRule(t, route)
	assertDNSHijackRule(t, route)
}

func TestTunConfigCanDisableRussianDirect(t *testing.T) {
	cfg, err := TunConfigWithRouting("xray_reality_xhttp", []map[string]any{
		{"type": "socks", "tag": "xray_reality_xhttp", "server": "127.0.0.1", "server_port": 2081},
	}, nil, false, false)
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal([]byte(cfg), &decoded); err != nil {
		t.Fatal(err)
	}
	route := decoded["route"].(map[string]any)
	if _, ok := route["rule_set"]; ok {
		t.Fatalf("rule_set should be absent when russian direct is disabled: %#v", route["rule_set"])
	}
	assertDNSHijackRule(t, route)
}

func assertDNSHijackRule(t *testing.T, route map[string]any) {
	t.Helper()
	rules := route["rules"].([]any)
	for _, rule := range rules {
		item := rule.(map[string]any)
		if item["protocol"] == "dns" && item["action"] == "hijack-dns" {
			return
		}
	}
	t.Fatalf("dns hijack rule missing: %#v", rules)
}
