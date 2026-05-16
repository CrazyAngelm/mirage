package probe

import (
	"net"
	"testing"
	"time"
)

func TestClassify(t *testing.T) {
	if got := Classify(50*time.Millisecond, nil); got.Status != Online {
		t.Fatalf("status = %s", got.Status)
	}
	if got := Classify(900*time.Millisecond, nil); got.Status != Slow {
		t.Fatalf("status = %s", got.Status)
	}
	if got := Classify(0, errTest{}); got.Status != Offline {
		t.Fatalf("status = %s", got.Status)
	}
}

func TestTCPProbe(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() {
		conn, _ := ln.Accept()
		if conn != nil {
			conn.Close()
		}
	}()
	result := TCP(ln.Addr().String(), time.Second)
	if result.Status == Offline {
		t.Fatalf("result = %+v", result)
	}
	if result.LatencyMS <= 0 {
		t.Fatalf("latency = %d", result.LatencyMS)
	}
}

type errTest struct{}

func (errTest) Error() string { return "boom" }
