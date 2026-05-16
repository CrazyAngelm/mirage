package amneziawg

import (
	"context"
	"fmt"

	"mirage/internal/transports"
)

type Transport struct {
	Port             int
	PrivateKey       string
	PublicKey        string
	Address          string
	ClientPublicKey  string
	ClientPrivateKey string
	Jc               int
	Jf               int
	Jd               int
	Jmin             int
	Jmax             int
}

func (t Transport) Name() string          { return "amneziawg" }
func (t Transport) Role() transports.Role { return transports.RoleFallback }

func (t Transport) ServerFiles(ctx transports.Context) ([]transports.GeneratedFile, error) {
	peerSection := ""
	if t.ClientPublicKey != "" {
		peerSection = fmt.Sprintf("\n[Peer]\nPublicKey = %s\nAllowedIPs = 10.77.0.2/32\n", t.ClientPublicKey)
	}
	jitterSection := ""
	if t.Jc > 0 {
		jitterSection = fmt.Sprintf("Jc = %d\nJf = %d\nJd = %d\nJmin = %d\nJmax = %d\n", t.Jc, t.Jf, t.Jd, t.Jmin, t.Jmax)
	}
	content := fmt.Sprintf(`[Interface]
Address = %s
ListenPort = %d
PrivateKey = %s
%s
PostUp = iptables -t nat -A POSTROUTING -s %s -o eth0 -j MASQUERADE
PostDown = iptables -t nat -D POSTROUTING -s %s -o eth0 -j MASQUERADE
%s`, t.Address, t.Port, t.PrivateKey, jitterSection, t.Address, t.Address, peerSection)
	return []transports.GeneratedFile{{Path: ctx.BaseDir + "/configs/amneziawg.conf", Content: content, Mode: 0600}}, nil
}

func (t Transport) ClientBundle(ctx transports.Context) (map[string]any, error) {
	return map[string]any{
		"server":             ctx.Domain,
		"port":               t.Port,
		"public_key":         t.PublicKey,
		"address":            "10.77.0.2/32",
		"client_private_key": t.ClientPrivateKey,
		"jc":                 t.Jc,
		"jf":                 t.Jf,
		"jd":                 t.Jd,
		"jmin":               t.Jmin,
		"jmax":               t.Jmax,
	}, nil
}

func (t Transport) HealthCheck(ctx context.Context) transports.HealthResult {
	return transports.HealthResult{Name: t.Name(), Reachable: false, Message: "not started"}
}
