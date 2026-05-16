package app

import (
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"mirage/internal/bundle"
	"mirage/internal/diagnostics"
	"mirage/internal/health"
	"mirage/internal/healthcheck"
	"mirage/internal/modes"
	"mirage/internal/platform"
	"mirage/internal/profiles"
	"mirage/internal/singbox"
	"mirage/internal/sysproxy"
	"mirage/internal/templates"
	"mirage/internal/xrayclient"
)

const defaultClientMode = modes.Normal
const defaultClientProtocol = "auto"
const defaultRussianDirect = false

const (
	ClientProtocolAuto      = "auto"
	ClientProtocolHysteria2 = "hysteria2"
	ClientProtocolXray      = "xray_reality_xhttp"
	ClientProtocolAmneziaWG = "amneziawg"
)

var xraySidecarCmd *exec.Cmd

func RunClient(args []string, stdout io.Writer, stderr io.Writer) error {
	cmd := "status"
	if len(args) > 0 {
		cmd = args[0]
	}

	switch cmd {
	case "status":
		return ClientStatusGUI(stdout)
	case "import":
		if len(args) < 2 {
			return fmt.Errorf("usage: mirage-client import <mirage-link-or-file>")
		}
		return ImportProfileGUI(args[1], stdout)
	case "mode":
		if len(args) < 2 {
			return fmt.Errorf("usage: mirage-client mode <normal|cheap|full>")
		}
		base := platform.ClientBaseDir()
		link, err := ReadProfileLink(base)
		if err != nil {
			return fmt.Errorf("no imported profile; run mirage-client import first")
		}
		b, err := activeBundle(base, link)
		if err != nil {
			return err
		}
		return SetClientModeGUI(base, args[1], b)
	case "ru-direct":
		if len(args) < 2 {
			return fmt.Errorf("usage: mirage-client ru-direct <on|off>")
		}
		enabled, err := ParseRussianDirect(args[1])
		if err != nil {
			return err
		}
		base := platform.ClientBaseDir()
		link, err := ReadProfileLink(base)
		if err != nil {
			return fmt.Errorf("no imported profile; run mirage-client import first")
		}
		b, err := activeBundle(base, link)
		if err != nil {
			return err
		}
		return SetRussianDirectGUI(base, enabled, b)
	case "connect":
		return ConnectClient(stdout, stderr)
	case "disconnect":
		return DisconnectClient(stdout)
	case "prepare-xray":
		return prepareXrayClient(stdout)
	case "doctor":
		return doctorClient(stdout)
	case "profiles":
		store := profiles.NewStore(filepath.Join(platform.ClientBaseDir(), "state"))
		items, err := store.List()
		if err != nil {
			return err
		}
		for _, item := range items {
			fmt.Fprintf(stdout, "%s\t%s\t%s\n", item.ID, item.DisplayName, item.LastStatus)
		}
		return nil
	case "use-profile":
		if len(args) < 2 {
			return fmt.Errorf("usage: mirage-client use-profile PROFILE_ID")
		}
		store := profiles.NewStore(filepath.Join(platform.ClientBaseDir(), "state"))
		return store.SetActive(args[1])
	case "remove-profile":
		if len(args) < 2 {
			return fmt.Errorf("usage: mirage-client remove-profile PROFILE_ID")
		}
		store := profiles.NewStore(filepath.Join(platform.ClientBaseDir(), "state"))
		return store.Remove(args[1])
	case "logs":
		return fmt.Errorf("client command %q not implemented yet", cmd)
	default:
		return fmt.Errorf("unknown client command %q", cmd)
	}
}

func ClientStatusGUI(stdout io.Writer) error {
	base := platform.ClientBaseDir()
	if _, err := ReadProfileLink(base); err != nil {
		_, err := fmt.Fprintln(stdout, "mirage client: no profile imported")
		return err
	}
	mode := ReadClientMode(base)
	ruDirect := russianDirectState(ReadRussianDirect(base))
	_, err := fmt.Fprintf(stdout, "mirage client: profile imported, mode=%s, ru_direct=%s\n", mode, ruDirect)
	return err
}

func ImportProfileGUI(input string, stdout io.Writer) error {
	link := input
	if !strings.HasPrefix(input, bundle.Scheme) {
		payload, err := os.ReadFile(input)
		if err != nil {
			return err
		}
		link = strings.TrimSpace(strings.TrimPrefix(string(payload), "\ufeff"))
	}
	link = strings.TrimSpace(strings.TrimPrefix(link, "\ufeff"))
	b, err := bundle.Decode(link)
	if err != nil {
		return err
	}
	base := platform.ClientBaseDir()
	for _, dir := range []string{"configs", "state", "logs", "bin"} {
		if err := os.MkdirAll(filepath.Join(base, dir), 0755); err != nil {
			return err
		}
	}
	store := profiles.NewStore(filepath.Join(base, "state"))
	if err := store.ImportLink(link); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(base, "state", "mode.txt"), []byte(defaultClientMode+"\n"), 0600); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(base, "state", "transport.txt"), []byte(defaultClientProtocol+"\n"), 0600); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(base, "state", "ru_direct.txt"), []byte(russianDirectState(defaultRussianDirect)+"\n"), 0600); err != nil {
		return err
	}
	if err := regenerateClientConfig(base, b, defaultClientMode); err != nil {
		return err
	}
	if err := generateXrayConfig(base, b); err != nil {
		fmt.Fprintf(stdout, "warning: xray config generation failed: %v\n", err)
	}
	_, err = fmt.Fprintf(stdout, "profile imported: %s\n", b.ProfileID)
	return err
}

func SetClientModeGUI(base string, modeStr string, b bundle.Bundle) error {
	m := modes.Normalize(modeStr)
	if m == modes.Super {
		return fmt.Errorf("super mode is reserved after MVP")
	}
	if err := os.WriteFile(filepath.Join(base, "state", "mode.txt"), []byte(string(m)+"\n"), 0600); err != nil {
		return err
	}
	if err := regenerateClientConfig(base, b, m); err != nil {
		return err
	}
	return nil
}

func SetRussianDirectGUI(base string, enabled bool, b bundle.Bundle) error {
	if err := os.WriteFile(filepath.Join(base, "state", "ru_direct.txt"), []byte(russianDirectState(enabled)+"\n"), 0600); err != nil {
		return err
	}
	return regenerateClientConfig(base, b, ReadClientMode(base))
}

func ConnectClient(stdout io.Writer, stderr io.Writer) error {
	base := platform.ClientBaseDir()
	cfg := filepath.Join(base, "configs", "sing-box.json")
	if _, err := os.Stat(cfg); err != nil {
		return fmt.Errorf("no imported profile; run mirage-client import first")
	}
	mode := ReadClientMode(base)
	singBox := filepath.Join(base, "bin", "sing-box.exe")
	if _, err := os.Stat(singBox); err != nil {
		_, err := fmt.Fprintf(stdout, "sing-box missing at %s\nplace sing-box.exe there and run connect again\nmode=%s\n", singBox, mode)
		return err
	}
	if modes.IsTun(mode) && !sysproxy.IsAdmin() {
		return fmt.Errorf("tun mode requires administrator rights; restart as administrator")
	}
	fmt.Fprintf(stdout, "starting sing-box with %s\nmode=%s\n", cfg, mode)

	if !modes.IsTun(mode) {
		if err := sysproxy.Set("127.0.0.1:2080"); err != nil {
			fmt.Fprintf(stdout, "warning: could not set system proxy: %v\n", err)
		}
		defer sysproxy.Unset()
	}

	xraySidecarCmd, _ = startXraySidecar(base, stdout)
	defer func() {
		if xraySidecarCmd != nil && xraySidecarCmd.Process != nil {
			xraySidecarCmd.Process.Kill()
			xraySidecarCmd = nil
		}
	}()

	cmd := exec.Command(singBox, "run", "-c", cfg)
	cmd.SysProcAttr = hiddenProcessAttrs()
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Start(); err != nil {
		return err
	}

	// Start background health checker
	link, _ := ReadProfileLink(base)
	b, _ := activeBundle(base, link)
	checker := newHealthChecker(b, mode)
	if checker != nil {
		checker.Start()
		defer checker.Stop()
		go func() {
			if mode != modes.Cheap {
				return
			}
			ticker := time.NewTicker(20 * time.Second)
			defer ticker.Stop()
			for range ticker.C {
				outbounds, _ := buildSingBoxOutbounds(b)
				if next, ok := checker.ShouldAutoSwitch(chooseActiveTag(outbounds)); ok {
					fmt.Fprintf(stdout, "auto-switch: transport %s appears unhealthy, switching to %s\n", chooseActiveTag(outbounds), next)
					_ = SetClientModeGUI(base, string(modes.Cheap), b)
					_ = DisconnectClient(stdout)
					_ = ConnectClient(stdout, stderr)
					return
				}
			}
		}()
	}

	return cmd.Wait()
}

func newHealthChecker(b bundle.Bundle, mode modes.Mode) *healthcheck.Checker {
	endpoints := make([]healthcheck.TransportEndpoint, 0, 3)
	if v, ok := b.Transports["hysteria2"].(map[string]any); ok {
		server, _ := v["server"].(string)
		port, _ := v["port"].(float64)
		endpoints = append(endpoints, healthcheck.TransportEndpoint{Name: "hysteria2", Server: server, Port: int(port), Network: "udp", ProxyPort: -1})
	}
	if v, ok := b.Transports["xray_reality_xhttp"].(map[string]any); ok {
		server, _ := v["server"].(string)
		port, _ := v["port"].(float64)
		endpoints = append(endpoints, healthcheck.TransportEndpoint{Name: "xray_reality_xhttp", Server: server, Port: int(port), ProxyPort: 2081})
	}
	if v, ok := b.Transports["amneziawg"].(map[string]any); ok {
		server, _ := v["server"].(string)
		port, _ := v["port"].(float64)
		endpoints = append(endpoints, healthcheck.TransportEndpoint{Name: "amneziawg", Server: server, Port: int(port), Network: "udp", ProxyPort: -1})
	}
	if len(endpoints) == 0 {
		return nil
	}
	proxy := "127.0.0.1:2080"
	if mode == modes.Cheap {
		proxy = ""
	}
	return healthcheck.New(endpoints, proxy, func(m map[string]health.TransportStatus) {
		// CLI mode: log to stdout? No-op for now to avoid noise.
	})
}

func startXraySidecar(base string, stdout io.Writer) (*exec.Cmd, error) {
	xrayBin := filepath.Join(base, "bin", "xray.exe")
	if _, err := os.Stat(xrayBin); err != nil {
		return nil, nil
	}
	cfgPath := filepath.Join(base, "configs", "xray-client.json")
	if _, err := os.Stat(cfgPath); err != nil {
		// Auto-generate config from profile if missing
		link, err := ReadProfileLink(base)
		if err != nil {
			return nil, nil
		}
		b, err := bundle.Decode(link)
		if err != nil {
			return nil, nil
		}
		if err := generateXrayConfig(base, b); err != nil {
			return nil, nil
		}
	}
	cmd := exec.Command(xrayBin, "run", "-config", cfgPath)
	cmd.SysProcAttr = hiddenProcessAttrs()
	cmd.Stdout = stdout
	cmd.Stderr = stdout
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("xray start: %w", err)
	}
	fmt.Fprintf(stdout, "xray sidecar started pid=%d\n", cmd.Process.Pid)
	go func() { _ = cmd.Wait() }()
	return cmd, nil
}

func killXray() error {
	if xraySidecarCmd != nil && xraySidecarCmd.Process != nil {
		xraySidecarCmd.Process.Kill()
		xraySidecarCmd = nil
	}
	cmd := exec.Command("taskkill", "/F", "/IM", "xray.exe")
	cmd.SysProcAttr = hiddenProcessAttrs()
	out, err := cmd.CombinedOutput()
	if err != nil && !strings.Contains(string(out), "not found") {
		return fmt.Errorf("taskkill xray: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func DisconnectClient(stdout io.Writer) error {
	if err := killSingBox(); err != nil {
		fmt.Fprintf(stdout, "warning: %v\n", err)
	}
	if err := killXray(); err != nil {
		fmt.Fprintf(stdout, "warning: %v\n", err)
	}
	if err := sysproxy.Unset(); err != nil {
		fmt.Fprintf(stdout, "warning: could not unset system proxy: %v\n", err)
	}
	_, err := fmt.Fprintln(stdout, "disconnected")
	return err
}

func killSingBox() error {
	cmd := exec.Command("taskkill", "/F", "/IM", "sing-box.exe")
	cmd.SysProcAttr = hiddenProcessAttrs()
	out, err := cmd.CombinedOutput()
	if err != nil && !strings.Contains(string(out), "not found") {
		return fmt.Errorf("taskkill: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func doctorClient(stdout io.Writer) error {
	base := platform.ClientBaseDir()
	results := []diagnostics.Result{}

	link, profileErr := ReadProfileLink(base)
	if profileErr != nil {
		results = append(results, diagnostics.Result{Name: "profile", OK: false, Fix: "run mirage-client import"})
	} else {
		results = append(results, diagnostics.Result{Name: "profile", OK: true})
	}

	mode := ReadClientMode(base)
	if mode == modes.Normal {
		results = append(results, diagnostics.Result{Name: "mode", OK: true})
	} else {
		results = append(results, diagnostics.Result{Name: "mode", OK: true, Fix: fmt.Sprintf("current=%s", mode)})
	}

	singBox := filepath.Join(base, "bin", "sing-box.exe")
	if _, err := os.Stat(singBox); err != nil {
		results = append(results, diagnostics.Result{Name: "sing-box.exe", OK: false, Fix: "place sing-box.exe in bin directory"})
	} else {
		results = append(results, diagnostics.Result{Name: "sing-box.exe", OK: true})
	}

	xrayBin := filepath.Join(base, "bin", "xray.exe")
	if _, err := os.Stat(xrayBin); err != nil {
		results = append(results, diagnostics.Result{Name: "xray.exe", OK: false, Fix: "place xray.exe in bin directory"})
	} else {
		results = append(results, diagnostics.Result{Name: "xray.exe", OK: true})
	}

	xrayCfg := filepath.Join(base, "configs", "xray-client.json")
	if _, err := os.Stat(xrayCfg); err != nil {
		results = append(results, diagnostics.Result{Name: "xray-client.json", OK: false, Fix: "run mirage-client import"})
	} else {
		results = append(results, diagnostics.Result{Name: "xray-client.json", OK: true})
	}

	cfg := filepath.Join(base, "configs", "sing-box.json")
	if _, err := os.Stat(cfg); err != nil {
		results = append(results, diagnostics.Result{Name: "sing-box.json", OK: false, Fix: "run mirage-client import or mode"})
	} else {
		results = append(results, diagnostics.Result{Name: "sing-box.json", OK: true})
	}

	if !modes.IsTun(mode) {
		if conn, err := net.DialTimeout("tcp", "127.0.0.1:2080", time.Second); err == nil {
			conn.Close()
			results = append(results, diagnostics.Result{Name: "local-2080", OK: true})
		} else {
			results = append(results, diagnostics.Result{Name: "local-2080", OK: false, Fix: "run connect to start sing-box"})
		}
		if conn, err := net.DialTimeout("tcp", "127.0.0.1:2081", time.Second); err == nil {
			conn.Close()
			results = append(results, diagnostics.Result{Name: "local-2081-xray", OK: true})
		} else {
			results = append(results, diagnostics.Result{Name: "local-2081-xray", OK: false, Fix: "xray sidecar not running; will auto-start on connect"})
		}
	} else {
		results = append(results, diagnostics.Result{Name: "tun-mode", OK: true, Fix: "verify traffic routes through tunnel"})
	}

	if profileErr == nil {
		b, _ := activeBundle(base, link)
		server := b.ServerHost()
		if server != "" {
			if conn, err := net.DialTimeout("tcp", net.JoinHostPort(server, "443"), 5*time.Second); err == nil {
				conn.Close()
				results = append(results, diagnostics.Result{Name: "server-tcp-443", OK: true})
			} else {
				results = append(results, diagnostics.Result{Name: "server-tcp-443", OK: false, Fix: "check server firewall / service"})
			}
		}
	}

	for _, r := range results {
		fmt.Fprintln(stdout, r.Line())
	}
	return nil
}

func generateXrayConfig(base string, b bundle.Bundle) error {
	v, ok := b.Transports["xray_reality_xhttp"].(map[string]any)
	if !ok {
		return nil
	}
	server, _ := v["server"].(string)
	portFloat, _ := v["port"].(float64)
	uuid, _ := v["uuid"].(string)
	publicKey, _ := v["public_key"].(string)
	shortID, _ := v["short_id"].(string)
	path, _ := v["path"].(string)
	serverName, _ := v["server_name"].(string)
	cfg, err := xrayclient.Config(xrayclient.ConfigInput{
		Server:     server,
		Port:       int(portFloat),
		UUID:       uuid,
		PublicKey:  publicKey,
		ShortID:    shortID,
		Path:       path,
		ServerName: serverName,
		LocalPort:  2081,
	})
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(base, "configs", "xray-client.json"), []byte(cfg), 0600)
}

func prepareXrayClient(stdout io.Writer) error {
	base := platform.ClientBaseDir()
	link, err := ReadProfileLink(base)
	if err != nil {
		return fmt.Errorf("no imported profile; run mirage-client import first")
	}
	b, err := activeBundle(base, link)
	if err != nil {
		return err
	}
	v, ok := b.Transports["xray_reality_xhttp"].(map[string]any)
	if !ok {
		return fmt.Errorf("xray_reality_xhttp transport not found in profile")
	}
	server, _ := v["server"].(string)
	portFloat, _ := v["port"].(float64)
	uuid, _ := v["uuid"].(string)
	publicKey, _ := v["public_key"].(string)
	shortID, _ := v["short_id"].(string)
	path, _ := v["path"].(string)
	serverName, _ := v["server_name"].(string)
	cfg, err := xrayclient.Config(xrayclient.ConfigInput{
		Server:     server,
		Port:       int(portFloat),
		UUID:       uuid,
		PublicKey:  publicKey,
		ShortID:    shortID,
		Path:       path,
		ServerName: serverName,
		LocalPort:  2081,
	})
	if err != nil {
		return err
	}
	cfgPath := filepath.Join(base, "configs", "xray-client.json")
	if err := os.WriteFile(cfgPath, []byte(cfg), 0600); err != nil {
		return err
	}
	_, err = fmt.Fprintf(stdout, "xray client config written to %s\n", cfgPath)
	return err
}

// RegenerateConfig regenerates the sing-box config from current state files.
// Called on every Connect to ensure config matches protocol/russianDirect/mode.
func RegenerateConfig(base string, b bundle.Bundle, mode modes.Mode) error {
	return regenerateClientConfig(base, b, mode)
}

func applyCamouflageOverride(b *bundle.Bundle, site string) {
	site = strings.TrimSpace(site)
	if site == "" {
		return
	}
	if b.Server == nil {
		b.Server = map[string]any{}
	}
	b.Server["camouflage_site"] = site
	if xray, ok := b.Transports["xray_reality_xhttp"].(map[string]any); ok {
		xray["server_name"] = site
	}
}

func activeBundle(base string, link string) (bundle.Bundle, error) {
	b, err := bundle.Decode(link)
	if err != nil {
		return bundle.Bundle{}, err
	}
	store := profiles.NewStore(filepath.Join(base, "state"))
	if active, err := store.Active(); err == nil {
		applyCamouflageOverride(&b, active.CamouflageSite())
	}
	return b, nil
}

func regenerateClientConfig(base string, b bundle.Bundle, mode modes.Mode) error {
	outbounds, endpoints := buildSingBoxOutbounds(b)
	protocol := ReadClientProtocol(base)
	russianDirect := ReadRussianDirect(base)
	outbounds, endpoints = keepOnlyRouteTag(outbounds, endpoints, protocol)
	active := chooseActiveTag(outbounds)
	if len(endpoints) > 0 && active == "direct" {
		if tag, ok := endpoints[0]["tag"].(string); ok {
			active = tag
		}
	}
	if russianDirect && protocol != ClientProtocolAuto && protocol != ClientProtocolXray {
		outbounds = ensureXrayDownloadOutbound(outbounds, b)
	}

	var cfg string
	var err error
	useURLTest := protocol == ClientProtocolAuto
	switch mode {
	case modes.Cheap:
		if useURLTest {
			active = chooseHealthyActiveTag(outbounds, endpoints)
			outbounds, endpoints = keepOnlyTag(outbounds, endpoints, active)
		}
		if modes.IsTun(mode) {
			cfg, err = templates.SingBoxTunConfigWithRouting(active, outbounds, endpoints, false, russianDirect)
		} else {
			cfg, err = templates.SingBoxConfigWithRouting(active, outbounds, endpoints, false, russianDirect)
		}
	case modes.Normal:
		if useURLTest {
			cfg, err = templates.SingBoxConfigWithRouting("auto", outbounds, endpoints, true, russianDirect)
		} else {
			cfg, err = templates.SingBoxConfigWithRouting(active, outbounds, endpoints, false, russianDirect)
		}
	case modes.Full:
		if useURLTest {
			cfg, err = templates.SingBoxTunConfigWithRouting("auto", outbounds, endpoints, true, russianDirect)
		} else {
			cfg, err = templates.SingBoxTunConfigWithRouting(active, outbounds, endpoints, false, russianDirect)
		}
	default:
		if modes.IsTun(mode) {
			cfg, err = templates.SingBoxTunConfigWithRouting(active, outbounds, endpoints, false, russianDirect)
		} else {
			cfg, err = templates.SingBoxConfigWithRouting(active, outbounds, endpoints, false, russianDirect)
		}
	}
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(base, "configs", "sing-box.json"), []byte(cfg), 0600)
}

func ReadProfileLink(base string) (string, error) {
	store := profiles.NewStore(filepath.Join(base, "state"))
	profile, err := store.Active()
	if err == nil {
		return profile.Link, nil
	}
	payload, readErr := os.ReadFile(filepath.Join(base, "state", "profile.link"))
	if readErr != nil {
		return "", err
	}
	return strings.TrimSpace(strings.TrimPrefix(string(payload), "\ufeff")), nil
}

func ReadClientMode(base string) modes.Mode {
	payload, err := os.ReadFile(filepath.Join(base, "state", "mode.txt"))
	if err != nil {
		return defaultClientMode
	}
	return modes.Normalize(strings.TrimSpace(string(payload)))
}

func ParseRussianDirect(value string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "on", "true", "1", "yes", "direct":
		return true, nil
	case "off", "false", "0", "no", "proxy":
		return false, nil
	default:
		return false, fmt.Errorf("usage: mirage-client ru-direct <on|off>")
	}
}

func ReadRussianDirect(base string) bool {
	payload, err := os.ReadFile(filepath.Join(base, "state", "ru_direct.txt"))
	if err != nil {
		return defaultRussianDirect
	}
	enabled, err := ParseRussianDirect(string(payload))
	if err != nil {
		return defaultRussianDirect
	}
	return enabled
}

func russianDirectState(enabled bool) string {
	if enabled {
		return "on"
	}
	return "off"
}

func NormalizeClientProtocol(value string) string {
	switch strings.TrimSpace(value) {
	case ClientProtocolHysteria2:
		return ClientProtocolHysteria2
	case ClientProtocolXray, "xray":
		return ClientProtocolXray
	case ClientProtocolAmneziaWG:
		return ClientProtocolAmneziaWG
	case ClientProtocolAuto:
		return ClientProtocolAuto
	default:
		return ClientProtocolAuto
	}
}

func ReadClientProtocol(base string) string {
	payload, err := os.ReadFile(filepath.Join(base, "state", "transport.txt"))
	if err != nil {
		return defaultClientProtocol
	}
	return NormalizeClientProtocol(string(payload))
}

func SetClientProtocol(base string, protocol string) error {
	return os.WriteFile(filepath.Join(base, "state", "transport.txt"), []byte(NormalizeClientProtocol(protocol)+"\n"), 0600)
}

func chooseActiveTag(outbounds []map[string]any) string {
	for _, preferred := range []string{"hysteria2", "xray_reality_xhttp", "amneziawg"} {
		for _, outbound := range outbounds {
			if outbound["tag"] == preferred {
				return preferred
			}
		}
	}
	return "direct"
}

func chooseHealthyActiveTag(outbounds []map[string]any, endpoints []map[string]any) string {
	fallback := chooseActiveTag(outbounds)
	candidates := routeCandidates(outbounds, endpoints)
	for _, preferred := range []string{"xray_reality_xhttp", "hysteria2", "amneziawg"} {
		for _, candidate := range candidates {
			if candidate["tag"] != preferred {
				continue
			}
			if outboundReachable(candidate) {
				return preferred
			}
		}
	}
	return fallback
}

func routeCandidates(outbounds []map[string]any, endpoints []map[string]any) []map[string]any {
	items := make([]map[string]any, 0, len(outbounds)+len(endpoints))
	items = append(items, outbounds...)
	for _, endpoint := range endpoints {
		peer := endpointPeer(endpoint)
		if peer == nil {
			continue
		}
		items = append(items, map[string]any{"tag": endpoint["tag"], "server": peer["address"], "server_port": peer["port"]})
	}
	return items
}

func endpointPeer(endpoint map[string]any) map[string]any {
	peers, ok := endpoint["peers"].([]any)
	if !ok || len(peers) == 0 {
		return nil
	}
	peer, _ := peers[0].(map[string]any)
	return peer
}

func outboundReachable(outbound map[string]any) bool {
	server, ok := outbound["server"].(string)
	if !ok || server == "" {
		return false
	}
	port, ok := outboundPort(outbound["server_port"])
	if !ok {
		return false
	}
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(server, strconv.Itoa(port)), 800*time.Millisecond)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

func outboundPort(value any) (int, bool) {
	switch v := value.(type) {
	case int:
		return v, true
	case float64:
		return int(v), true
	case string:
		port, err := strconv.Atoi(v)
		return port, err == nil
	default:
		return 0, false
	}
}

func keepOnlyTag(outbounds []map[string]any, endpoints []map[string]any, tag string) ([]map[string]any, []map[string]any) {
	for _, outbound := range outbounds {
		if outbound["tag"] == tag {
			return []map[string]any{outbound}, endpoints
		}
	}
	for _, endpoint := range endpoints {
		if endpoint["tag"] == tag {
			return nil, []map[string]any{endpoint}
		}
	}
	return outbounds, endpoints
}

func keepOnlyRouteTag(outbounds []map[string]any, endpoints []map[string]any, tag string) ([]map[string]any, []map[string]any) {
	if tag == ClientProtocolAuto {
		return outbounds, endpoints
	}
	for _, outbound := range outbounds {
		if outbound["tag"] == tag {
			return []map[string]any{outbound}, nil
		}
	}
	for _, endpoint := range endpoints {
		if endpoint["tag"] == tag {
			return nil, []map[string]any{endpoint}
		}
	}
	return outbounds, endpoints
}

func xraySOCKSOutbound() map[string]any {
	return map[string]any{"type": "socks", "tag": "xray_reality_xhttp", "server": "127.0.0.1", "server_port": 2081}
}

func ensureXrayDownloadOutbound(outbounds []map[string]any, b bundle.Bundle) []map[string]any {
	if _, ok := b.Transports["xray_reality_xhttp"].(map[string]any); !ok {
		return outbounds
	}
	for _, o := range outbounds {
		if o["tag"] == "xray_reality_xhttp" {
			return outbounds
		}
	}
	return append(outbounds, xraySOCKSOutbound())
}

func buildSingBoxOutbounds(b bundle.Bundle) ([]map[string]any, []map[string]any) {
	items := make([]map[string]any, 0, 3)
	var endpoints []map[string]any
	if v, ok := b.Transports["hysteria2"].(map[string]any); ok {
		items = append(items, map[string]any{"type": "hysteria2", "tag": "hysteria2", "server": v["server"], "server_port": v["port"], "password": v["password"], "tls": map[string]any{"enabled": true, "insecure": true}})
	}
	if _, ok := b.Transports["xray_reality_xhttp"].(map[string]any); ok {
		items = append(items, map[string]any{"type": "socks", "tag": "xray_reality_xhttp", "server": "127.0.0.1", "server_port": 2081})
	}
	if v, ok := b.Transports["amneziawg"].(map[string]any); ok {
		pubKey, _ := v["public_key"].(string)
		server, _ := v["server"].(string)
		port, _ := v["port"].(float64)
		addr, _ := v["address"].(string)
		clientPrivKey, _ := v["client_private_key"].(string)
		if pubKey != "" && clientPrivKey != "" {
			ep := singbox.BuildWireGuardEndpoint(server, int(port), clientPrivKey, pubKey, addr)
			endpoints = append(endpoints, ep)
		}
	}
	return items, endpoints
}
