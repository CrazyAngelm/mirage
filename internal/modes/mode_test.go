package modes

import "testing"

func TestNormalizeMode(t *testing.T) {
	cases := map[string]Mode{
		"":       Normal,
		"normal": Normal,
		"cheap":  Cheap,
		"full":   Full,
		"super":  Super,
		"bad":    Normal,
	}
	for input, want := range cases {
		if got := Normalize(input); got != want {
			t.Fatalf("Normalize(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestIsTun(t *testing.T) {
	if !IsTun(Full) {
		t.Fatal("Full should be TUN")
	}
	if IsTun(Normal) || IsTun(Cheap) || IsTun(Super) {
		t.Fatal("non-Full modes should not be TUN")
	}
}

func TestCheapKeepsOneTransport(t *testing.T) {
	outbounds := []Outbound{
		{Tag: "hysteria2", Reachable: true},
		{Tag: "xray_reality_xhttp", Reachable: true},
		{Tag: "amneziawg", Reachable: false},
	}
	selected := Select(Cheap, outbounds)
	if len(selected) != 1 || selected[0].Tag != "hysteria2" {
		t.Fatalf("selected = %+v", selected)
	}
}

func TestNormalKeepsAllTransports(t *testing.T) {
	outbounds := []Outbound{{Tag: "hysteria2"}, {Tag: "xray_reality_xhttp"}, {Tag: "amneziawg"}}
	selected := Select(Normal, outbounds)
	if len(selected) != 3 {
		t.Fatalf("normal selected %d transports", len(selected))
	}
}

func TestSuperReservedKeepsAllTransports(t *testing.T) {
	outbounds := []Outbound{{Tag: "hysteria2"}, {Tag: "xray_reality_xhttp"}}
	selected := Select(Super, outbounds)
	if len(selected) != 2 {
		t.Fatalf("super selected %d transports", len(selected))
	}
}
