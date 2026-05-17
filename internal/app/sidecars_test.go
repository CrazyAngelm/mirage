package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveSidecarPrefersPackagedBin(t *testing.T) {
	exeDir := t.TempDir()
	base := t.TempDir()
	packaged := filepath.Join(exeDir, "bin", "sing-box.exe")
	fallback := filepath.Join(base, "bin", "sing-box.exe")
	if err := os.MkdirAll(filepath.Dir(packaged), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(fallback), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(packaged, []byte("packaged"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fallback, []byte("fallback"), 0600); err != nil {
		t.Fatal(err)
	}

	got, ok := resolveSidecar(exeDir, base, "sing-box.exe")
	if !ok {
		t.Fatal("resolveSidecar returned ok=false")
	}
	if got != packaged {
		t.Fatalf("resolveSidecar = %q, want %q", got, packaged)
	}
}

func TestResolveSidecarFallsBackToProgramDataBin(t *testing.T) {
	exeDir := t.TempDir()
	base := t.TempDir()
	fallback := filepath.Join(base, "bin", "xray.exe")
	if err := os.MkdirAll(filepath.Dir(fallback), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fallback, []byte("fallback"), 0600); err != nil {
		t.Fatal(err)
	}

	got, ok := resolveSidecar(exeDir, base, "xray.exe")
	if !ok {
		t.Fatal("resolveSidecar returned ok=false")
	}
	if got != fallback {
		t.Fatalf("resolveSidecar = %q, want %q", got, fallback)
	}
}

func TestMissingSidecarMessageReferencesWindowsZip(t *testing.T) {
	msg := missingSidecarMessage("sing-box.exe")
	if !strings.Contains(msg, "mirage-windows-amd64.zip") {
		t.Fatalf("message = %q, want zip guidance", msg)
	}
	if !strings.Contains(msg, "sing-box.exe") {
		t.Fatalf("message = %q, want binary name", msg)
	}
}

func TestSingBoxConfigLooksStaleDetectsLegacyTag(t *testing.T) {
	payload := []byte(`{"outbounds":[{"type":"socks","tag":"xray_ws_tls"}]}`)
	stale, reason := singBoxConfigLooksStale(payload)
	if !stale {
		t.Fatal("singBoxConfigLooksStale = false, want true")
	}
	if !strings.Contains(reason, "xray_ws_tls") {
		t.Fatalf("reason = %q, want xray_ws_tls", reason)
	}
}

func TestSingBoxConfigLooksStaleDetectsNativeVLESS(t *testing.T) {
	payload := []byte(`{"outbounds":[{"type":"vless","tag":"xray_reality_xhttp"}]}`)
	stale, reason := singBoxConfigLooksStale(payload)
	if !stale {
		t.Fatal("singBoxConfigLooksStale = false, want true")
	}
	if !strings.Contains(reason, "native vless") {
		t.Fatalf("reason = %q, want native vless", reason)
	}
}

func TestSingBoxConfigLooksStaleAcceptsCurrentXraySOCKS(t *testing.T) {
	payload := []byte(`{"outbounds":[{"type":"socks","tag":"xray_reality_xhttp","server":"127.0.0.1","server_port":2081}]}`)
	stale, reason := singBoxConfigLooksStale(payload)
	if stale {
		t.Fatalf("singBoxConfigLooksStale = true, reason %q", reason)
	}
}
