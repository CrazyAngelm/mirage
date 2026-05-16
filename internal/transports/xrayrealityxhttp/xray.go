package xrayrealityxhttp

import (
	"context"
	"encoding/json"

	"mirage/internal/transports"
)

type Transport struct {
	Port       int
	UUID       string
	PrivateKey string
	PublicKey  string
	ShortID    string
	Path       string
	ServerName string
}

func (t Transport) Name() string          { return "xray_reality_xhttp" }
func (t Transport) Role() transports.Role { return transports.RoleStealth }

func (t Transport) ServerFiles(ctx transports.Context) ([]transports.GeneratedFile, error) {
	cfg := map[string]any{
		"log": map[string]any{"loglevel": "warning"},
		"inbounds": []any{map[string]any{
			"port":     t.Port,
			"protocol": "vless",
			"settings": map[string]any{"clients": []any{map[string]any{"id": t.UUID, "flow": ""}}, "decryption": "none"},
			"streamSettings": map[string]any{
				"network":       "xhttp",
				"security":      "reality",
				"xhttpSettings": map[string]any{"path": t.Path},
				"realitySettings": map[string]any{
					"dest":        t.ServerName + ":443",
					"serverNames": []string{t.ServerName},
					"privateKey":  t.PrivateKey,
					"shortIds":    []string{t.ShortID},
				},
			},
		}},
		"outbounds": []any{map[string]any{"protocol": "freedom"}},
	}
	payload, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return nil, err
	}
	return []transports.GeneratedFile{{Path: ctx.BaseDir + "/configs/xray.json", Content: string(payload), Mode: 0600}}, nil
}

func (t Transport) ClientBundle(ctx transports.Context) (map[string]any, error) {
	return map[string]any{"server": ctx.Domain, "port": t.Port, "uuid": t.UUID, "public_key": t.PublicKey, "short_id": t.ShortID, "path": t.Path, "server_name": t.ServerName}, nil
}

func (t Transport) HealthCheck(ctx context.Context) transports.HealthResult {
	return transports.HealthResult{Name: t.Name(), Reachable: false, Message: "not started"}
}
