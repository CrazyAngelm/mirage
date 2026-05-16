package config

type MirageConfig struct {
	Version    int               `json:"version" yaml:"version"`
	Server     ServerConfig      `json:"server" yaml:"server"`
	Clients    []ClientRecord    `json:"clients" yaml:"clients"`
	Transports TransportSettings `json:"transports" yaml:"transports"`
	DNS        DNSConfig         `json:"dns" yaml:"dns"`
	Routing    RoutingConfig     `json:"routing" yaml:"routing"`
}

type ServerConfig struct {
	Domain    string `json:"domain" yaml:"domain"`
	BaseDir   string `json:"base_dir" yaml:"base_dir"`
	HTTPPort  int    `json:"http_port" yaml:"http_port"`
	HTTPSPort int    `json:"https_port" yaml:"https_port"`
	UDPPort   int    `json:"udp_port" yaml:"udp_port"`
}

type ClientRecord struct {
	ID   string `json:"id" yaml:"id"`
	Name string `json:"name" yaml:"name"`
}

type TransportSettings struct {
	Hysteria2        Hysteria2Settings        `json:"hysteria2" yaml:"hysteria2"`
	XrayRealityXHTTP XrayRealityXHTTPSettings `json:"xray_reality_xhttp" yaml:"xray_reality_xhttp"`
	AmneziaWG        AmneziaWGSettings        `json:"amneziawg" yaml:"amneziawg"`
}

type Hysteria2Settings struct {
	Enabled  bool   `json:"enabled" yaml:"enabled"`
	Port     int    `json:"port" yaml:"port"`
	Password string `json:"password" yaml:"password"`
}

type XrayRealityXHTTPSettings struct {
	Enabled    bool   `json:"enabled" yaml:"enabled"`
	Port       int    `json:"port" yaml:"port"`
	UUID       string `json:"uuid" yaml:"uuid"`
	PrivateKey string `json:"private_key" yaml:"private_key"`
	PublicKey  string `json:"public_key" yaml:"public_key"`
	ShortID    string `json:"short_id" yaml:"short_id"`
	Path       string `json:"path" yaml:"path"`
	ServerName string `json:"server_name" yaml:"server_name"`
}

type AmneziaWGSettings struct {
	Enabled          bool   `json:"enabled" yaml:"enabled"`
	Port             int    `json:"port" yaml:"port"`
	PrivateKey       string `json:"private_key" yaml:"private_key"`
	PublicKey        string `json:"public_key" yaml:"public_key"`
	Address          string `json:"address" yaml:"address"`
	ClientPublicKey  string `json:"client_public_key" yaml:"client_public_key"`
	ClientPrivateKey string `json:"client_private_key" yaml:"client_private_key"`
	Jc               int    `json:"jc" yaml:"jc"`
	Jf               int    `json:"jf" yaml:"jf"`
	Jd               int    `json:"jd" yaml:"jd"`
	Jmin             int    `json:"jmin" yaml:"jmin"`
	Jmax             int    `json:"jmax" yaml:"jmax"`
}

type DNSConfig struct {
	Mode           string `json:"mode" yaml:"mode"`
	BlockSystemDNS bool   `json:"block_system_dns" yaml:"block_system_dns"`
	BlockIPv6Leaks bool   `json:"block_ipv6_leaks" yaml:"block_ipv6_leaks"`
}

type RoutingConfig struct {
	Mode        string `json:"mode" yaml:"mode"`
	DefaultExit string `json:"default_exit" yaml:"default_exit"`
}

func DefaultServerConfig(domain string, baseDir string) MirageConfig {
	return MirageConfig{
		Version: 1,
		Server:  ServerConfig{Domain: domain, BaseDir: baseDir, HTTPPort: 80, HTTPSPort: 443, UDPPort: 443},
		Transports: TransportSettings{
			Hysteria2:        Hysteria2Settings{Enabled: true, Port: 443},
			XrayRealityXHTTP: XrayRealityXHTTPSettings{Enabled: true, Port: 443, Path: "/api/session", ServerName: "www.microsoft.com"},
			AmneziaWG:        AmneziaWGSettings{Enabled: true, Port: 51820, Address: "10.77.0.1/24"},
		},
		DNS:     DNSConfig{Mode: "secure", BlockSystemDNS: true, BlockIPv6Leaks: true},
		Routing: RoutingConfig{Mode: "auto", DefaultExit: "foreign"},
	}
}
