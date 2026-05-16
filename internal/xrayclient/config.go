package xrayclient

import "encoding/json"

type ConfigInput struct {
	Server     string
	Port       int
	UUID       string
	PublicKey  string
	ShortID    string
	Path       string
	ServerName string
	LocalPort  int
}

func Config(input ConfigInput) (string, error) {
	cfg := map[string]any{
		"log": map[string]any{"loglevel": "warning"},
		"inbounds": []any{map[string]any{
			"listen":   "127.0.0.1",
			"port":     input.LocalPort,
			"protocol": "socks",
			"settings": map[string]any{"udp": true},
		}},
		"outbounds": []any{map[string]any{
			"protocol": "vless",
			"settings": map[string]any{"vnext": []any{map[string]any{
				"address": input.Server,
				"port":    input.Port,
				"users":   []any{map[string]any{"id": input.UUID, "encryption": "none"}},
			}}},
			"streamSettings": map[string]any{
				"network":         "xhttp",
				"security":        "reality",
				"xhttpSettings":   map[string]any{"path": input.Path},
				"realitySettings": map[string]any{"serverName": input.ServerName, "publicKey": input.PublicKey, "shortId": input.ShortID, "fingerprint": "chrome"},
			},
		}},
	}
	payload, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return "", err
	}
	return string(payload) + "\n", nil
}
