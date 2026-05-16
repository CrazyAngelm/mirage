package probe

import (
	"net"
	"time"
)

type Status string

const (
	Unknown Status = "unknown"
	Online  Status = "online"
	Slow    Status = "slow"
	Offline Status = "offline"
)

type Result struct {
	Status    Status
	LatencyMS int
	Error     string
}

func TCP(address string, timeout time.Duration) Result {
	start := time.Now()
	conn, err := net.DialTimeout("tcp", address, timeout)
	latency := time.Since(start)
	if conn != nil {
		conn.Close()
	}
	return Classify(latency, err)
}

func Classify(latency time.Duration, err error) Result {
	if err != nil {
		return Result{Status: Offline, Error: err.Error()}
	}
	ms := int(latency.Milliseconds())
	if ms < 1 {
		ms = 1
	}
	if latency > 750*time.Millisecond {
		return Result{Status: Slow, LatencyMS: ms}
	}
	return Result{Status: Online, LatencyMS: ms}
}
