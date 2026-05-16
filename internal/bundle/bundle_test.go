package bundle

import "testing"

func TestEncodeDecode(t *testing.T) {
	original := Bundle{
		Version:    1,
		ProfileID:  "default",
		Server:     map[string]any{"domain": "vpn.example.com"},
		Client:     map[string]any{"id": "client-1", "name": "pc"},
		Transports: map[string]any{"hysteria2": map[string]any{"port": float64(443)}},
		DNS:        map[string]any{"mode": "secure"},
		Routing:    map[string]any{"mode": "auto"},
	}
	link, err := Encode(original)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := Decode(link)
	if err != nil {
		t.Fatal(err)
	}
	if decoded.Version != 1 || decoded.ProfileID != "default" {
		t.Fatalf("decoded bundle mismatch: %+v", decoded)
	}
}

func TestDecodeRejectsWrongScheme(t *testing.T) {
	if _, err := Decode("http://bad"); err == nil {
		t.Fatal("expected wrong scheme error")
	}
}

func TestBundleServerHostPrefersHostThenDomain(t *testing.T) {
	b := Bundle{Server: map[string]any{"host": "203.0.113.10", "domain": "vpn.example.com"}}
	if got := b.ServerHost(); got != "203.0.113.10" {
		t.Fatalf("ServerHost() = %q", got)
	}

	b = Bundle{Server: map[string]any{"domain": "vpn.example.com"}}
	if got := b.ServerHost(); got != "vpn.example.com" {
		t.Fatalf("ServerHost() fallback = %q", got)
	}
}

func TestBundleDisplayNameAndCamouflageSite(t *testing.T) {
	b := Bundle{
		ProfileID:   "main-vps",
		DisplayName: "Main VPS",
		Server:      map[string]any{"camouflage_site": "www.microsoft.com"},
	}
	if b.Name() != "Main VPS" {
		t.Fatalf("Name() = %q", b.Name())
	}
	if b.CamouflageSite() != "www.microsoft.com" {
		t.Fatalf("CamouflageSite() = %q", b.CamouflageSite())
	}
}
