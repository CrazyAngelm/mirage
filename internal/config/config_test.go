package config

import "testing"

func TestDefaultServerConfig(t *testing.T) {
	cfg := DefaultServerConfig("vpn.example.com", "/opt/mirage")
	if cfg.Version != 1 {
		t.Fatalf("Version = %d, want 1", cfg.Version)
	}
	if cfg.Server.Domain != "vpn.example.com" {
		t.Fatalf("Domain = %q", cfg.Server.Domain)
	}
	if !cfg.Transports.Hysteria2.Enabled || !cfg.Transports.XrayRealityXHTTP.Enabled || !cfg.Transports.AmneziaWG.Enabled {
		t.Fatalf("all MVP transports must be enabled by default")
	}
	if !cfg.DNS.BlockSystemDNS || !cfg.DNS.BlockIPv6Leaks {
		t.Fatalf("DNS leak protection defaults must be enabled")
	}
}
