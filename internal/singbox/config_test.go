package singbox

import (
	"encoding/json"
	"testing"
)

func TestMixedProxyConfigUsesModernDNSAndInbound(t *testing.T) {
	cfg, err := MixedProxyConfig("hysteria2", []map[string]any{
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
	if inbounds[0].(map[string]any)["type"] != "mixed" {
		t.Fatalf("inbound type = %v", inbounds[0])
	}
	dns := decoded["dns"].(map[string]any)
	server := dns["servers"].([]any)[0].(map[string]any)
	if server["type"] != "udp" {
		t.Fatalf("dns server = %+v", server)
	}
}

func TestURLTestIncludesEndpointTags(t *testing.T) {
	cfg, err := MixedProxyConfigURLTest(
		[]map[string]any{{"type": "socks", "tag": "xray_reality_xhttp", "server": "127.0.0.1", "server_port": 2081}},
		[]map[string]any{{"type": "wireguard", "tag": "amneziawg"}},
	)
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal([]byte(cfg), &decoded); err != nil {
		t.Fatal(err)
	}
	outbounds := decoded["outbounds"].([]any)
	var auto map[string]any
	for _, outbound := range outbounds {
		item := outbound.(map[string]any)
		if item["tag"] == "auto" {
			auto = item
		}
	}
	if auto == nil {
		t.Fatal("auto outbound missing")
	}
	if got := auto["outbounds"].([]any); len(got) != 2 || got[0] != "xray_reality_xhttp" || got[1] != "amneziawg" {
		t.Fatalf("auto outbounds = %#v", got)
	}
}

func TestMixedProxyConfigDoesNotUseDeprecatedDNSOutbound(t *testing.T) {
	cfg, err := MixedProxyConfig("xray_reality_xhttp", []map[string]any{
		{"type": "socks", "tag": "xray_reality_xhttp", "server": "127.0.0.1", "server_port": 2081},
	})
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal([]byte(cfg), &decoded); err != nil {
		t.Fatal(err)
	}
	outbounds := decoded["outbounds"].([]any)
	for _, outbound := range outbounds {
		item := outbound.(map[string]any)
		if item["type"] == "dns" {
			t.Fatalf("deprecated dns outbound present: %#v", item)
		}
	}
}

func TestDirectOutboundUsesLocalDomainResolver(t *testing.T) {
	cfg, err := MixedProxyConfig("xray_reality_xhttp", []map[string]any{
		{"type": "socks", "tag": "xray_reality_xhttp", "server": "127.0.0.1", "server_port": 2081},
	})
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal([]byte(cfg), &decoded); err != nil {
		t.Fatal(err)
	}
	outbounds := decoded["outbounds"].([]any)
	for _, outbound := range outbounds {
		item := outbound.(map[string]any)
		if item["tag"] == "direct" {
			if item["domain_resolver"] != "ru-direct-dns" {
				t.Fatalf("direct domain_resolver = %v, want ru-direct-dns", item["domain_resolver"])
			}
			return
		}
	}
	t.Fatal("direct outbound missing")
}

func TestMixedProxyConfigUsesRUDNSFinalForRussianDirect(t *testing.T) {
	cfg, err := MixedProxyConfig("xray_reality_xhttp", []map[string]any{
		{"type": "socks", "tag": "xray_reality_xhttp", "server": "127.0.0.1", "server_port": 2081},
	})
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal([]byte(cfg), &decoded); err != nil {
		t.Fatal(err)
	}
	dns := decoded["dns"].(map[string]any)
	if dns["final"] != "ru-direct-dns" {
		t.Fatalf("dns final = %v, want ru-direct-dns", dns["final"])
	}
}

func TestMixedProxyConfigRoutesRussianIPDirect(t *testing.T) {
	cfg, err := MixedProxyConfig("xray_reality_xhttp", []map[string]any{
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
	if route["final"] != "xray_reality_xhttp" {
		t.Fatalf("route final = %v, want xray_reality_xhttp", route["final"])
	}
	assertRussianDirectRuleSet(t, route, "xray_reality_xhttp")
	assertRussianDomainDirectRule(t, route)
	assertRussianDirectRule(t, route)
}

func TestMixedProxyConfigURLTestUsesXrayForRemoteRuleSetAndRUDNS(t *testing.T) {
	cfg, err := MixedProxyConfigURLTest(
		[]map[string]any{
			{"type": "hysteria2", "tag": "hysteria2", "server": "example.com", "server_port": 443},
			{"type": "socks", "tag": "xray_reality_xhttp", "server": "127.0.0.1", "server_port": 2081},
		},
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal([]byte(cfg), &decoded); err != nil {
		t.Fatal(err)
	}
	route := decoded["route"].(map[string]any)
	assertRussianDirectRuleSet(t, route, "xray_reality_xhttp")
	dns := decoded["dns"].(map[string]any)
	servers := dns["servers"].([]any)
	ruDNS := servers[1].(map[string]any)
	if ruDNS["detour"] != "xray_reality_xhttp" {
		t.Fatalf("ru-direct-dns detour = %v, want xray_reality_xhttp", ruDNS["detour"])
	}
}

func TestMixedProxyConfigCanDisableRussianDirect(t *testing.T) {
	cfg, err := MixedProxyConfigWithRouting("xray_reality_xhttp", []map[string]any{
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
	if _, ok := route["rules"]; ok {
		t.Fatalf("rules should be absent when russian direct is disabled: %#v", route["rules"])
	}
	dns := decoded["dns"].(map[string]any)
	if _, ok := dns["rules"]; ok {
		t.Fatalf("dns rules should be absent when russian direct is disabled: %#v", dns["rules"])
	}
}

func assertRussianDirectRuleSet(t *testing.T, route map[string]any, downloadDetour string) {
	t.Helper()
	ruleSets := route["rule_set"].([]any)
	if len(ruleSets) != 1 {
		t.Fatalf("rule_set length = %d, want 1: %#v", len(ruleSets), ruleSets)
	}
	ru := ruleSets[0].(map[string]any)
	if ru["tag"] != "geoip-ru" {
		t.Fatalf("rule_set tag = %v, want geoip-ru", ru["tag"])
	}
	if ru["type"] != "remote" {
		t.Fatalf("rule_set type = %v, want remote", ru["type"])
	}
	if ru["format"] != "binary" {
		t.Fatalf("rule_set format = %v, want binary", ru["format"])
	}
	if ru["url"] != geoipRuSRSURL {
		t.Fatalf("rule_set url = %v, want %v", ru["url"], geoipRuSRSURL)
	}
	if ru["download_detour"] != downloadDetour {
		t.Fatalf("download_detour = %v, want %v", ru["download_detour"], downloadDetour)
	}
}

func assertRussianDomainDirectRule(t *testing.T, route map[string]any) {
	t.Helper()
	rules := route["rules"].([]any)
	for _, rule := range rules {
		item := rule.(map[string]any)
		if item["outbound"] != "direct" {
			continue
		}
		suffixes, ok := item["domain_suffix"].([]any)
		if ok && len(suffixes) == 2 && suffixes[0] == "ru" && suffixes[1] == "рф" {
			return
		}
	}
	t.Fatalf("russian domain direct rule missing: %#v", rules)
}

func assertRussianDirectRule(t *testing.T, route map[string]any) {
	t.Helper()
	rules := route["rules"].([]any)
	for _, rule := range rules {
		item := rule.(map[string]any)
		if item["outbound"] != "direct" {
			continue
		}
		sets, ok := item["rule_set"].([]any)
		if ok && len(sets) == 1 && sets[0] == "geoip-ru" {
			return
		}
	}
	t.Fatalf("geoip-ru direct rule missing: %#v", rules)
}
