package app

import (
	"context"
	"crypto/ecdh"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"io"
	"math/big"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"gopkg.in/yaml.v3"

	"mirage/internal/bundle"
	"mirage/internal/config"
	"mirage/internal/diagnostics"
	"mirage/internal/downloader"
	"mirage/internal/platform"
	"mirage/internal/serverhost"
	"mirage/internal/sidecar"
	"mirage/internal/templates"
	"mirage/internal/transports"
	"mirage/internal/transports/amneziawg"
	"mirage/internal/transports/hysteria2"
	"mirage/internal/transports/xrayrealityxhttp"
)

func RunServer(args []string, stdout io.Writer, stderr io.Writer) error {
	cmd := "status"
	if len(args) > 0 {
		cmd = args[0]
	}

	switch cmd {
	case "status":
		return serverStatus(stdout)
	case "setup":
		parsed := parseServerSetupArgs(args[1:])
		return setupServer(parsed.Host, parsed.Auto, stdout)
	case "show-link":
		return showLink(stdout)
	case "start":
		return startServer(stdout, stderr)
	case "download-sidecars":
		return downloadSidecars(args[1:], stdout)
	case "doctor":
		return doctorServer(stdout)
	case "stop", "restart", "logs", "add-client", "remove-client", "show-qr", "rotate-client":
		return fmt.Errorf("server command %q not implemented yet", cmd)
	default:
		return fmt.Errorf("unknown server command %q", cmd)
	}
}

type serverSetupArgs struct {
	Auto bool
	Host string
}

func parseServerSetupArgs(args []string) serverSetupArgs {
	parsed := serverSetupArgs{}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--auto":
			parsed.Auto = true
		case "--host":
			if i+1 < len(args) {
				parsed.Host = args[i+1]
				i++
			}
		default:
			if parsed.Host == "" {
				parsed.Host = args[i]
			}
		}
	}
	return parsed
}

func serverStatus(stdout io.Writer) error {
	cfgPath := filepath.Join(platform.ServerBaseDir(), "configs", "mirage.yaml")
	if _, err := os.Stat(cfgPath); err != nil {
		_, err := fmt.Fprintln(stdout, "mirage server: not configured")
		return err
	}
	_, err := fmt.Fprintln(stdout, "mirage server: configured")
	return err
}

func setupServer(host string, auto bool, stdout io.Writer) error {
	base := platform.ServerBaseDir()
	for _, dir := range []string{"configs", "state", "logs", "web", "bin"} {
		if err := os.MkdirAll(filepath.Join(base, dir), 0755); err != nil {
			return err
		}
	}
	resolvedHost, err := serverhost.Resolve(host, nil)
	if err != nil {
		return err
	}
	cfg := config.DefaultServerConfig(resolvedHost, base)
	cfg.Clients = []config.ClientRecord{{ID: randomHex(16), Name: "default-pc"}}
	cfg.Transports.Hysteria2.Password = randomHex(24)
	cfg.Transports.XrayRealityXHTTP.UUID = randomHex(16)
	cfg.Transports.XrayRealityXHTTP.PrivateKey, cfg.Transports.XrayRealityXHTTP.PublicKey = generateXrayRealityKeys(base)
	cfg.Transports.XrayRealityXHTTP.ShortID = randomHex(4)
	serverPrivateKey, serverPublicKey, err := generateWireGuardKeyPair()
	if err != nil {
		return err
	}
	clientPrivateKey, clientPublicKey, err := generateWireGuardKeyPair()
	if err != nil {
		return err
	}
	cfg.Transports.AmneziaWG.PrivateKey = serverPrivateKey
	cfg.Transports.AmneziaWG.PublicKey = serverPublicKey
	cfg.Transports.AmneziaWG.ClientPublicKey = clientPublicKey
	cfg.Transports.AmneziaWG.ClientPrivateKey = clientPrivateKey

	payload, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(base, "configs", "mirage.yaml"), payload, 0600); err != nil {
		return err
	}
	if err := writeHysteriaCertificate(base, resolvedHost); err != nil {
		return err
	}
	if err := writeServerTransportFiles(cfg); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(base, "web", "index.html"), []byte(templates.DefaultIndexHTML()), 0644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(base, "web", "about.html"), []byte(templates.DefaultAboutHTML()), 0644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(base, "web", "robots.txt"), []byte(templates.DefaultRobotsTXT()), 0644); err != nil {
		return err
	}
	link, err := buildDefaultBundle(cfg)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(base, "state", "default.link"), []byte(link+"\n"), 0600); err != nil {
		return err
	}
	if auto {
		_, err = fmt.Fprintf(stdout, "mirage server configured at %s\nclient link written to %s\n", base, filepath.Join(base, "state", "default.link"))
		return err
	}
	_, err = fmt.Fprintf(stdout, "mirage server configured at %s\nclient link: %s\n", base, link)
	return err
}

func writeHysteriaCertificate(base string, domain string) error {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return err
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return err
	}
	tmpl := x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: domain},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:     x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	if ip := net.ParseIP(domain); ip != nil {
		tmpl.IPAddresses = []net.IP{ip}
	} else {
		tmpl.DNSNames = []string{domain}
	}
	certDER, err := x509.CreateCertificate(rand.Reader, &tmpl, &tmpl, &key.PublicKey, key)
	if err != nil {
		return err
	}
	certOut, err := os.OpenFile(filepath.Join(base, "state", "hysteria.crt"), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	if err := pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: certDER}); err != nil {
		certOut.Close()
		return err
	}
	if err := certOut.Close(); err != nil {
		return err
	}
	keyOut, err := os.OpenFile(filepath.Join(base, "state", "hysteria.key"), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	if err := pem.Encode(keyOut, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}); err != nil {
		keyOut.Close()
		return err
	}
	return keyOut.Close()
}

func writeServerTransportFiles(cfg config.MirageConfig) error {
	ctx := transports.Context{BaseDir: cfg.Server.BaseDir, Domain: cfg.Server.Domain}
	items := []transports.Transport{
		hysteria2.Transport{Port: cfg.Transports.Hysteria2.Port, Password: cfg.Transports.Hysteria2.Password},
		xrayrealityxhttp.Transport{Port: cfg.Transports.XrayRealityXHTTP.Port, UUID: cfg.Transports.XrayRealityXHTTP.UUID, PrivateKey: cfg.Transports.XrayRealityXHTTP.PrivateKey, PublicKey: cfg.Transports.XrayRealityXHTTP.PublicKey, ShortID: cfg.Transports.XrayRealityXHTTP.ShortID, Path: cfg.Transports.XrayRealityXHTTP.Path, ServerName: cfg.Transports.XrayRealityXHTTP.ServerName},
		amneziawg.Transport{Port: cfg.Transports.AmneziaWG.Port, PrivateKey: cfg.Transports.AmneziaWG.PrivateKey, PublicKey: cfg.Transports.AmneziaWG.PublicKey, Address: cfg.Transports.AmneziaWG.Address, ClientPublicKey: cfg.Transports.AmneziaWG.ClientPublicKey, ClientPrivateKey: cfg.Transports.AmneziaWG.ClientPrivateKey, Jc: cfg.Transports.AmneziaWG.Jc, Jf: cfg.Transports.AmneziaWG.Jf, Jd: cfg.Transports.AmneziaWG.Jd, Jmin: cfg.Transports.AmneziaWG.Jmin, Jmax: cfg.Transports.AmneziaWG.Jmax},
	}
	for _, item := range items {
		files, err := item.ServerFiles(ctx)
		if err != nil {
			return err
		}
		for _, file := range files {
			if err := os.WriteFile(file.Path, []byte(file.Content), os.FileMode(file.Mode)); err != nil {
				return err
			}
		}
	}
	return nil
}

func showLink(stdout io.Writer) error {
	payload, err := os.ReadFile(filepath.Join(platform.ServerBaseDir(), "state", "default.link"))
	if err != nil {
		return err
	}
	_, err = stdout.Write(payload)
	return err
}

func startServer(stdout io.Writer, stderr io.Writer) error {
	base := platform.ServerBaseDir()
	cfgPath := filepath.Join(base, "configs", "mirage.yaml")
	if _, err := os.Stat(cfgPath); err != nil {
		return fmt.Errorf("server not configured; run mirage-server setup --auto")
	}
	processes := []sidecar.Process{
		{Name: "xray", Path: filepath.Join(base, "bin", binaryName("xray")), Args: []string{"run", "-config", filepath.Join(base, "configs", "xray.json")}},
		{Name: "hysteria", Path: filepath.Join(base, "bin", binaryName("hysteria")), Args: []string{"server", "-c", filepath.Join(base, "configs", "hysteria.yaml")}},
		{Name: "amneziawg", Path: filepath.Join(base, "bin", binaryName("amneziawg")), Args: []string{"-f", "awg0"}},
	}
	started := 0
	ctx := context.Background()
	for _, proc := range processes {
		if _, err := os.Stat(proc.Path); err != nil {
			fmt.Fprintf(stdout, "%s missing at %s\n", proc.Name, proc.Path)
			continue
		}
		cmd, err := proc.Start(ctx, stdout, stderr)
		if err != nil {
			fmt.Fprintf(stderr, "%s failed to start: %v\n", proc.Name, err)
			continue
		}
		started++
		fmt.Fprintf(stdout, "%s started pid=%d\n", proc.Name, cmd.Process.Pid)
	}
	if started == 0 {
		return fmt.Errorf("no sidecar binaries started; place xray, hysteria, amneziawg in %s", filepath.Join(base, "bin"))
	}
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	return nil
}

func downloadSidecars(args []string, stdout io.Writer) error {
	manifest := downloader.RuntimeManifest()
	if len(manifest.Entries) == 0 {
		manifest = downloader.DefaultManifest("linux", "amd64")
	}
	if len(args) > 0 && args[0] == "--print-manifest" {
		for _, entry := range manifest.Entries {
			fmt.Fprintf(stdout, "%s target=%s url=%s env=%s\n", entry.Name, entry.TargetName, entry.URL, entry.EnvURL)
		}
		return nil
	}
	return downloader.DownloadAll(context.Background(), manifest, filepath.Join(platform.ServerBaseDir(), "bin"))
}

func doctorServer(stdout io.Writer) error {
	base := platform.ServerBaseDir()
	results := []diagnostics.Result{}

	for _, name := range []string{"xray", "hysteria"} {
		path := filepath.Join(base, "bin", binaryName(name))
		if _, err := os.Stat(path); err != nil {
			results = append(results, diagnostics.Result{Name: name, OK: false, Fix: fmt.Sprintf("place %s in %s", name, filepath.Join(base, "bin"))})
		} else {
			results = append(results, diagnostics.Result{Name: name, OK: true})
		}
	}

	amneziawgPath := filepath.Join(base, "bin", binaryName("amneziawg"))
	if _, err := os.Stat(amneziawgPath); err != nil {
		results = append(results, diagnostics.Result{Name: "amneziawg", OK: true, Fix: "experimental - optional"})
	} else {
		results = append(results, diagnostics.Result{Name: "amneziawg", OK: true})
	}

	for _, name := range []string{"mirage.yaml", "xray.json", "hysteria.yaml"} {
		path := filepath.Join(base, "configs", name)
		if _, err := os.Stat(path); err != nil {
			results = append(results, diagnostics.Result{Name: name, OK: false, Fix: fmt.Sprintf("run mirage-server setup")})
		} else {
			results = append(results, diagnostics.Result{Name: name, OK: true})
		}
	}

	if conn, err := net.DialTimeout("tcp", "127.0.0.1:443", 2*time.Second); err == nil {
		conn.Close()
		results = append(results, diagnostics.Result{Name: "tcp-443", OK: true})
	} else {
		results = append(results, diagnostics.Result{Name: "tcp-443", OK: false, Fix: "check firewall / sidecar start"})
	}

	if conn, err := net.DialTimeout("udp", "127.0.0.1:443", 2*time.Second); err == nil {
		conn.Close()
		results = append(results, diagnostics.Result{Name: "udp-443", OK: true})
	} else {
		results = append(results, diagnostics.Result{Name: "udp-443", OK: false, Fix: "check firewall / sidecar start"})
	}

	for _, r := range results {
		fmt.Fprintln(stdout, r.Line())
	}
	return nil
}

func buildDefaultBundle(cfg config.MirageConfig) (string, error) {
	return bundle.Encode(bundle.Bundle{
		Version:     1,
		ProfileID:   "main-vps",
		DisplayName: "Main VPS",
		Server: map[string]any{
			"host":            cfg.Server.Domain,
			"domain":          "",
			"health_url":      "https://" + cfg.Server.Domain + "/api/mirage/health",
			"camouflage_site": "www.microsoft.com",
		},
		Client: map[string]any{"id": cfg.Clients[0].ID, "name": cfg.Clients[0].Name},
		Transports: map[string]any{
			"hysteria2":          map[string]any{"server": cfg.Server.Domain, "port": cfg.Transports.Hysteria2.Port, "password": cfg.Transports.Hysteria2.Password},
			"xray_reality_xhttp": map[string]any{"server": cfg.Server.Domain, "port": cfg.Transports.XrayRealityXHTTP.Port, "uuid": cfg.Transports.XrayRealityXHTTP.UUID, "public_key": cfg.Transports.XrayRealityXHTTP.PublicKey, "short_id": cfg.Transports.XrayRealityXHTTP.ShortID, "path": cfg.Transports.XrayRealityXHTTP.Path, "server_name": cfg.Transports.XrayRealityXHTTP.ServerName},
			"amneziawg":          map[string]any{"server": cfg.Server.Domain, "port": cfg.Transports.AmneziaWG.Port, "public_key": cfg.Transports.AmneziaWG.PublicKey, "address": "10.77.0.2/32", "client_private_key": cfg.Transports.AmneziaWG.ClientPrivateKey, "jc": cfg.Transports.AmneziaWG.Jc, "jf": cfg.Transports.AmneziaWG.Jf, "jd": cfg.Transports.AmneziaWG.Jd, "jmin": cfg.Transports.AmneziaWG.Jmin, "jmax": cfg.Transports.AmneziaWG.Jmax},
		},
		DNS:     map[string]any{"mode": cfg.DNS.Mode, "block_system_dns": cfg.DNS.BlockSystemDNS, "block_ipv6_leaks": cfg.DNS.BlockIPv6Leaks},
		Routing: map[string]any{"mode": cfg.Routing.Mode, "default_exit": cfg.Routing.DefaultExit, "client_mode": "normal"},
	})
}

func generateWireGuardKeyPair() (string, string, error) {
	privateKey, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		return "", "", err
	}
	return base64.StdEncoding.EncodeToString(privateKey.Bytes()), base64.StdEncoding.EncodeToString(privateKey.PublicKey().Bytes()), nil
}

func generateXrayRealityKeys(base string) (string, string) {
	xrayPath := filepath.Join(base, "bin", binaryName("xray"))
	out, err := exec.Command(xrayPath, "x25519").CombinedOutput()
	if err != nil {
		return randomHex(32), randomHex(32)
	}
	var privateKey, publicKey string
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.SplitN(line, ":", 2)
		if len(fields) != 2 {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(fields[0]))
		value := strings.TrimSpace(fields[1])
		if strings.Contains(key, "private") {
			privateKey = value
		}
		if strings.Contains(key, "public") {
			publicKey = value
		}
	}
	if privateKey == "" || publicKey == "" {
		return randomHex(32), randomHex(32)
	}
	return privateKey, publicKey
}

func randomHex(bytes int) string {
	buf := make([]byte, bytes)
	if _, err := rand.Read(buf); err != nil {
		panic(err)
	}
	return hex.EncodeToString(buf)
}

func binaryName(name string) string {
	if filepath.Separator == '\\' {
		return name + ".exe"
	}
	return name
}
