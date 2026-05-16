package hysteria2

import (
	"context"
	"fmt"

	"mirage/internal/transports"
)

type Transport struct {
	Port     int
	Password string
}

func (t Transport) Name() string          { return "hysteria2" }
func (t Transport) Role() transports.Role { return transports.RoleSpeed }

func (t Transport) ServerFiles(ctx transports.Context) ([]transports.GeneratedFile, error) {
	content := fmt.Sprintf("listen: :%d\ntls:\n  cert: %s/state/hysteria.crt\n  key: %s/state/hysteria.key\nauth:\n  type: password\n  password: %q\nmasquerade:\n  type: proxy\n  proxy:\n    url: https://%s/\n    rewriteHost: true\n", t.Port, ctx.BaseDir, ctx.BaseDir, t.Password, ctx.Domain)
	return []transports.GeneratedFile{{Path: ctx.BaseDir + "/configs/hysteria.yaml", Content: content, Mode: 0600}}, nil
}

func (t Transport) ClientBundle(ctx transports.Context) (map[string]any, error) {
	return map[string]any{"server": ctx.Domain, "port": t.Port, "password": t.Password}, nil
}

func (t Transport) HealthCheck(ctx context.Context) transports.HealthResult {
	return transports.HealthResult{Name: t.Name(), Reachable: false, Message: "not started"}
}
