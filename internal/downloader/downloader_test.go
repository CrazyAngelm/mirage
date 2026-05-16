package downloader

import (
	"strings"
	"testing"
)

func TestDefaultManifestHasAllServerSidecars(t *testing.T) {
	manifest := DefaultManifest("linux", "arm64")
	for _, name := range []string{"xray", "hysteria", "amneziawg"} {
		entry, ok := manifest.Entry(name)
		if !ok {
			t.Fatalf("missing sidecar %s", name)
		}
		if entry.TargetName == "" {
			t.Fatalf("%s target name empty", name)
		}
		if entry.URL == "" && entry.EnvURL == "" {
			t.Fatalf("%s needs URL or env URL", name)
		}
	}
}

func TestDefaultManifestRejectsUnknownArch(t *testing.T) {
	manifest := DefaultManifest("plan9", "mips")
	if len(manifest.Entries) != 0 {
		t.Fatalf("unexpected entries: %+v", manifest.Entries)
	}
}

func TestDefaultManifestPinsVersions(t *testing.T) {
	manifest := DefaultManifest("linux", "amd64")
	for _, entry := range manifest.Entries {
		if entry.Name == "amneziawg" {
			continue
		}
		if !strings.Contains(entry.URL, "/download/") {
			t.Fatalf("%s URL must use versioned release URL: %s", entry.Name, entry.URL)
		}
		if strings.Contains(entry.URL, "/latest/") {
			t.Fatalf("%s URL must not use latest: %s", entry.Name, entry.URL)
		}
		if entry.SHA256 == "" {
			t.Fatalf("%s checksum missing", entry.Name)
		}
	}
}
