package transports

import "context"

type Role string

const (
	RoleSpeed    Role = "speed"
	RoleStealth  Role = "stealth"
	RoleFallback Role = "fallback"
)

type GeneratedFile struct {
	Path    string
	Content string
	Mode    uint32
}

type HealthResult struct {
	Name      string
	Reachable bool
	LatencyMS int
	Message   string
}

type Context struct {
	BaseDir string
	Domain  string
}

type Transport interface {
	Name() string
	Role() Role
	ServerFiles(ctx Context) ([]GeneratedFile, error)
	ClientBundle(ctx Context) (map[string]any, error)
	HealthCheck(ctx context.Context) HealthResult
}
