package app

import (
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mirage/internal/bundle"
	"mirage/internal/profiles"
	"mirage/internal/templates"
)

func TestReadProfileLinkUsesActiveProfile(t *testing.T) {
	base := t.TempDir()
	state := filepath.Join(base, "state")
	store := profiles.NewStore(state)
	first := mustProfileLink(t, "main", "203.0.113.10")
	second := mustProfileLink(t, "backup", "198.51.100.2")
	if err := store.ImportLink(first); err != nil {
		t.Fatal(err)
	}
	if err := store.ImportLink(second); err != nil {
		t.Fatal(err)
	}
	if err := store.SetActive("backup"); err != nil {
		t.Fatal(err)
	}
	link, err := ReadProfileLink(base)
	if err != nil {
		t.Fatal(err)
	}
	b, err := bundle.Decode(link)
	if err != nil {
		t.Fatal(err)
	}
	if b.ProfileID != "backup" {
		t.Fatalf("ProfileID = %q", b.ProfileID)
	}
}

func TestCamouflageOverrideChangesXrayServerName(t *testing.T) {
	b := bundle.Bundle{
		Version:   1,
		ProfileID: "main",
		Server:    map[string]any{"host": "203.0.113.10", "camouflage_site": "www.microsoft.com"},
		Transports: map[string]any{
			"xray_reality_xhttp": map[string]any{"server": "203.0.113.10", "port": float64(443), "uuid": "u", "server_name": "www.microsoft.com", "public_key": "pk", "short_id": "sid", "path": "/api/session"},
		},
	}
	applyCamouflageOverride(&b, "www.cloudflare.com")
	xray := b.Transports["xray_reality_xhttp"].(map[string]any)
	if xray["server_name"] != "www.cloudflare.com" {
		t.Fatalf("server_name = %v", xray["server_name"])
	}
}

func mustProfileLink(t *testing.T, id string, host string) string {
	t.Helper()
	link, err := bundle.Encode(bundle.Bundle{
		Version:     1,
		ProfileID:   id,
		DisplayName: id,
		Server:      map[string]any{"host": host, "camouflage_site": "www.microsoft.com"},
		Transports:  map[string]any{},
		DNS:         map[string]any{},
		Routing:     map[string]any{},
	})
	if err != nil {
		t.Fatal(err)
	}
	return link
}

func TestCheapModeKeepsOnlyActiveTransport(t *testing.T) {
	b := bundle.Bundle{Transports: map[string]any{
		"hysteria2":          map[string]any{"server": "example.com", "port": float64(443), "password": "p"},
		"xray_reality_xhttp": map[string]any{"server": "example.com", "port": float64(443), "uuid": "u", "server_name": "www.microsoft.com", "public_key": "pk", "short_id": "sid", "path": "/api/session"},
		"amneziawg":          map[string]any{"server": "example.com", "port": float64(51820), "public_key": "wg", "client_private_key": "abc", "address": "10.77.0.1/24"},
	}}
	outbounds, endpoints := buildSingBoxOutbounds(b)
	outbounds, endpoints = keepOnlyTag(outbounds, endpoints, "hysteria2")
	cfg, err := templates.SingBoxConfig("hysteria2", outbounds)
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal([]byte(cfg), &decoded); err != nil {
		t.Fatal(err)
	}
	items := decoded["outbounds"].([]any)
	transportCount := 0
	for _, item := range items {
		tag := item.(map[string]any)["tag"]
		if tag == "hysteria2" || tag == "xray_reality_xhttp" || tag == "amneziawg" {
			transportCount++
		}
	}
	if transportCount != 1 {
		t.Fatalf("transportCount = %d, want 1", transportCount)
	}
	if len(endpoints) != 1 {
		t.Fatalf("endpoints = %d, want 1 (amneziawg should still have endpoint even in cheap mode when filtered to other transport)", len(endpoints))
	}
}

func TestChooseHealthyActiveTagUsesReachableTransport(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	go func() {
		conn, err := listener.Accept()
		if err == nil {
			conn.Close()
		}
	}()
	_, port, err := net.SplitHostPort(listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	outbounds := []map[string]any{
		{"tag": "hysteria2", "server": "127.0.0.1", "server_port": float64(1)},
	}
	endpoints := []map[string]any{
		{"tag": "amneziawg", "peers": []any{map[string]any{"address": "127.0.0.1", "port": port}}},
	}
	if got := chooseHealthyActiveTag(outbounds, endpoints); got != "amneziawg" {
		t.Fatalf("active tag = %s, want amneziawg", got)
	}
}

func TestKeepOnlyTagCanSelectEndpoint(t *testing.T) {
	outbounds := []map[string]any{{"tag": "hysteria2"}, {"tag": "xray_reality_xhttp"}}
	endpoints := []map[string]any{{"type": "wireguard", "tag": "amneziawg"}}
	keptOutbounds, keptEndpoints := keepOnlyTag(outbounds, endpoints, "amneziawg")
	if len(keptOutbounds) != 0 {
		t.Fatalf("kept outbounds = %#v, want none", keptOutbounds)
	}
	if len(keptEndpoints) != 1 || keptEndpoints[0]["tag"] != "amneziawg" {
		t.Fatalf("kept endpoints = %#v", keptEndpoints)
	}
}

func TestClientProtocolDefaultsToAuto(t *testing.T) {
	base := t.TempDir()
	if got := ReadClientProtocol(base); got != "auto" {
		t.Fatalf("ReadClientProtocol = %q, want auto", got)
	}
}

func TestSetClientProtocolPersistsSelection(t *testing.T) {
	base := t.TempDir()
	if err := os.MkdirAll(filepath.Join(base, "state"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := SetClientProtocol(base, "xray_reality_xhttp"); err != nil {
		t.Fatal(err)
	}
	if got := ReadClientProtocol(base); got != "xray_reality_xhttp" {
		t.Fatalf("ReadClientProtocol = %q, want xray_reality_xhttp", got)
	}
}

func TestNormalizeClientProtocolRejectsUnknown(t *testing.T) {
	if got := NormalizeClientProtocol("bogus"); got != "auto" {
		t.Fatalf("NormalizeClientProtocol = %q, want auto", got)
	}
}

func TestRussianDirectDefaultsToOff(t *testing.T) {
	base := t.TempDir()
	if ReadRussianDirect(base) {
		t.Fatal("ReadRussianDirect default = true, want false")
	}
}

func TestParseRussianDirectAcceptsOnOff(t *testing.T) {
	on, err := ParseRussianDirect("on")
	if err != nil || !on {
		t.Fatalf("ParseRussianDirect(on) = %v, %v; want true, nil", on, err)
	}
	off, err := ParseRussianDirect("off")
	if err != nil || off {
		t.Fatalf("ParseRussianDirect(off) = %v, %v; want false, nil", off, err)
	}
}

func TestReadRussianDirectPersistsOff(t *testing.T) {
	base := t.TempDir()
	state := filepath.Join(base, "state")
	if err := os.MkdirAll(state, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(state, "ru_direct.txt"), []byte("off\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if ReadRussianDirect(base) {
		t.Fatal("ReadRussianDirect = true, want false")
	}
}

func TestKeepOnlyRouteTagKeepsSelectedOutbound(t *testing.T) {
	outbounds := []map[string]any{
		{"type": "hysteria2", "tag": "hysteria2"},
		{"type": "socks", "tag": "xray_reality_xhttp"},
	}
	endpoints := []map[string]any{{"type": "wireguard", "tag": "amneziawg"}}
	gotOutbounds, gotEndpoints := keepOnlyRouteTag(outbounds, endpoints, "xray_reality_xhttp")
	if len(gotOutbounds) != 1 || gotOutbounds[0]["tag"] != "xray_reality_xhttp" {
		t.Fatalf("outbounds = %#v, want only xray_reality_xhttp", gotOutbounds)
	}
	if len(gotEndpoints) != 0 {
		t.Fatalf("endpoints = %#v, want empty", gotEndpoints)
	}
}

func TestKeepOnlyRouteTagKeepsSelectedEndpoint(t *testing.T) {
	outbounds := []map[string]any{
		{"type": "hysteria2", "tag": "hysteria2"},
		{"type": "socks", "tag": "xray_reality_xhttp"},
	}
	endpoints := []map[string]any{{"type": "wireguard", "tag": "amneziawg"}}
	gotOutbounds, gotEndpoints := keepOnlyRouteTag(outbounds, endpoints, "amneziawg")
	if len(gotOutbounds) != 0 {
		t.Fatalf("outbounds = %#v, want empty", gotOutbounds)
	}
	if len(gotEndpoints) != 1 || gotEndpoints[0]["tag"] != "amneziawg" {
		t.Fatalf("endpoints = %#v, want only amneziawg", gotEndpoints)
	}
}

func TestKeepOnlyRouteTagAutoReturnsUnchanged(t *testing.T) {
	outbounds := []map[string]any{
		{"type": "hysteria2", "tag": "hysteria2"},
		{"type": "socks", "tag": "xray_reality_xhttp"},
	}
	endpoints := []map[string]any{{"type": "wireguard", "tag": "amneziawg"}}
	gotOutbounds, gotEndpoints := keepOnlyRouteTag(outbounds, endpoints, "auto")
	if len(gotOutbounds) != 2 {
		t.Fatalf("outbounds = %#v, want 2 (auto returns unchanged)", gotOutbounds)
	}
	if len(gotEndpoints) != 1 {
		t.Fatalf("endpoints = %#v, want 1 (auto returns unchanged)", gotEndpoints)
	}
}

func TestForcedXrayConfigExcludesAutoOutbound(t *testing.T) {
	b := bundle.Bundle{Transports: map[string]any{
		"hysteria2":          map[string]any{"server": "example.com", "port": float64(443), "password": "p"},
		"xray_reality_xhttp": map[string]any{"server": "example.com", "port": float64(443), "uuid": "u", "server_name": "www.microsoft.com", "public_key": "pk", "short_id": "sid", "path": "/api/session"},
		"amneziawg":          map[string]any{"server": "example.com", "port": float64(51820), "public_key": "wg", "client_private_key": "abc", "address": "10.77.0.1/24"},
	}}
	outbounds, endpoints := buildSingBoxOutbounds(b)
	gotOutbounds, gotEndpoints := keepOnlyRouteTag(outbounds, endpoints, "xray_reality_xhttp")
	if len(gotOutbounds) != 1 {
		t.Fatalf("outbounds = %#v, want 1", gotOutbounds)
	}
	if gotOutbounds[0]["tag"] != "xray_reality_xhttp" {
		t.Fatalf("tag = %q, want xray_reality_xhttp", gotOutbounds[0]["tag"])
	}
	if len(gotEndpoints) != 0 {
		t.Fatalf("endpoints = %#v, want none when forced xray", gotEndpoints)
	}
	// Verify the hysteria2 outbound is NOT in the result
	for _, ob := range gotOutbounds {
		if ob["tag"] == "hysteria2" {
			t.Fatal("hysteria2 outbound should be absent when forced xray")
		}
	}
}

func TestForcedAmneziaWGConfigIncludesEndpointAndNoURLTest(t *testing.T) {
	b := bundle.Bundle{Transports: map[string]any{
		"hysteria2":          map[string]any{"server": "example.com", "port": float64(443), "password": "p"},
		"xray_reality_xhttp": map[string]any{"server": "example.com", "port": float64(443), "uuid": "u", "server_name": "www.microsoft.com", "public_key": "pk", "short_id": "sid", "path": "/api/session"},
		"amneziawg":          map[string]any{"server": "example.com", "port": float64(51820), "public_key": "wg", "client_private_key": "abc", "address": "10.77.0.1/24"},
	}}
	outbounds, endpoints := buildSingBoxOutbounds(b)
	gotOutbounds, gotEndpoints := keepOnlyRouteTag(outbounds, endpoints, "amneziawg")

	// Forced amneziawg: outbounds must be empty (amneziawg lives in endpoints)
	if len(gotOutbounds) != 0 {
		t.Fatalf("outbounds = %#v, want empty (amneziawg is an endpoint)", gotOutbounds)
	}
	if len(gotEndpoints) != 1 {
		t.Fatalf("endpoints = %#v, want 1 amneziawg endpoint", gotEndpoints)
	}
	if gotEndpoints[0]["tag"] != "amneziawg" {
		t.Fatalf("endpoint tag = %q, want amneziawg", gotEndpoints[0]["tag"])
	}

	// Generate config with the new WithEndpoints path
	cfg, err := templates.SingBoxConfigWithEndpoints("amneziawg", gotOutbounds, gotEndpoints)
	if err != nil {
		t.Fatal(err)
	}

	var decoded map[string]any
	if err := json.Unmarshal([]byte(cfg), &decoded); err != nil {
		t.Fatal(err)
	}

	// Assert endpoints contains tag amneziawg
	eps, ok := decoded["endpoints"].([]any)
	if !ok || len(eps) == 0 {
		t.Fatal("endpoints missing or empty in generated config")
	}
	foundEndpoint := false
	for _, ep := range eps {
		epMap := ep.(map[string]any)
		if epMap["tag"] == "amneziawg" {
			foundEndpoint = true
			break
		}
	}
	if !foundEndpoint {
		t.Fatal("endpoints does not contain amneziawg tag")
	}

	// Assert route.final == "amneziawg"
	route := decoded["route"].(map[string]any)
	if route["final"] != "amneziawg" {
		t.Fatalf("route.final = %q, want amneziawg", route["final"])
	}

	// Assert outbounds has no tag "auto" (no urltest outbound)
	outboundsList := decoded["outbounds"].([]any)
	for _, ob := range outboundsList {
		obMap := ob.(map[string]any)
		if obMap["tag"] == "auto" {
			t.Fatal("outbounds must not contain 'auto' tag (no urltest in forced mode)")
		}
	}

	// Verify direct and block outbounds are present
	hasDirect := false
	hasBlock := false
	for _, ob := range outboundsList {
		obMap := ob.(map[string]any)
		if obMap["tag"] == "direct" {
			hasDirect = true
		}
		if obMap["tag"] == "block" {
			hasBlock = true
		}
	}
	if !hasDirect {
		t.Fatal("direct outbound missing")
	}
	if !hasBlock {
		t.Fatal("block outbound missing")
	}
}

func TestBuildSingBoxOutboundsUsesXraySidecarSOCKS(t *testing.T) {
	b := bundle.Bundle{Transports: map[string]any{
		"xray_reality_xhttp": map[string]any{"server": "example.com", "port": float64(443), "uuid": "u", "server_name": "www.microsoft.com", "public_key": "pk", "short_id": "sid", "path": "/api/session"},
	}}
	outbounds, endpoints := buildSingBoxOutbounds(b)
	if len(endpoints) != 0 {
		t.Fatalf("endpoints = %#v, want none", endpoints)
	}
	if len(outbounds) != 1 {
		t.Fatalf("outbounds = %#v, want one xray outbound", outbounds)
	}
	got := outbounds[0]
	if got["type"] != "socks" {
		t.Fatalf("xray outbound type = %v, want socks", got["type"])
	}
	if got["tag"] != "xray_reality_xhttp" {
		t.Fatalf("xray outbound tag = %v, want xray_reality_xhttp", got["tag"])
	}
	if got["server"] != "127.0.0.1" {
		t.Fatalf("xray outbound server = %v, want 127.0.0.1", got["server"])
	}
	if got["server_port"] != 2081 {
		t.Fatalf("xray outbound server_port = %v, want 2081", got["server_port"])
	}
}

func TestGeneratedSingBoxConfigHasNoLegacyNativeVLESS(t *testing.T) {
	b := bundle.Bundle{Transports: map[string]any{
		"xray_reality_xhttp": map[string]any{"server": "example.com", "port": float64(443), "uuid": "u", "server_name": "www.microsoft.com", "public_key": "pk", "short_id": "sid", "path": "/api/session"},
	}}
	outbounds, endpoints := buildSingBoxOutbounds(b)
	cfg, err := templates.SingBoxConfigWithEndpoints("xray_reality_xhttp", outbounds, endpoints)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(cfg, "xray_ws_tls") {
		t.Fatalf("generated config contains legacy xray_ws_tls: %s", cfg)
	}
	var decoded map[string]any
	if err := json.Unmarshal([]byte(cfg), &decoded); err != nil {
		t.Fatal(err)
	}
	for _, outbound := range decoded["outbounds"].([]any) {
		item := outbound.(map[string]any)
		if item["type"] == "vless" {
			t.Fatalf("generated config contains native vless outbound: %#v", item)
		}
	}
}
