package amneziawg

import (
	"testing"

	"mirage/internal/transports"
)

func TestAmneziaWGClientBundle(t *testing.T) {
	transport := Transport{Port: 51820, Address: "10.77.0.1/24"}
	bundle, err := transport.ClientBundle(transports.Context{Domain: "example.com"})
	if err != nil {
		t.Fatal(err)
	}
	if bundle["server"] != "example.com" {
		t.Fatalf("server = %v", bundle["server"])
	}
	if bundle["port"] != 51820 {
		t.Fatalf("port = %v", bundle["port"])
	}
	if bundle["address"] != "10.77.0.2/32" {
		t.Fatalf("address = %v", bundle["address"])
	}
	if bundle["status"] != nil {
		t.Fatalf("status should be absent, got %v", bundle["status"])
	}
}

func TestAmneziaWGServerFilesWithJitter(t *testing.T) {
	transport := Transport{Port: 51820, Address: "10.77.0.1/24", PrivateKey: "testkey", Jc: 4, Jf: 3, Jd: 40, Jmin: 50, Jmax: 1000}
	files, err := transport.ServerFiles(transports.Context{BaseDir: "/opt/mirage", Domain: "example.com"})
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}
	content := files[0].Content
	if !contains(content, "Jc = 4") {
		t.Fatal("missing Jc in config")
	}
	if !contains(content, "Jf = 3") {
		t.Fatal("missing Jf in config")
	}
}

func TestAmneziaWGServerFilesWithoutJitter(t *testing.T) {
	transport := Transport{Port: 51820, Address: "10.77.0.1/24", PrivateKey: "testkey"}
	files, err := transport.ServerFiles(transports.Context{BaseDir: "/opt/mirage", Domain: "example.com"})
	if err != nil {
		t.Fatal(err)
	}
	content := files[0].Content
	if contains(content, "Jc") {
		t.Fatal("Jc should not be present when Jc=0")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && (s[0:len(substr)] == substr || contains(s[1:], substr)))
}
