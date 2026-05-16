# Three Protocols Full Integration Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Make all 3 transport protocols (Hysteria2, Xray/XHTTP, AmneziaWG) fully functional end-to-end on both server and client, with proper health checking, auto-selection, and GUI status display.

**Architecture:** Xray becomes a managed sidecar process alongside sing-box, wired into sing-box via SOCKS outbound. AmneziaWG server config gets proper jitter params + peer management, client uses Jc=0 fallback for sing-box WireGuard compatibility. Hysteria2 gracefully handles ISP UDP blocking via health-based deprioritization.

**Tech Stack:** Go, sing-box, xray-core, amneziawg-tools, Windows systray GUI

---

## Current State Audit

| Protocol | Server | Client sing-box | Client sidecar | Auto-select | Data path |
|---|---|---|---|---|---|
| Hysteria2 | ✅ | ✅ native | N/A | ✅ urltest | ❌ ISP UDP block |
| Xray/XHTTP | ✅ | ❌ not in outbounds | ⚠️ manual prepare-xray | ❌ | ✅ TCP works |
| AmneziaWG | ⚠️ incomplete | ❌ incompatible | ❌ no client | ❌ skipped | ❌ no client |

## Root Causes

1. **`buildSingBoxOutbounds()`** (`internal/app/client.go:466-476`) — Only adds hysteria2. Xray and amneziawg excluded.
2. **Xray sidecar disconnected from flow** — `prepare-xray` generates config but nothing auto-starts xray during `connect`.
3. **AmneziaWG server config incomplete** — Missing jitter params (Jc/Jf/Jd), no [Peer] section, no NAT rules.
4. **Health checker can't test xray** — Goes through sing-box proxy which has no xray outbound.
5. **`chooseActiveTag()`** (`internal/app/client.go:400-408`) — Only checks hysteria2 and amneziawg, never xray.

---

## Phase 1: Xray Full Integration (TCP/443 — the working transport)

### Task 1.1: Add xray SOCKS outbound to sing-box config

**Files:**
- Modify: `internal/app/client.go:466-476` (buildSingBoxOutbounds)

**What:** Add a `socks` type outbound pointing to `127.0.0.1:2081` with tag `xray_reality_xhttp` so sing-box can route traffic through the xray sidecar.

```go
func buildSingBoxOutbounds(b bundle.Bundle) []map[string]any {
	items := make([]map[string]any, 0, 3)
	if v, ok := b.Transports["hysteria2"].(map[string]any); ok {
		items = append(items, map[string]any{"type": "hysteria2", "tag": "hysteria2", "server": v["server"], "server_port": v["port"], "password": v["password"], "tls": map[string]any{"enabled": true, "insecure": true}})
	}
	if _, ok := b.Transports["xray_reality_xhttp"].(map[string]any); ok {
		items = append(items, map[string]any{"type": "socks", "tag": "xray_reality_xhttp", "server": "127.0.0.1", "server_port": 2081})
	}
	if v, ok := b.Transports["amneziawg"].(map[string]any); ok {
		if status, _ := v["status"].(string); status != "experimental" {
			// Will be added in Phase 2
		}
	}
	return items
}
```

### Task 1.2: Auto-start xray sidecar during connect (CLI)

**Files:**
- Modify: `internal/app/client.go:129-186` (ConnectClient)

**What:** Before starting sing-box, auto-generate xray-client.json and start xray.exe as a background process. Store the xray cmd reference so we can kill it on disconnect.

Add a package-level var for xray process:
```go
var xrayCmd *exec.Cmd
```

In `ConnectClient()`, before starting sing-box:
```go
// Auto-start xray sidecar if xray transport is in profile
xrayCmd, err = startXraySidecar(base, link, stdout)
if err != nil {
    fmt.Fprintf(stdout, "warning: xray sidecar failed: %v\n", err)
}
```

New function:
```go
func startXraySidecar(base string, link string, stdout io.Writer) (*exec.Cmd, error) {
    b, err := bundle.Decode(link)
    if err != nil {
        return nil, err
    }
    if _, ok := b.Transports["xray_reality_xhttp"].(map[string]any); !ok {
        return nil, nil // no xray transport in profile
    }
    xrayBin := filepath.Join(base, "bin", "xray.exe")
    if _, err := os.Stat(xrayBin); err != nil {
        return nil, fmt.Errorf("xray.exe not found at %s", xrayBin)
    }
    // Generate config if not exists
    cfgPath := filepath.Join(base, "configs", "xray-client.json")
    if _, err := os.Stat(cfgPath); err != nil {
        // Generate config
        v, _ := b.Transports["xray_reality_xhttp"].(map[string]any)
        cfg, err := xrayclient.Config(xrayclient.ConfigInput{
            Server:     v["server"].(string),
            Port:       int(v["port"].(float64)),
            UUID:       v["uuid"].(string),
            PublicKey:  v["public_key"].(string),
            ShortID:    v["short_id"].(string),
            Path:       v["path"].(string),
            ServerName: v["server_name"].(string),
            LocalPort:  2081,
        })
        if err != nil {
            return nil, err
        }
        os.WriteFile(cfgPath, []byte(cfg), 0600)
    }
    cmd := exec.Command(xrayBin, "run", "-config", cfgPath)
    cmd.Stdout = stdout
    cmd.Stderr = stdout
    if err := cmd.Start(); err != nil {
        return nil, err
    }
    fmt.Fprintf(stdout, "xray sidecar started pid=%d\n", cmd.Process.Pid)
    return cmd, nil
}
```

### Task 1.3: Kill xray on disconnect

**Files:**
- Modify: `internal/app/client.go:221-231` (DisconnectClient)
- Modify: `internal/app/client.go:233-240` (killSingBox)

**What:** Also kill xray.exe process on disconnect.

```go
func DisconnectClient(stdout io.Writer) error {
    if err := killSingBox(); err != nil {
        fmt.Fprintf(stdout, "warning: %v\n", err)
    }
    if err := killXray(); err != nil {
        fmt.Fprintf(stdout, "warning: %v\n", err)
    }
    if xrayCmd != nil && xrayCmd.Process != nil {
        xrayCmd.Process.Kill()
        xrayCmd = nil
    }
    if err := sysproxy.Unset(); err != nil {
        fmt.Fprintf(stdout, "warning: could not unset system proxy: %v\n", err)
    }
    _, err := fmt.Fprintln(stdout, "disconnected")
    return err
}

func killXray() error {
    cmd := exec.Command("taskkill", "/F", "/IM", "xray.exe")
    out, err := cmd.CombinedOutput()
    if err != nil && !strings.Contains(string(out), "not found") {
        return fmt.Errorf("taskkill xray: %w (%s)", err, strings.TrimSpace(string(out)))
    }
    return nil
}
```

### Task 1.4: Auto-start xray in GUI mode

**Files:**
- Modify: `internal/gui/supervisor.go:31-81` (Start method)

**What:** Before starting sing-box, auto-start xray sidecar. Store reference for cleanup.

Add `xrayCmd *exec.Cmd` field to Supervisor struct. In Start(), before cmd.Start():
```go
// Start xray sidecar
s.startXraySidecar(base)
```

New method:
```go
func (s *Supervisor) startXraySidecar(base string) {
    link, _ := readProfileLink(base)
    if link == "" {
        return
    }
    b, err := bundle.Decode(link)
    if err != nil {
        return
    }
    if _, ok := b.Transports["xray_reality_xhttp"].(map[string]any); !ok {
        return
    }
    xrayBin := filepath.Join(base, "bin", "xray.exe")
    if _, err := os.Stat(xrayBin); err != nil {
        return
    }
    cfgPath := filepath.Join(base, "configs", "xray-client.json")
    if _, err := os.Stat(cfgPath); err != nil {
        return // config should have been generated during import
    }
    logPath := filepath.Join(base, "logs", "xray.log")
    logFile, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
    if err != nil {
        return
    }
    cmd := exec.Command(xrayBin, "run", "-config", cfgPath)
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
```

In Stop(), add xray cleanup:
```go
if s.xrayCmd != nil && s.xrayCmd.Process != nil {
    _ = s.xrayCmd.Process.Kill()
    s.xrayCmd = nil
}
```

Also kill xray.exe in the process exit monitor goroutine.

### Task 1.5: Auto-generate xray config during import

**Files:**
- Modify: `internal/app/client.go:82-113` (ImportProfileGUI)

**What:** After importing profile, also generate xray-client.json so it's ready for auto-start.

After `regenerateClientConfig(b, defaultClientMode)`:
```go
// Pre-generate xray client config
if err := generateXrayConfig(base, b); err != nil {
    fmt.Fprintf(stdout, "warning: xray config generation failed: %v\n", err)
}
```

New function:
```go
func generateXrayConfig(base string, b bundle.Bundle) error {
    v, ok := b.Transports["xray_reality_xhttp"].(map[string]any)
    if !ok {
        return nil
    }
    cfg, err := xrayclient.Config(xrayclient.ConfigInput{
        Server:     v["server"].(string),
        Port:       int(v["port"].(float64)),
        UUID:       v["uuid"].(string),
        PublicKey:  v["public_key"].(string),
        ShortID:    v["short_id"].(string),
        Path:       v["path"].(string),
        ServerName: v["server_name"].(string),
        LocalPort:  2081,
    })
    if err != nil {
        return err
    }
    return os.WriteFile(filepath.Join(base, "configs", "xray-client.json"), []byte(cfg), 0600)
}
```

### Task 1.6: Add xray to health checker

**Files:**
- Modify: `internal/app/client.go:188-219` (newHealthChecker)
- Modify: `internal/gui/supervisor.go:119-160` (startHealthChecker)
- Modify: `internal/healthcheck/healthcheck.go` (TransportEndpoint, checkOne)

**What:** Add xray health check via its own SOCKS proxy (127.0.0.1:2081) instead of through sing-box.

Add `ProxyPort` field to TransportEndpoint:
```go
type TransportEndpoint struct {
    Name      string
    Server    string
    Port      int
    ProxyPort int // 0 = TCP only, >0 = test via SOCKS proxy on this port
}
```

In `newHealthChecker()`, add xray endpoint with its own proxy port:
```go
if v, ok := b.Transports["xray_reality_xhttp"].(map[string]any); ok {
    server, _ := v["server"].(string)
    port, _ := v["port"].(float64)
    endpoints = append(endpoints, healthcheck.TransportEndpoint{
        Name: "xray_reality_xhttp", Server: server, Port: int(port), ProxyPort: 2081,
    })
}
```

In `checkOne()`, if `ep.ProxyPort > 0`, test via SOCKS proxy on that port instead of the shared proxy.

### Task 1.7: Fix auto-select to include xray

**Files:**
- Modify: `internal/app/client.go:400-424` (chooseActiveTag, chooseHealthyActiveTag)

**What:** Include xray_reality_xhttp in the preference list.

```go
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

func chooseHealthyActiveTag(outbounds []map[string]any) string {
    fallback := chooseActiveTag(outbounds)
    for _, preferred := range []string{"xray_reality_xhttp", "hysteria2", "amneziawg"} {
        for _, outbound := range outbounds {
            if outbound["tag"] != preferred {
                continue
            }
            if outboundReachable(outbound) {
                return preferred
            }
        }
    }
    return fallback
}
```

Note: xray_reality_xhttp is first in healthy preference because TCP/443 works reliably.

### Task 1.8: Update doctor to check xray sidecar

**Files:**
- Modify: `internal/app/client.go:242-303` (doctorClient)

**What:** Add check for xray.exe binary and xray-client.json config.

---

## Phase 2: AmneziaWG Full Integration

### Task 2.1: Add jitter params to AmneziaWG server config

**Files:**
- Modify: `internal/transports/amneziawg/amneziawg.go:20-22` (ServerFiles)
- Modify: `internal/config/config.go:48-54` (AmneziaWGSettings)

**What:** Add Jc, Jf, Jd, Jmin, Jmax params to config and server file generation.

Add to AmneziaWGSettings:
```go
type AmneziaWGSettings struct {
    Enabled    bool   `json:"enabled" yaml:"enabled"`
    Port       int    `json:"port" yaml:"port"`
    PrivateKey string `json:"private_key" yaml:"private_key"`
    PublicKey  string `json:"public_key" yaml:"public_key"`
    Address    string `json:"address" yaml:"address"`
    Jc         int    `json:"jc" yaml:"jc"`
    Jf         int    `json:"jf" yaml:"jf"`
    Jd         int    `json:"jd" yaml:"jd"`
    Jmin       int    `json:"jmin" yaml:"jmin"`
    Jmax       int    `json:"jmax" yaml:"jmax"`
}
```

Update DefaultServerConfig:
```go
AmneziaWG: AmneziaWGSettings{
    Enabled: true, Port: 51820, Address: "10.77.0.1/24",
    Jc: 4, Jf: 3, Jd: 40, Jmin: 50, Jmax: 1000,
},
```

Update ServerFiles to include jitter params:
```go
func (t Transport) ServerFiles(ctx transports.Context) ([]transports.GeneratedFile, error) {
    content := fmt.Sprintf(`[Interface]
Address = %s
ListenPort = %d
PrivateKey = %s
Jc = %d
Jf = %d
Jd = %d
Jmin = %d
Jmax = %d

PostUp = iptables -t nat -A POSTROUTING -s %s -o eth0 -j MASQUERADE
PostDown = iptables -t nat -D POSTROUTING -s %s -o eth0 -j MASQUERADE
`,
        t.Address, t.Port, t.PrivateKey,
        t.Jc, t.Jf, t.Jd, t.Jmin, t.Jmax,
        t.Address, t.Address)
    return []transports.GeneratedFile{{Path: ctx.BaseDir + "/configs/amneziawg.conf", Content: content, Mode: 0600}}, nil
}
```

### Task 2.2: Add peer management for AmneziaWG

**Files:**
- Modify: `internal/transports/amneziawg/amneziawg.go` (add PeerConfig, generate client keys)
- Modify: `internal/app/server.go:78-125` (setupServer — generate client keypair)
- Modify: `internal/config/config.go` (add ClientPublicKey to AmneziaWGSettings)

**What:** Generate client keypair during setup, add [Peer] section to server config with client public key.

Add to AmneziaWGSettings:
```go
ClientPublicKey  string `json:"client_public_key" yaml:"client_public_key"`
```

In setupServer, after generating server keys:
```go
// Generate AmneziaWG client keypair
awgClientPriv, awgClientPub := generateAmneziaWGKeypair(base)
cfg.Transports.AmneziaWG.ClientPublicKey = awgClientPub
```

New function to generate keypair using `awg genkey`/`awg pubkey`:
```go
func generateAmneziaWGKeypair(base string) (string, string) {
    awgPath := filepath.Join(base, "bin", binaryName("amneziawg"))
    // Try awg genkey
    privOut, err := exec.Command(awgPath, "genkey").CombinedOutput()
    if err != nil {
        return randomHex(32), randomHex(32)
    }
    privateKey := strings.TrimSpace(string(privOut))
    // Try awg pubkey
    pubCmd := exec.Command(awgPath, "pubkey")
    pubCmd.Stdin = strings.NewReader(privateKey)
    pubOut, err := pubCmd.CombinedOutput()
    if err != nil {
        return randomHex(32), randomHex(32)
    }
    publicKey := strings.TrimSpace(string(pubOut))
    return privateKey, publicKey
}
```

Update ServerFiles to include [Peer]:
```go
func (t Transport) ServerFiles(ctx transports.Context) ([]transports.GeneratedFile, error) {
    peerSection := ""
    if t.ClientPublicKey != "" {
        peerSection = fmt.Sprintf(`
[Peer]
PublicKey = %s
AllowedIPs = 10.77.0.2/32
`, t.ClientPublicKey)
    }
    content := fmt.Sprintf(`[Interface]
Address = %s
ListenPort = %d
PrivateKey = %s
Jc = %d
Jf = %d
Jd = %d
Jmin = %d
Jmax = %d

PostUp = iptables -t nat -A POSTROUTING -s %s -o eth0 -j MASQUERADE
PostDown = iptables -t nat -D POSTROUTING -s %s -o eth0 -j MASQUERADE
%s`, t.Address, t.Port, t.PrivateKey, t.Jc, t.Jf, t.Jd, t.Jmin, t.Jmax, t.Address, t.Address, peerSection)
    return []transports.GeneratedFile{{Path: ctx.BaseDir + "/configs/amneziawg.conf", Content: content, Mode: 0600}}, nil
}
```

### Task 2.3: Generate client-side AmneziaWG config

**Files:**
- Modify: `internal/transports/amneziawg/amneziawg.go` (ClientBundle)
- Modify: `internal/app/server.go:303-317` (buildDefaultBundle)

**What:** Export jitter params and client private key in bundle so client can generate WireGuard config.

Update ClientBundle:
```go
func (t Transport) ClientBundle(ctx transports.Context) (map[string]any, error) {
    return map[string]any{
        "server":      ctx.Domain,
        "port":        t.Port,
        "public_key":  t.PublicKey,
        "address":     t.Address,
        "jc":          t.Jc,
        "jf":          t.Jf,
        "jd":          t.Jd,
        "jmin":        t.Jmin,
        "jmax":        t.Jmax,
    }, nil
}
```

Update buildDefaultBundle to include jitter params:
```go
"amneziawg": map[string]any{
    "server": cfg.Server.Domain, "port": cfg.Transports.AmneziaWG.Port,
    "public_key": cfg.Transports.AmneziaWG.PublicKey, "address": cfg.Transports.AmneziaWG.Address,
    "jc": cfg.Transports.AmneziaWG.Jc, "jf": cfg.Transports.AmneziaWG.Jf,
    "jd": cfg.Transports.AmneziaWG.Jd, "jmin": cfg.Transports.AmneziaWG.Jmin,
    "jmax": cfg.Transports.AmneziaWG.Jmax,
},
```

### Task 2.4: Add AmneziaWG outbound to sing-box (Jc=0 fallback)

**Files:**
- Modify: `internal/app/client.go:466-476` (buildSingBoxOutbounds)

**What:** When Jc=0 (or when user opts for WireGuard compatibility), add a sing-box `wireguard` outbound. Otherwise, skip (requires separate amneziawg-go sidecar).

For MVP, use Jc=0 approach: if bundle has amneziawg with jitter params, generate a sing-box wireguard outbound with the server's public key. The server must also have Jc=0 for this to work.

**Alternative approach (simpler for MVP):** Keep amneziawg as "experimental" on client but fix the server config so it's ready for future client integration. Focus on getting xray working as the reliable fallback.

**Decision:** For MVP, keep amneziawg server-side complete (jitter params + peers) but mark client-side as requiring AmneziaVPN app. This is honest and avoids broken promises.

### Task 2.5: Remove "experimental" from amneziawg in health checker

**Files:**
- Modify: `internal/app/client.go:200-205` (newHealthChecker)
- Modify: `internal/gui/supervisor.go:140-145` (startHealthChecker)

**What:** Always check amneziawg health via TCP, regardless of status field. The "experimental" flag only affects whether we try to route traffic through it.

---

## Phase 3: Hysteria2 Graceful Handling

### Task 3.1: Detect UDP block in health checker

**Files:**
- Modify: `internal/healthcheck/healthcheck.go:144-186` (checkOne)

**What:** For hysteria2, if TCP check passes but HTTP proxy check fails (in normal mode), mark with a special message indicating possible UDP blocking. This helps the user understand why hysteria2 isn't being selected.

### Task 3.2: Ensure urltest naturally prefers xray over blocked hysteria2

**Files:**
- No code changes needed — sing-box urltest already tests each outbound and picks the one with lowest latency. If hysteria2 fails HTTP checks, it won't be selected.

**What:** Verify this works by testing: connect in normal mode, check that urltest selects xray when hysteria2 HTTP fails.

---

## Testing Plan

### Test 1: Xray end-to-end via sing-box
1. Import profile
2. Connect in normal mode
3. Verify xray sidecar is running (tasklist)
4. Verify sing-box has xray outbound (check sing-box.json)
5. `curl -x socks5://127.0.0.1:2080 ifconfig.me` — should return server IP
6. Check GUI shows "xray_reality_xhttp: reachable (XXms)"

### Test 2: Auto-select prefers xray
1. Connect in normal mode
2. Wait 1 minute for urltest to run
3. Check sing-box logs for urltest selection
4. Verify xray is selected (since hysteria2 UDP is blocked)

### Test 3: Cheap mode with xray
1. Set mode to cheap
2. Connect
3. Verify only xray outbound is active
4. Verify HTTP proxy works

### Test 4: AmneziaWG server-side complete
1. SSH to server
2. Check amneziawg.conf has jitter params and [Peer] section
3. Restart amneziawg service
4. Verify awg0 interface is up

### Test 5: Disconnect cleanup
1. Connect (starts sing-box + xray)
2. Disconnect
3. Verify both processes are killed
4. Verify system proxy is unset

---

## Implementation Order

1. Task 1.1: Add xray SOCKS outbound to sing-box config
2. Task 1.5: Auto-generate xray config during import
3. Task 1.2: Auto-start xray sidecar during connect (CLI)
4. Task 1.3: Kill xray on disconnect
5. Task 1.4: Auto-start xray in GUI mode
6. Task 1.6: Add xray to health checker
7. Task 1.7: Fix auto-select to include xray
8. Task 1.8: Update doctor
9. Task 2.1: Add jitter params to AmneziaWG server config
10. Task 2.2: Add peer management
11. Task 2.3: Generate client-side config
12. Task 2.5: Remove experimental from health checker
13. Test everything end-to-end
