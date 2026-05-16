package healthcheck

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"

	"mirage/internal/health"
)

func mustParseURL(raw string) *url.URL {
	u, err := url.Parse(raw)
	if err != nil {
		panic(err)
	}
	return u
}

const (
	checkInterval   = 15 * time.Second
	tcpTimeout      = 2 * time.Second
	httpTimeout     = 10 * time.Second
	failThreshold   = 3
	autoSwitchAfter = 3 // consecutive failures before auto-switch in cheap mode
)

// Checker periodically tests transport health.
type Checker struct {
	mu        sync.RWMutex
	statuses  map[string]health.TransportStatus
	results   map[string]health.Result
	transport []TransportEndpoint
	proxyAddr string // e.g. "127.0.0.1:2080" or empty
	cb        func(map[string]health.TransportStatus)
	cancel    context.CancelFunc
	wg        sync.WaitGroup
}

type TransportEndpoint struct {
	Name      string
	Server    string
	Port      int
	Network   string
	ProxyPort int // 0 = use shared proxy, >0 = use this SOCKS port, -1 = no proxy datapath check
}

func New(endpoints []TransportEndpoint, proxyAddr string, callback func(map[string]health.TransportStatus)) *Checker {
	c := &Checker{
		transport: endpoints,
		proxyAddr: proxyAddr,
		cb:        callback,
		statuses:  make(map[string]health.TransportStatus),
		results:   make(map[string]health.Result),
	}
	for _, ep := range endpoints {
		c.statuses[ep.Name] = health.TransportStatus{Name: ep.Name, Status: health.StatusUnknown}
	}
	return c
}

func (c *Checker) Start() {
	ctx, cancel := context.WithCancel(context.Background())
	c.cancel = cancel
	c.wg.Add(1)
	go c.loop(ctx)
}

func (c *Checker) Stop() {
	if c.cancel != nil {
		c.cancel()
	}
	c.wg.Wait()
}

func (c *Checker) Statuses() map[string]health.TransportStatus {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make(map[string]health.TransportStatus, len(c.statuses))
	for k, v := range c.statuses {
		out[k] = v
	}
	return out
}

func (c *Checker) Best() (health.TransportStatus, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	var best health.TransportStatus
	bestScore := -1
	for _, s := range c.statuses {
		if s.Status != health.StatusReachable {
			continue
		}
		if s.Score > bestScore {
			best = s
			bestScore = s.Score
		}
	}
	return best, bestScore >= 0
}

func (c *Checker) ShouldAutoSwitch(current string) (string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	cur, ok := c.results[current]
	if !ok || cur.Reachable && cur.RecentFailures < autoSwitchAfter {
		return "", false
	}
	best, found := health.SelectBest(sliceResults(c.results))
	if !found || best.Name == current {
		return "", false
	}
	return best.Name, true
}

func (c *Checker) loop(ctx context.Context) {
	defer c.wg.Done()
	// immediate first check
	c.runChecks(ctx)
	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.runChecks(ctx)
		}
	}
}

func (c *Checker) runChecks(ctx context.Context) {
	for _, ep := range c.transport {
		c.checkOne(ctx, ep)
	}
	if c.cb != nil {
		c.cb(c.Statuses())
	}
}

func (c *Checker) checkOne(ctx context.Context, ep TransportEndpoint) {
	c.mu.Lock()
	st := c.statuses[ep.Name]
	st.Status = health.StatusChecking
	c.statuses[ep.Name] = st
	c.mu.Unlock()

	// 1. TCP connectivity
	start := time.Now()
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(ep.Server, strconv.Itoa(ep.Port)), tcpTimeout)
	latency := int(time.Since(start).Milliseconds())
	if err != nil {
		c.recordResult(ep.Name, health.Result{Name: ep.Name, Reachable: false, RecentFailures: c.incFailures(ep.Name)}, fmt.Sprintf("tcp: %v", err))
		return
	}
	conn.Close()

	if ep.Network == "udp" && ep.ProxyPort < 0 {
		// Cannot perform HTTP proxy check for UDP-only transports without a local SOCKS sidecar.
		// Leave status as "checking" → will be treated as unknown by the GUI.
		c.mu.Lock()
		st := c.statuses[ep.Name]
		st.Status = health.StatusUnknown
		st.Message = "udp datapath check unavailable"
		c.statuses[ep.Name] = st
		c.mu.Unlock()
		return
	}

	// 2. Determine which proxy to use for HTTP check
	proxyToUse := ""
	if ep.ProxyPort > 0 {
		proxyToUse = "127.0.0.1:" + strconv.Itoa(ep.ProxyPort)
	} else if ep.ProxyPort == 0 && c.proxyAddr != "" {
		proxyToUse = c.proxyAddr
	}
	// ep.ProxyPort == -1 means no HTTP datapath check

	// 3. HTTP via proxy if configured
	if proxyToUse != "" {
		proxyURL := "http://" + proxyToUse
		client := &http.Client{
			Timeout: httpTimeout,
			Transport: &http.Transport{
				Proxy: http.ProxyURL(mustParseURL(proxyURL)),
			},
		}
		reqCtx, cancel := context.WithTimeout(ctx, httpTimeout)
		defer cancel()
		req, _ := http.NewRequestWithContext(reqCtx, "HEAD", "http://www.gstatic.com/generate_204", nil)
		resp, err := client.Do(req)
		if err != nil {
			c.recordResult(ep.Name, health.Result{Name: ep.Name, Reachable: false, RecentFailures: c.incFailures(ep.Name)}, fmt.Sprintf("http: %v", err))
			return
		}
		resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode > 299 {
			c.recordResult(ep.Name, health.Result{Name: ep.Name, Reachable: false, RecentFailures: c.incFailures(ep.Name)}, fmt.Sprintf("http: %d", resp.StatusCode))
			return
		}
	}

	c.recordResult(ep.Name, health.Result{Name: ep.Name, Reachable: true, LatencyMS: latency}, "")
}

func (c *Checker) recordResult(name string, r health.Result, msg string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.results[name] = r
	st := c.statuses[name]
	st.Name = name
	st.LatencyMS = r.LatencyMS
	st.Score = health.Score(r)
	if r.Reachable {
		st.Status = health.StatusReachable
		st.Message = ""
	} else {
		st.Status = health.StatusFailed
		st.Message = msg
	}
	c.statuses[name] = st
}

func (c *Checker) incFailures(name string) int {
	if prev, ok := c.results[name]; ok {
		return prev.RecentFailures + 1
	}
	return 1
}

func sliceResults(m map[string]health.Result) []health.Result {
	out := make([]health.Result, 0, len(m))
	for _, v := range m {
		out = append(out, v)
	}
	return out
}
