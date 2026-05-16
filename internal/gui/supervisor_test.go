package gui

import (
	"net"
	"testing"
)

func TestIsTCPListeningDetectsOpenPort(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	if !isTCPListening(listener.Addr().String()) {
		t.Fatalf("expected %s to be detected as listening", listener.Addr().String())
	}
}

func TestIsTCPListeningReturnsFalseForClosedPort(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := listener.Addr().String()
	listener.Close()

	if isTCPListening(addr) {
		t.Fatalf("expected %s to be detected as closed", addr)
	}
}

func TestSupervisorStartRefusesExistingMixedProxyPort(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:2080")
	if err != nil {
		t.Skipf("127.0.0.1:2080 already in use: %v", err)
	}
	defer listener.Close()

	var s Supervisor
	if err := s.Start("missing-sing-box.exe", "missing-config.json", "normal"); err == nil || err.Error() != "already running" {
		t.Fatalf("Start error = %v, want already running", err)
	}
}
