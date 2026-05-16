package app

import (
	"bytes"
	"encoding/base64"
	"path/filepath"
	"strings"
	"testing"

	"mirage/internal/bundle"
	"mirage/internal/config"
)

func TestServerDownloadSidecarsPrintManifest(t *testing.T) {
	var out bytes.Buffer
	if err := RunServer([]string{"download-sidecars", "--print-manifest"}, &out, &out); err != nil {
		t.Fatal(err)
	}
	text := out.String()
	for _, want := range []string{"xray", "hysteria", "amneziawg"} {
		if !strings.Contains(text, want) {
			t.Fatalf("manifest output missing %s: %s", want, text)
		}
	}
}

func TestDefaultBundleUsesClientWireGuardAddress(t *testing.T) {
	cfg := config.DefaultServerConfig("example.com", t.TempDir())
	cfg.Clients = []config.ClientRecord{{ID: "client", Name: "pc"}}
	cfg.Transports.AmneziaWG.PublicKey = "server-public"
	cfg.Transports.AmneziaWG.ClientPrivateKey = "client-private"
	link, err := buildDefaultBundle(cfg)
	if err != nil {
		t.Fatal(err)
	}
	b, err := bundle.Decode(link)
	if err != nil {
		t.Fatal(err)
	}
	awg := b.Transports["amneziawg"].(map[string]any)
	if got := awg["address"]; got != "10.77.0.2/32" {
		t.Fatalf("amneziawg client address = %v, want 10.77.0.2/32", got)
	}
	if got := awg["port"]; got != float64(51820) {
		t.Fatalf("amneziawg client port = %v, want 51820", got)
	}
}

func TestRunServerSetupAcceptsAutoHost(t *testing.T) {
	args := parseServerSetupArgs([]string{"--auto", "--host", "203.0.113.10"})
	if !args.Auto {
		t.Fatal("Auto = false")
	}
	if args.Host != "203.0.113.10" {
		t.Fatalf("Host = %q", args.Host)
	}
}

func TestPrintSetupSummaryAutoIncludesClientImportLink(t *testing.T) {
	var out bytes.Buffer
	if err := printSetupSummary(&out, "/opt/mirage", "203.0.113.10", "mirage://abc", true); err != nil {
		t.Fatal(err)
	}
	text := out.String()
	for _, want := range []string{
		"Server host: 203.0.113.10",
		"Client import link: mirage://abc",
		"Link saved to: " + filepath.Join("/opt/mirage", "state", "default.link"),
		"contains client secrets",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("setup summary missing %q: %s", want, text)
		}
	}
}

func TestBuildDefaultBundleIncludesHostDisplayNameAndCamouflage(t *testing.T) {
	cfg := config.DefaultServerConfig("203.0.113.10", t.TempDir())
	cfg.Clients = []config.ClientRecord{{ID: "client-id", Name: "default-pc"}}
	cfg.Transports.Hysteria2.Password = "hysteria-password"
	cfg.Transports.XrayRealityXHTTP.UUID = "uuid"
	cfg.Transports.XrayRealityXHTTP.PublicKey = "public-key"
	cfg.Transports.XrayRealityXHTTP.ShortID = "short-id"
	cfg.Transports.AmneziaWG.PublicKey = "server-public"
	cfg.Transports.AmneziaWG.ClientPrivateKey = "client-private"

	link, err := buildDefaultBundle(cfg)
	if err != nil {
		t.Fatal(err)
	}
	b, err := bundle.Decode(link)
	if err != nil {
		t.Fatal(err)
	}
	if b.DisplayName != "Main VPS" {
		t.Fatalf("DisplayName = %q", b.DisplayName)
	}
	if b.ServerHost() != "203.0.113.10" {
		t.Fatalf("ServerHost = %q", b.ServerHost())
	}
	if b.CamouflageSite() != "www.microsoft.com" {
		t.Fatalf("CamouflageSite = %q", b.CamouflageSite())
	}
}

func TestWireGuardKeyPairUsesBase64Raw32ByteKeys(t *testing.T) {
	privateKey, publicKey, err := generateWireGuardKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	for name, value := range map[string]string{"private": privateKey, "public": publicKey} {
		decoded, err := base64.StdEncoding.DecodeString(value)
		if err != nil {
			t.Fatalf("%s key is not base64: %v", name, err)
		}
		if len(decoded) != 32 {
			t.Fatalf("%s key decoded length = %d, want 32", name, len(decoded))
		}
	}
}
