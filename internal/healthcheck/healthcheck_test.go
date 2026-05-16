package healthcheck

import (
	"context"
	"net"
	"strconv"
	"testing"

	"mirage/internal/health"
)

func TestUDPOnlyTransportIsNotMarkedReachableByTCPPort(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	go func() {
		conn, err := listener.Accept()
		if err == nil {
			conn.Close()
		}
	}()
	_, port, err := net.SplitHostPort(listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}

	checker := New([]TransportEndpoint{{Name: "hysteria2", Server: "127.0.0.1", Port: mustAtoi(port), Network: "udp", ProxyPort: -1}}, "", nil)
	checker.checkOne(context.Background(), checker.transport[0])
	status := checker.Statuses()["hysteria2"]
	if status.Status == health.StatusReachable {
		t.Fatalf("status = %+v, want not reachable", status)
	}
}

func mustAtoi(value string) int {
	port, err := strconv.Atoi(value)
	if err != nil {
		panic(err)
	}
	return port
}
