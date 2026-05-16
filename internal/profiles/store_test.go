package profiles

import (
	"path/filepath"
	"testing"

	"mirage/internal/bundle"
)

func TestImportAddsProfilesAndKeepsExisting(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "state"))
	first := testLink(t, "main", "Main VPS", "203.0.113.10")
	second := testLink(t, "backup", "Backup EU", "198.51.100.2")

	if err := store.ImportLink(first); err != nil {
		t.Fatal(err)
	}
	if err := store.ImportLink(second); err != nil {
		t.Fatal(err)
	}

	profiles, err := store.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(profiles) != 2 {
		t.Fatalf("len = %d", len(profiles))
	}
	active, err := store.Active()
	if err != nil {
		t.Fatal(err)
	}
	if active.ID != "main" {
		t.Fatalf("active = %q", active.ID)
	}
}

func TestSetActiveAndCamouflageOverride(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "state"))
	if err := store.ImportLink(testLink(t, "main", "Main VPS", "203.0.113.10")); err != nil {
		t.Fatal(err)
	}
	if err := store.SetCamouflageOverride("main", "www.cloudflare.com"); err != nil {
		t.Fatal(err)
	}
	active, err := store.Active()
	if err != nil {
		t.Fatal(err)
	}
	if active.CamouflageSite() != "www.cloudflare.com" {
		t.Fatalf("camouflage = %q", active.CamouflageSite())
	}
}

func testLink(t *testing.T, id, name, host string) string {
	t.Helper()
	link, err := bundle.Encode(bundle.Bundle{
		Version:     1,
		ProfileID:   id,
		DisplayName: name,
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
