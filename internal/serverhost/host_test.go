package serverhost

import "testing"

func TestNormalizeHost(t *testing.T) {
	cases := map[string]string{
		" 203.0.113.10 ":          "203.0.113.10",
		"https://vpn.example.com": "vpn.example.com",
		"http://example.com/":     "example.com",
	}
	for input, want := range cases {
		if got := Normalize(input); got != want {
			t.Fatalf("Normalize(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestResolveUsesExplicitHost(t *testing.T) {
	host, err := Resolve("203.0.113.10", func() (string, error) { return "198.51.100.2", nil })
	if err != nil {
		t.Fatal(err)
	}
	if host != "203.0.113.10" {
		t.Fatalf("host = %q", host)
	}
}

func TestResolveUsesDetector(t *testing.T) {
	host, err := Resolve("", func() (string, error) { return "198.51.100.2", nil })
	if err != nil {
		t.Fatal(err)
	}
	if host != "198.51.100.2" {
		t.Fatalf("host = %q", host)
	}
}
