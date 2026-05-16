package xrayclient

import (
	"encoding/json"
	"testing"
)

func TestClientConfigCreatesLocalSocksOutbound(t *testing.T) {
	cfg, err := Config(ConfigInput{
		Server:     "203.0.113.10",
		Port:       443,
		UUID:       "ab620e8ef1a4f790d1b1302c0cd5b650",
		PublicKey:  "abc",
		ShortID:    "1234abcd",
		Path:       "/api/session",
		ServerName: "www.microsoft.com",
		LocalPort:  2081,
	})
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal([]byte(cfg), &decoded); err != nil {
		t.Fatal(err)
	}
	inbounds := decoded["inbounds"].([]any)
	if inbounds[0].(map[string]any)["port"] != float64(2081) {
		t.Fatalf("local port not set: %+v", inbounds[0])
	}
}
