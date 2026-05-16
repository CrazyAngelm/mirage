package gui

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"mirage/internal/bundle"
	"mirage/internal/health"
	"mirage/internal/healthcheck"
	"mirage/internal/modes"
	"mirage/internal/platform"
)

// Supervisor manages the sing-box child process for GUI mode.
type Supervisor struct {
	mu       sync.Mutex
	cmd      *exec.Cmd
	xrayCmd  *exec.Cmd
	cancel   context.CancelFunc
	running  bool
	mode     modes.Mode
	checker  *healthcheck.Checker
	statuses map[string]health.TransportStatus
	statusMu sync.RWMutex
}

func (s *Supervisor) Start(singBoxPath, configPath string, mode modes.Mode) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.running || isTCPListening("127.0.0.1:2080") {
		return fmt.Errorf("already running")
	}

	ctx, cancel := context.WithCancel(context.Background())
	logPath := filepath.Join(platform.ClientBaseDir(), "logs", "sing-box.log")
	_ = os.MkdirAll(filepath.Dir(logPath), 0755)
	logFile, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		cancel()
		return fmt.Errorf("open log: %w", err)
	}

	s.startXraySidecar()

	cmd := exec.CommandContext(ctx, singBoxPath, "run", "-c", configPath)
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: 0x08000000}
	cmd.Stdout = logFile
	cmd.Stderr = logFile

	if err := cmd.Start(); err != nil {
		logFile.Close()
		cancel()
		return fmt.Errorf("start sing-box: %w", err)
	}

	s.cmd = cmd
	s.cancel = cancel
	s.running = true
	s.mode = mode

	s.startHealthChecker()

	// Monitor process exit in background
	go func() {
		_ = cmd.Wait()
		logFile.Close()
		s.mu.Lock()
		s.running = false
		if s.checker != nil {
			s.checker.Stop()
			s.checker = nil
		}
		if s.xrayCmd != nil && s.xrayCmd.Process != nil {
			_ = s.xrayCmd.Process.Kill()
			s.xrayCmd = nil
		}
		s.mu.Unlock()
	}()

	// Give sing-box a moment to bind
	time.Sleep(500 * time.Millisecond)
	return nil
}

func (s *Supervisor) startXraySidecar() {
	// Always start xray if available; it serves as a reliable TCP sidecar
	// for rule-set downloads even when the active transport is UDP-based.
	base := platform.ClientBaseDir()
	xrayBin := filepath.Join(base, "bin", "xray.exe")
	if _, err := os.Stat(xrayBin); err != nil {
		return
	}
	cfgPath := filepath.Join(base, "configs", "xray-client.json")
	if _, err := os.Stat(cfgPath); err != nil {
		return
	}
	logPath := filepath.Join(base, "logs", "xray.log")
	logFile, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	cmd := exec.Command(xrayBin, "run", "-config", cfgPath)
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: 0x08000000}
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	if err := cmd.Start(); err != nil {
		logFile.Close()
		return
	}
	s.xrayCmd = cmd
	go func() {
		_ = cmd.Wait()
		logFile.Close()
	}()
}

func (s *Supervisor) Stop() {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return
	}
	if s.checker != nil {
		s.checker.Stop()
		s.checker = nil
	}
	if s.xrayCmd != nil && s.xrayCmd.Process != nil {
		_ = s.xrayCmd.Process.Kill()
		s.xrayCmd = nil
	}
	if s.cancel != nil {
		s.cancel()
	}
	if s.cmd != nil && s.cmd.Process != nil {
		_ = s.cmd.Process.Kill()
	}
	s.running = false
	s.mu.Unlock()
	waitForTCPClosed("127.0.0.1:2080", 2*time.Second)
}

func (s *Supervisor) IsRunning() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if isTCPListening("127.0.0.1:2080") {
		return true
	}
	if !s.running || s.cmd == nil || s.cmd.Process == nil {
		return false
	}
	err := s.cmd.Process.Signal(os.Signal(nil))
	return err == nil
}

func isTCPListening(addr string) bool {
	conn, err := net.DialTimeout("tcp", addr, 300*time.Millisecond)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

func waitForTCPClosed(addr string, timeout time.Duration) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if !isTCPListening(addr) {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func (s *Supervisor) Mode() modes.Mode {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.mode
}

func (s *Supervisor) startHealthChecker() {
	base := platform.ClientBaseDir()
	link, _ := readProfileLink(base)
	if link == "" {
		return
	}
	b, err := bundle.Decode(link)
	if err != nil {
		return
	}
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
		return
	}
	proxy := "127.0.0.1:2080"
	if s.mode == modes.Cheap {
		proxy = ""
	}
	s.checker = healthcheck.New(endpoints, proxy, func(m map[string]health.TransportStatus) {
		s.statusMu.Lock()
		s.statuses = m
		s.statusMu.Unlock()
	})
	s.checker.Start()
}

func (s *Supervisor) Statuses() map[string]health.TransportStatus {
	s.statusMu.RLock()
	defer s.statusMu.RUnlock()
	out := make(map[string]health.TransportStatus, len(s.statuses))
	for k, v := range s.statuses {
		out[k] = v
	}
	return out
}

func (s *Supervisor) BestTransport() (health.TransportStatus, bool) {
	if s.checker == nil {
		return health.TransportStatus{}, false
	}
	return s.checker.Best()
}

func readClientProtocol(base string) string {
	payload, err := os.ReadFile(filepath.Join(base, "state", "transport.txt"))
	if err != nil {
		return "auto"
	}
	return strings.TrimSpace(string(payload))
}

func readRussianDirect(base string) bool {
	payload, err := os.ReadFile(filepath.Join(base, "state", "ru_direct.txt"))
	if err != nil {
		return false
	}
	v := strings.TrimSpace(string(payload))
	return v == "on" || v == "true" || v == "1" || v == "yes" || v == "direct"
}
