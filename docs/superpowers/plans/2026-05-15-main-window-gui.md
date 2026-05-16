# Mirage Main Window GUI Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a Fyne-based Windows main window for Mirage client while keeping tray controls as backup quick access.

**Architecture:** Add a testable GUI controller and view state layer in `internal/gui`, then build Fyne window on top of that controller. Keep process supervision in `Supervisor`, app-level profile/config commands in `internal/app`, and use one shared supervisor/controller instance for tray and window state.

**Tech Stack:** Go 1.25, Fyne v2, `github.com/getlantern/systray`, `github.com/sqweek/dialog`, existing Mirage packages (`internal/app`, `internal/modes`, `internal/singbox`, `internal/sysproxy`).

---

## File Structure

- Create: `internal/gui/state.go`
  - Defines `ConnectionState`, `ProtocolSelection`, `ViewState`, display labels, and pure state transition helpers.
- Create: `internal/gui/controller.go`
  - Owns state, calls `Supervisor`, app commands, proxy functions, doctor runner, logs opener, and dialog callbacks.
- Create: `internal/gui/window.go`
  - Builds Fyne window, binds widgets to controller state, handles close-to-tray behavior.
- Modify: `internal/gui/tray.go`
  - Replace direct action logic with controller calls, add `Show window`, keep tray as fallback.
- Modify: `internal/gui/supervisor.go`
  - Expose `RunningState()` and `Stop()` behavior needed by controller, keep real port detection.
- Modify: `internal/app/client.go`
  - Add protocol persistence and forced protocol config generation.
- Modify: `internal/modes/mode.go`
  - Keep mode constants; do not add protocol constants here.
- Modify: `internal/singbox/config.go`
  - No interface change expected; use existing URLTest/single outbound builders.
- Modify: `cmd/mirage-client/main.go`
  - Launch combined main window + tray GUI for no-argument mode.
- Modify: `go.mod`, `go.sum`
  - Add Fyne dependency.
- Create: `internal/gui/state_test.go`
  - Unit tests for view-state transitions and restart-required behavior.
- Create: `internal/gui/controller_test.go`
  - Unit tests for controller state methods using dependency injection.
- Modify: `internal/gui/tray_test.go`
  - Keep message tests; add tray-state label helper tests if helper extracted.
- Modify: `internal/app/client_test.go`
  - Add protocol persistence and forced config tests.

---

### Task 1: Add protocol and view state model

**Files:**
- Create: `internal/gui/state.go`
- Create: `internal/gui/state_test.go`

- [ ] **Step 1: Write failing state tests**

Create `internal/gui/state_test.go`:

```go
package gui

import "testing"

func TestInitialViewStateIsDisconnectedAutoNormal(t *testing.T) {
	state := InitialViewState()
	if state.Connection != ConnectionDisconnected {
		t.Fatalf("Connection = %q, want %q", state.Connection, ConnectionDisconnected)
	}
	if state.ModeLabel != "Normal proxy" {
		t.Fatalf("ModeLabel = %q, want Normal proxy", state.ModeLabel)
	}
	if state.Protocol != ProtocolAuto {
		t.Fatalf("Protocol = %q, want %q", state.Protocol, ProtocolAuto)
	}
	if state.PrimaryAction != "Connect" {
		t.Fatalf("PrimaryAction = %q, want Connect", state.PrimaryAction)
	}
}

func TestConnectedModeChangeRequiresRestart(t *testing.T) {
	state := InitialViewState()
	state = state.WithConnection(ConnectionConnected)
	state = state.WithMode("full")
	if !state.RestartRequired {
		t.Fatal("RestartRequired = false, want true")
	}
	if state.PrimaryAction != "Restart to apply" {
		t.Fatalf("PrimaryAction = %q, want Restart to apply", state.PrimaryAction)
	}
	if state.ModeLabel != "Full TUN" {
		t.Fatalf("ModeLabel = %q, want Full TUN", state.ModeLabel)
	}
}

func TestConnectedProtocolChangeRequiresRestart(t *testing.T) {
	state := InitialViewState()
	state = state.WithConnection(ConnectionConnected)
	state = state.WithProtocol(ProtocolXray)
	if !state.RestartRequired {
		t.Fatal("RestartRequired = false, want true")
	}
	if state.PrimaryAction != "Restart to apply" {
		t.Fatalf("PrimaryAction = %q, want Restart to apply", state.PrimaryAction)
	}
	if state.ProtocolLabel != "Xray" {
		t.Fatalf("ProtocolLabel = %q, want Xray", state.ProtocolLabel)
	}
}

func TestConnectingAndErrorLabels(t *testing.T) {
	state := InitialViewState().WithConnection(ConnectionConnecting)
	if state.StatusLabel != "Connecting" {
		t.Fatalf("StatusLabel = %q, want Connecting", state.StatusLabel)
	}
	state = state.WithError("Import profile first")
	if state.Connection != ConnectionError {
		t.Fatalf("Connection = %q, want %q", state.Connection, ConnectionError)
	}
	if state.ErrorText != "Import profile first" {
		t.Fatalf("ErrorText = %q, want Import profile first", state.ErrorText)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run:

```powershell
go -C mirage test ./internal/gui -run "TestInitialViewState|TestConnectedModeChange|TestConnectedProtocolChange|TestConnectingAndError" -count=1
```

Expected: FAIL with compile errors like `undefined: InitialViewState` and `undefined: ConnectionDisconnected`.

- [ ] **Step 3: Implement state model**

Create `internal/gui/state.go`:

```go
package gui

import "mirage/internal/modes"

type ConnectionState string

const (
	ConnectionDisconnected ConnectionState = "disconnected"
	ConnectionConnecting    ConnectionState = "connecting"
	ConnectionConnected     ConnectionState = "connected"
	ConnectionError         ConnectionState = "error"
)

type ProtocolSelection string

const (
	ProtocolAuto      ProtocolSelection = "auto"
	ProtocolHysteria2 ProtocolSelection = "hysteria2"
	ProtocolXray      ProtocolSelection = "xray_reality_xhttp"
	ProtocolAmneziaWG ProtocolSelection = "amneziawg"
)

type ViewState struct {
	Connection      ConnectionState
	Mode            modes.Mode
	Protocol        ProtocolSelection
	StatusLabel     string
	ModeLabel       string
	ProtocolLabel   string
	PrimaryAction   string
	ActiveTransport string
	RestartRequired bool
	ErrorText       string
}

func InitialViewState() ViewState {
	state := ViewState{
		Connection: ConnectionDisconnected,
		Mode:       modes.Normal,
		Protocol:   ProtocolAuto,
	}
	state.refreshLabels()
	return state
}

func (s ViewState) WithConnection(connection ConnectionState) ViewState {
	s.Connection = connection
	if connection != ConnectionError {
		s.ErrorText = ""
	}
	if connection == ConnectionConnected {
		s.RestartRequired = false
	}
	s.refreshLabels()
	return s
}

func (s ViewState) WithMode(mode string) ViewState {
	next := modes.Normalize(mode)
	if s.Connection == ConnectionConnected && next != s.Mode {
		s.RestartRequired = true
	}
	s.Mode = next
	s.refreshLabels()
	return s
}

func (s ViewState) WithProtocol(protocol ProtocolSelection) ViewState {
	if !validProtocol(protocol) {
		protocol = ProtocolAuto
	}
	if s.Connection == ConnectionConnected && protocol != s.Protocol {
		s.RestartRequired = true
	}
	s.Protocol = protocol
	s.refreshLabels()
	return s
}

func (s ViewState) WithActiveTransport(name string) ViewState {
	s.ActiveTransport = name
	s.refreshLabels()
	return s
}

func (s ViewState) WithError(message string) ViewState {
	s.Connection = ConnectionError
	s.ErrorText = message
	s.refreshLabels()
	return s
}

func validProtocol(protocol ProtocolSelection) bool {
	switch protocol {
	case ProtocolAuto, ProtocolHysteria2, ProtocolXray, ProtocolAmneziaWG:
		return true
	default:
		return false
	}
}

func (s *ViewState) refreshLabels() {
	s.StatusLabel = connectionLabel(s.Connection)
	s.ModeLabel = modeLabel(s.Mode)
	s.ProtocolLabel = protocolLabel(s.Protocol)
	if s.RestartRequired {
		s.PrimaryAction = "Restart to apply"
		return
	}
	s.PrimaryAction = primaryActionLabel(s.Connection)
}

func connectionLabel(connection ConnectionState) string {
	switch connection {
	case ConnectionConnecting:
		return "Connecting"
	case ConnectionConnected:
		return "Connected"
	case ConnectionError:
		return "Error"
	default:
		return "Disconnected"
	}
}

func modeLabel(mode modes.Mode) string {
	if modes.IsTun(mode) {
		return "Full TUN"
	}
	return "Normal proxy"
}

func protocolLabel(protocol ProtocolSelection) string {
	switch protocol {
	case ProtocolHysteria2:
		return "Hysteria2"
	case ProtocolXray:
		return "Xray"
	case ProtocolAmneziaWG:
		return "AmneziaWG"
	default:
		return "Auto"
	}
}

func primaryActionLabel(connection ConnectionState) string {
	switch connection {
	case ConnectionConnected:
		return "Disconnect"
	case ConnectionConnecting:
		return "Connecting"
	default:
		return "Connect"
	}
}
```

- [ ] **Step 4: Run state tests**

Run:

```powershell
go -C mirage test ./internal/gui -run "TestInitialViewState|TestConnectedModeChange|TestConnectedProtocolChange|TestConnectingAndError" -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit**

```powershell
git add mirage/internal/gui/state.go mirage/internal/gui/state_test.go
git commit -m "feat: add gui view state model"
```

---

### Task 2: Add protocol persistence and forced config generation

**Files:**
- Modify: `internal/app/client.go`
- Modify: `internal/app/client_test.go`

- [ ] **Step 1: Write failing protocol persistence tests**

Append to `internal/app/client_test.go`:

```go
func TestClientProtocolDefaultsToAuto(t *testing.T) {
	base := t.TempDir()
	if got := ReadClientProtocol(base); got != "auto" {
		t.Fatalf("ReadClientProtocol = %q, want auto", got)
	}
}

func TestSetClientProtocolPersistsSelection(t *testing.T) {
	base := t.TempDir()
	if err := SetClientProtocol(base, "xray_reality_xhttp"); err != nil {
		t.Fatal(err)
	}
	if got := ReadClientProtocol(base); got != "xray_reality_xhttp" {
		t.Fatalf("ReadClientProtocol = %q, want xray_reality_xhttp", got)
	}
}

func TestNormalizeClientProtocolRejectsUnknown(t *testing.T) {
	if got := NormalizeClientProtocol("bogus"); got != "auto" {
		t.Fatalf("NormalizeClientProtocol = %q, want auto", got)
	}
}
```

- [ ] **Step 2: Run tests to verify failure**

Run:

```powershell
go -C mirage test ./internal/app -run "TestClientProtocol|TestSetClientProtocol|TestNormalizeClientProtocol" -count=1
```

Expected: FAIL with `undefined: ReadClientProtocol`.

- [ ] **Step 3: Implement protocol persistence helpers**

In `internal/app/client.go`, add after `const defaultClientMode = modes.Normal`:

```go
const defaultClientProtocol = "auto"

const (
	ClientProtocolAuto      = "auto"
	ClientProtocolHysteria2 = "hysteria2"
	ClientProtocolXray      = "xray_reality_xhttp"
	ClientProtocolAmneziaWG = "amneziawg"
)
```

Add after `ReadClientMode`:

```go
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
```

- [ ] **Step 4: Persist default protocol on import**

In `ImportProfileGUI`, after writing `mode.txt`, add:

```go
	if err := os.WriteFile(filepath.Join(base, "state", "transport.txt"), []byte(defaultClientProtocol+"\n"), 0600); err != nil {
		return err
	}
```

- [ ] **Step 5: Run persistence tests**

Run:

```powershell
go -C mirage test ./internal/app -run "TestClientProtocol|TestSetClientProtocol|TestNormalizeClientProtocol" -count=1
```

Expected: PASS.

- [ ] **Step 6: Write failing forced config tests**

Append to `internal/app/client_test.go`:

```go
func TestKeepOnlyRouteTagKeepsSelectedOutbound(t *testing.T) {
	outbounds := []map[string]any{
		{"type": "hysteria2", "tag": "hysteria2"},
		{"type": "socks", "tag": "xray_reality_xhttp"},
	}
	endpoints := []map[string]any{{"type": "wireguard", "tag": "amneziawg"}}
	gotOutbounds, gotEndpoints := keepOnlyRouteTag(outbounds, endpoints, "xray_reality_xhttp")
	if len(gotOutbounds) != 1 || gotOutbounds[0]["tag"] != "xray_reality_xhttp" {
		t.Fatalf("outbounds = %#v, want only xray_reality_xhttp", gotOutbounds)
	}
	if len(gotEndpoints) != 0 {
		t.Fatalf("endpoints = %#v, want empty", gotEndpoints)
	}
}

func TestKeepOnlyRouteTagKeepsSelectedEndpoint(t *testing.T) {
	outbounds := []map[string]any{
		{"type": "hysteria2", "tag": "hysteria2"},
		{"type": "socks", "tag": "xray_reality_xhttp"},
	}
	endpoints := []map[string]any{{"type": "wireguard", "tag": "amneziawg"}}
	gotOutbounds, gotEndpoints := keepOnlyRouteTag(outbounds, endpoints, "amneziawg")
	if len(gotOutbounds) != 0 {
		t.Fatalf("outbounds = %#v, want empty", gotOutbounds)
	}
	if len(gotEndpoints) != 1 || gotEndpoints[0]["tag"] != "amneziawg" {
		t.Fatalf("endpoints = %#v, want only amneziawg", gotEndpoints)
	}
}
```

- [ ] **Step 7: Run forced config tests to verify failure**

Run:

```powershell
go -C mirage test ./internal/app -run "TestKeepOnlyRouteTag" -count=1
```

Expected: FAIL with `undefined: keepOnlyRouteTag`.

- [ ] **Step 8: Add forced route helper**

In `internal/app/client.go`, after `keepOnlyTag`, add:

```go
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
```

- [ ] **Step 9: Apply protocol in config generation**

Replace `regenerateClientConfig` with:

```go
func regenerateClientConfig(b bundle.Bundle, mode modes.Mode) error {
	base := platform.ClientBaseDir()
	outbounds, endpoints := buildSingBoxOutbounds(b)
	protocol := ReadClientProtocol(base)
	outbounds, endpoints = keepOnlyRouteTag(outbounds, endpoints, protocol)
	active := chooseActiveTag(outbounds)
	if len(endpoints) > 0 && active == "direct" {
		if tag, ok := endpoints[0]["tag"].(string); ok {
			active = tag
		}
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
			cfg, err = templates.SingBoxTunConfig(active, outbounds)
		} else {
			cfg, err = templates.SingBoxConfig(active, outbounds)
		}
	case modes.Normal:
		if useURLTest {
			cfg, err = templates.SingBoxConfigURLTest(outbounds, endpoints)
		} else {
			cfg, err = templates.SingBoxConfig(active, outbounds)
		}
	case modes.Full:
		if useURLTest {
			cfg, err = templates.SingBoxTunConfigURLTest(outbounds, endpoints)
		} else {
			cfg, err = templates.SingBoxTunConfig(active, outbounds)
		}
	default:
		if modes.IsTun(mode) {
			cfg, err = templates.SingBoxTunConfig(active, outbounds)
		} else {
			cfg, err = templates.SingBoxConfig(active, outbounds)
		}
	}
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(base, "configs", "sing-box.json"), []byte(cfg), 0600)
}
```

- [ ] **Step 10: Run app tests**

Run:

```powershell
go -C mirage test ./internal/app -count=1
```

Expected: PASS.

- [ ] **Step 11: Commit**

```powershell
git add mirage/internal/app/client.go mirage/internal/app/client_test.go
git commit -m "feat: persist gui protocol selection"
```

---

### Task 3: Add controller with injected dependencies

**Files:**
- Create: `internal/gui/controller.go`
- Create: `internal/gui/controller_test.go`

- [ ] **Step 1: Write failing controller tests**

Create `internal/gui/controller_test.go`:

```go
package gui

import (
	"errors"
	"testing"

	"mirage/internal/modes"
)

type fakeControllerDeps struct {
	running       bool
	connectErr    error
	disconnectErr error
	modeSet       modes.Mode
	protocolSet   ProtocolSelection
}

func (f *fakeControllerDeps) IsRunning() bool { return f.running }
func (f *fakeControllerDeps) Connect() error {
	if f.connectErr != nil {
		return f.connectErr
	}
	f.running = true
	return nil
}
func (f *fakeControllerDeps) Disconnect() error {
	if f.disconnectErr != nil {
		return f.disconnectErr
	}
	f.running = false
	return nil
}
func (f *fakeControllerDeps) SetMode(mode modes.Mode) error {
	f.modeSet = mode
	return nil
}
func (f *fakeControllerDeps) SetProtocol(protocol ProtocolSelection) error {
	f.protocolSet = protocol
	return nil
}
func (f *fakeControllerDeps) ImportProfile(path string) (string, error) { return "profile imported: default", nil }
func (f *fakeControllerDeps) RunDoctor() (string, error) { return "doctor ok", nil }
func (f *fakeControllerDeps) OpenLogsFolder() error { return nil }
func (f *fakeControllerDeps) SetSystemProxy() error { return nil }
func (f *fakeControllerDeps) ClearSystemProxy() error { return nil }
func (f *fakeControllerDeps) BestTransportLabel() string { return "xray_reality_xhttp (42ms)" }

func TestControllerConnectUpdatesState(t *testing.T) {
	deps := &fakeControllerDeps{}
	controller := NewControllerWithDeps(deps)
	if err := controller.Connect(); err != nil {
		t.Fatal(err)
	}
	state := controller.State()
	if state.Connection != ConnectionConnected {
		t.Fatalf("Connection = %q, want %q", state.Connection, ConnectionConnected)
	}
	if state.ActiveTransport != "xray_reality_xhttp (42ms)" {
		t.Fatalf("ActiveTransport = %q, want xray label", state.ActiveTransport)
	}
}

func TestControllerConnectFailureShowsError(t *testing.T) {
	deps := &fakeControllerDeps{connectErr: errors.New("sing-box missing at C:\\Mirage\\bin\\sing-box.exe")}
	controller := NewControllerWithDeps(deps)
	if err := controller.Connect(); err == nil {
		t.Fatal("Connect error = nil, want error")
	}
	state := controller.State()
	if state.Connection != ConnectionError {
		t.Fatalf("Connection = %q, want %q", state.Connection, ConnectionError)
	}
	if state.ErrorText != "sing-box missing at C:\\Mirage\\bin\\sing-box.exe" {
		t.Fatalf("ErrorText = %q", state.ErrorText)
	}
}

func TestControllerModeChangeWhileConnectedRequiresRestart(t *testing.T) {
	deps := &fakeControllerDeps{running: true}
	controller := NewControllerWithDeps(deps)
	controller.RefreshRuntimeState()
	if err := controller.SetMode(modes.Full); err != nil {
		t.Fatal(err)
	}
	state := controller.State()
	if !state.RestartRequired {
		t.Fatal("RestartRequired = false, want true")
	}
	if deps.modeSet != modes.Full {
		t.Fatalf("modeSet = %q, want full", deps.modeSet)
	}
}

func TestControllerRestartToApplyDisconnectsThenConnects(t *testing.T) {
	deps := &fakeControllerDeps{running: true}
	controller := NewControllerWithDeps(deps)
	controller.RefreshRuntimeState()
	if err := controller.SetProtocol(ProtocolXray); err != nil {
		t.Fatal(err)
	}
	if err := controller.PrimaryAction(); err != nil {
		t.Fatal(err)
	}
	state := controller.State()
	if state.RestartRequired {
		t.Fatal("RestartRequired = true, want false")
	}
	if state.Connection != ConnectionConnected {
		t.Fatalf("Connection = %q, want connected", state.Connection)
	}
}
```

- [ ] **Step 2: Run tests to verify failure**

Run:

```powershell
go -C mirage test ./internal/gui -run "TestController" -count=1
```

Expected: FAIL with `undefined: NewControllerWithDeps`.

- [ ] **Step 3: Implement controller core**

Create `internal/gui/controller.go`:

```go
package gui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"mirage/internal/app"
	"mirage/internal/bundle"
	"mirage/internal/modes"
	"mirage/internal/platform"
	"mirage/internal/sysproxy"
)

type ControllerDeps interface {
	IsRunning() bool
	Connect() error
	Disconnect() error
	SetMode(mode modes.Mode) error
	SetProtocol(protocol ProtocolSelection) error
	ImportProfile(path string) (string, error)
	RunDoctor() (string, error)
	OpenLogsFolder() error
	SetSystemProxy() error
	ClearSystemProxy() error
	BestTransportLabel() string
}

type Controller struct {
	mu        sync.RWMutex
	state     ViewState
	deps      ControllerDeps
	onChange  func(ViewState)
	onError   func(string)
	onMessage func(string)
}

func NewController(supervisor *Supervisor) *Controller {
	return NewControllerWithDeps(newRuntimeControllerDeps(supervisor))
}

func NewControllerWithDeps(deps ControllerDeps) *Controller {
	controller := &Controller{state: InitialViewState(), deps: deps}
	controller.RefreshRuntimeState()
	return controller
}

func (c *Controller) SetOnChange(fn func(ViewState)) {
	c.mu.Lock()
	c.onChange = fn
	c.mu.Unlock()
}

func (c *Controller) SetOnError(fn func(string)) {
	c.mu.Lock()
	c.onError = fn
	c.mu.Unlock()
}

func (c *Controller) SetOnMessage(fn func(string)) {
	c.mu.Lock()
	c.onMessage = fn
	c.mu.Unlock()
}

func (c *Controller) State() ViewState {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.state
}

func (c *Controller) RefreshRuntimeState() {
	c.mu.Lock()
	if c.deps.IsRunning() {
		c.state = c.state.WithConnection(ConnectionConnected).WithActiveTransport(c.deps.BestTransportLabel())
	} else {
		c.state = c.state.WithConnection(ConnectionDisconnected)
	}
	state := c.state
	onChange := c.onChange
	c.mu.Unlock()
	emitChange(onChange, state)
}

func (c *Controller) PrimaryAction() error {
	state := c.State()
	if state.RestartRequired {
		return c.Restart()
	}
	if state.Connection == ConnectionConnected {
		return c.Disconnect()
	}
	return c.Connect()
}

func (c *Controller) Connect() error {
	c.setState(c.State().WithConnection(ConnectionConnecting))
	if err := c.deps.Connect(); err != nil {
		c.fail(err.Error())
		return err
	}
	c.setState(c.State().WithConnection(ConnectionConnected).WithActiveTransport(c.deps.BestTransportLabel()))
	return nil
}

func (c *Controller) Disconnect() error {
	if err := c.deps.Disconnect(); err != nil {
		c.fail(err.Error())
		return err
	}
	c.setState(c.State().WithConnection(ConnectionDisconnected))
	return nil
}

func (c *Controller) Restart() error {
	if err := c.deps.Disconnect(); err != nil {
		c.fail(err.Error())
		return err
	}
	c.mu.Lock()
	c.state.RestartRequired = false
	c.state.refreshLabels()
	c.mu.Unlock()
	return c.Connect()
}

func (c *Controller) SetMode(mode modes.Mode) error {
	if err := c.deps.SetMode(mode); err != nil {
		c.fail(err.Error())
		return err
	}
	c.setState(c.State().WithMode(string(mode)))
	return nil
}

func (c *Controller) SetProtocol(protocol ProtocolSelection) error {
	if err := c.deps.SetProtocol(protocol); err != nil {
		c.fail(err.Error())
		return err
	}
	c.setState(c.State().WithProtocol(protocol))
	return nil
}

func (c *Controller) ImportProfile(path string) error {
	message, err := c.deps.ImportProfile(path)
	if err != nil {
		c.fail(err.Error())
		return err
	}
	c.message(message)
	c.RefreshRuntimeState()
	return nil
}

func (c *Controller) RunDoctor() (string, error) {
	out, err := c.deps.RunDoctor()
	if err != nil {
		c.fail(err.Error())
		return out, err
	}
	return out, nil
}

func (c *Controller) OpenLogsFolder() error {
	if err := c.deps.OpenLogsFolder(); err != nil {
		c.fail(err.Error())
		return err
	}
	return nil
}

func (c *Controller) SetSystemProxy() error {
	if err := c.deps.SetSystemProxy(); err != nil {
		c.fail(err.Error())
		return err
	}
	c.message("System proxy set to 127.0.0.1:2080")
	return nil
}

func (c *Controller) ClearSystemProxy() error {
	if err := c.deps.ClearSystemProxy(); err != nil {
		c.fail(err.Error())
		return err
	}
	c.message("System proxy cleared")
	return nil
}

func (c *Controller) setState(state ViewState) {
	c.mu.Lock()
	c.state = state
	onChange := c.onChange
	c.mu.Unlock()
	emitChange(onChange, state)
}

func (c *Controller) fail(message string) {
	c.mu.Lock()
	c.state = c.state.WithError(message)
	state := c.state
	onChange := c.onChange
	onError := c.onError
	c.mu.Unlock()
	emitChange(onChange, state)
	if onError != nil {
		onError(message)
	}
}

func (c *Controller) message(message string) {
	c.mu.RLock()
	onMessage := c.onMessage
	c.mu.RUnlock()
	if onMessage != nil {
		onMessage(message)
	}
}

func emitChange(fn func(ViewState), state ViewState) {
	if fn != nil {
		fn(state)
	}
}

type runtimeControllerDeps struct {
	supervisor *Supervisor
}

func newRuntimeControllerDeps(supervisor *Supervisor) *runtimeControllerDeps {
	return &runtimeControllerDeps{supervisor: supervisor}
}

func (d *runtimeControllerDeps) IsRunning() bool {
	return d.supervisor.IsRunning()
}

func (d *runtimeControllerDeps) Connect() error {
	base := platform.ClientBaseDir()
	cfg := filepath.Join(base, "configs", "sing-box.json")
	if _, err := readRequiredFile(cfg, "Import profile first"); err != nil {
		return err
	}
	mode := readClientMode(base)
	if modes.IsTun(mode) && !sysproxy.IsAdmin() {
		return fmt.Errorf("TUN mode requires Administrator rights. Please restart Mirage as Administrator.")
	}
	singBox := filepath.Join(base, "bin", "sing-box.exe")
	if _, err := readRequiredFile(singBox, fmt.Sprintf("sing-box.exe missing at %s", singBox)); err != nil {
		return err
	}
	if err := d.supervisor.Start(singBox, cfg, mode); err != nil {
		return err
	}
	if !modes.IsTun(mode) {
		if err := sysproxy.Set("127.0.0.1:2080"); err != nil {
			return fmt.Errorf("connected but system proxy set failed: %w", err)
		}
	}
	return nil
}

func (d *runtimeControllerDeps) Disconnect() error {
	d.supervisor.Stop()
	var out strings.Builder
	return app.DisconnectClient(&out)
}

func (d *runtimeControllerDeps) SetMode(mode modes.Mode) error {
	base := platform.ClientBaseDir()
	link, err := readProfileLink(base)
	if err != nil {
		return fmt.Errorf("No imported profile")
	}
	b, err := bundle.Decode(link)
	if err != nil {
		return fmt.Errorf("Invalid profile")
	}
	return app.SetClientModeGUI(base, string(mode), b)
}

func (d *runtimeControllerDeps) SetProtocol(protocol ProtocolSelection) error {
	base := platform.ClientBaseDir()
	link, err := readProfileLink(base)
	if err != nil {
		return fmt.Errorf("No imported profile")
	}
	b, err := bundle.Decode(link)
	if err != nil {
		return fmt.Errorf("Invalid profile")
	}
	if err := app.SetClientProtocol(base, string(protocol)); err != nil {
		return err
	}
	return app.SetClientModeGUI(base, string(readClientMode(base)), b)
}

func (d *runtimeControllerDeps) ImportProfile(path string) (string, error) {
	var out strings.Builder
	if err := app.ImportProfileGUI(path, &out); err != nil {
		return "", err
	}
	return strings.TrimSpace(out.String()), nil
}

func (d *runtimeControllerDeps) RunDoctor() (string, error) {
	var out strings.Builder
	if err := app.RunClient([]string{"doctor"}, &out, &out); err != nil {
		return out.String(), err
	}
	return out.String(), nil
}

func (d *runtimeControllerDeps) OpenLogsFolder() error {
	path := filepath.Join(platform.ClientBaseDir(), "logs")
	return exec.Command("explorer.exe", path).Start()
}

func (d *runtimeControllerDeps) SetSystemProxy() error {
	return sysproxy.Set("127.0.0.1:2080")
}

func (d *runtimeControllerDeps) ClearSystemProxy() error {
	return sysproxy.Unset()
}

func (d *runtimeControllerDeps) BestTransportLabel() string {
	if best, ok := d.supervisor.BestTransport(); ok {
		return fmt.Sprintf("%s (%dms)", best.Name, best.LatencyMS)
	}
	return "unknown"
}

func readRequiredFile(path string, message string) (string, error) {
	if _, err := os.Stat(path); err != nil {
		return "", fmt.Errorf("%s", message)
	}
	return path, nil
}
```

The import block already includes `os` for `os.Stat`.

- [ ] **Step 4: Run controller tests**

Run:

```powershell
go -C mirage test ./internal/gui -run "TestController" -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit**

```powershell
git add mirage/internal/gui/controller.go mirage/internal/gui/controller_test.go
git commit -m "feat: add gui controller"
```

---

### Task 4: Add Fyne dependency and main window

**Files:**
- Modify: `go.mod`, `go.sum`
- Create: `internal/gui/window.go`
- Modify: `internal/gui/tray.go`
- Modify: `cmd/mirage-client/main.go`

- [ ] **Step 1: Add Fyne dependency**

Run:

```powershell
go -C mirage get fyne.io/fyne/v2@latest
```

Expected: `go.mod` contains `fyne.io/fyne/v2` and `go.sum` updates.

- [ ] **Step 2: Write Fyne window implementation**

Create `internal/gui/window.go`:

```go
package gui

import (
	"fmt"
	"time"

	"mirage/internal/modes"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

type DesktopGUI struct {
	fyneApp    fyne.App
	window     fyne.Window
	controller *Controller

	statusLabel    *widget.Label
	modeLabel      *widget.Label
	protocolLabel  *widget.Label
	transportLabel *widget.Label
	errorLabel     *widget.Label
	actionButton   *widget.Button
	modeRadio      *widget.RadioGroup
	protocolRadio  *widget.RadioGroup
	doctorOutput   *widget.Entry
}

func NewDesktopGUI(controller *Controller) *DesktopGUI {
	fyneApp := app.NewWithID("mirage.client")
	w := fyneApp.NewWindow("Mirage")
	gui := &DesktopGUI{fyneApp: fyneApp, window: w, controller: controller}
	gui.build()
	controller.SetOnChange(gui.applyState)
	controller.SetOnError(func(message string) {
		dialog.ShowError(fmt.Errorf("%s", message), w)
	})
	controller.SetOnMessage(func(message string) {
		dialog.ShowInformation("Mirage", message, w)
	})
	gui.applyState(controller.State())
	return gui
}

func (g *DesktopGUI) ShowAndRun() {
	g.window.Resize(fyne.NewSize(720, 620))
	g.window.Show()
	go g.refreshLoop()
	g.fyneApp.Run()
}

func (g *DesktopGUI) Show() {
	g.window.Show()
	g.window.RequestFocus()
}

func (g *DesktopGUI) build() {
	g.statusLabel = widget.NewLabel("Disconnected")
	g.statusLabel.TextStyle = fyne.TextStyle{Bold: true}
	g.modeLabel = widget.NewLabel("Normal proxy")
	g.protocolLabel = widget.NewLabel("Auto")
	g.transportLabel = widget.NewLabel("unknown")
	g.errorLabel = widget.NewLabel("")
	g.errorLabel.Wrapping = fyne.TextWrapWord

	g.actionButton = widget.NewButton("Connect", func() {
		go func() { _ = g.controller.PrimaryAction() }()
	})

	g.modeRadio = widget.NewRadioGroup([]string{"Normal proxy", "Full TUN"}, func(value string) {
		if value == "Full TUN" {
			go func() { _ = g.controller.SetMode(modes.Full) }()
			return
		}
		go func() { _ = g.controller.SetMode(modes.Normal) }()
	})

	g.protocolRadio = widget.NewRadioGroup([]string{"Auto", "Hysteria2", "Xray", "AmneziaWG"}, func(value string) {
		protocol := ProtocolAuto
		switch value {
		case "Hysteria2":
			protocol = ProtocolHysteria2
		case "Xray":
			protocol = ProtocolXray
		case "AmneziaWG":
			protocol = ProtocolAmneziaWG
		}
		go func() { _ = g.controller.SetProtocol(protocol) }()
	})

	setProxy := widget.NewButton("Set system proxy", func() {
		go func() { _ = g.controller.SetSystemProxy() }()
	})
	clearProxy := widget.NewButton("Clear system proxy", func() {
		go func() { _ = g.controller.ClearSystemProxy() }()
	})
	importProfile := widget.NewButton("Import profile", func() {
		dialog.ShowFileOpen(func(reader fyne.URIReadCloser, err error) {
			if err != nil || reader == nil {
				return
			}
			path := reader.URI().Path()
			_ = reader.Close()
			go func() { _ = g.controller.ImportProfile(path) }()
		}, g.window)
	})
	openLogs := widget.NewButton("Open logs folder", func() {
		go func() { _ = g.controller.OpenLogsFolder() }()
	})

	g.doctorOutput = widget.NewMultiLineEntry()
	g.doctorOutput.SetPlaceHolder("Doctor output appears here")
	runDoctor := widget.NewButton("Run doctor", func() {
		go func() {
			out, err := g.controller.RunDoctor()
			if err != nil {
				out = out + "\n" + err.Error()
			}
			g.doctorOutput.SetText(out)
		}()
	})

	statusCard := widget.NewCard("Connection status", "", container.NewVBox(
		g.statusLabel,
		widget.NewForm(
			widget.NewFormItem("Mode", g.modeLabel),
			widget.NewFormItem("Protocol", g.protocolLabel),
			widget.NewFormItem("Best/active transport", g.transportLabel),
		),
		g.errorLabel,
	))
	modeCard := widget.NewCard("Mode", "", g.modeRadio)
	protocolCard := widget.NewCard("Protocol", "", g.protocolRadio)
	proxyCard := widget.NewCard("Proxy controls", "Normal proxy uses 127.0.0.1:2080", container.NewHBox(setProxy, clearProxy))
	utilityCard := widget.NewCard("Utility actions", "", container.NewVBox(container.NewHBox(importProfile, runDoctor, openLogs), g.doctorOutput))

	g.window.SetContent(container.NewBorder(nil, g.actionButton, nil, nil, container.NewVScroll(container.NewVBox(
		statusCard,
		modeCard,
		protocolCard,
		proxyCard,
		transportCards(),
		utilityCard,
	))))
	g.window.SetCloseIntercept(func() {
		g.window.Hide()
	})
}

func transportCards() fyne.CanvasObject {
	return widget.NewCard("Transport cards", "Health updates every few seconds", container.NewGridWithColumns(3,
		widget.NewCard("Hysteria2", "UDP/443 fast path", widget.NewLabel("see tray/status updater")),
		widget.NewCard("Xray", "TCP/443 fallback", widget.NewLabel("see tray/status updater")),
		widget.NewCard("AmneziaWG", "WireGuard endpoint", widget.NewLabel("see tray/status updater")),
	))
}

func (g *DesktopGUI) applyState(state ViewState) {
	g.statusLabel.SetText(state.StatusLabel)
	g.modeLabel.SetText(state.ModeLabel)
	g.protocolLabel.SetText(state.ProtocolLabel)
	g.transportLabel.SetText(state.ActiveTransport)
	g.errorLabel.SetText(state.ErrorText)
	g.actionButton.SetText(state.PrimaryAction)
	if state.Mode == modes.Full {
		g.modeRadio.SetSelected("Full TUN")
	} else {
		g.modeRadio.SetSelected("Normal proxy")
	}
	g.protocolRadio.SetSelected(state.ProtocolLabel)
	if state.Connection == ConnectionConnecting {
		g.actionButton.Disable()
	} else {
		g.actionButton.Enable()
	}
}

func (g *DesktopGUI) refreshLoop() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for range ticker.C {
		g.controller.RefreshRuntimeState()
	}
}
```

- [ ] **Step 3: Fix transport cards to use live status labels**

Replace `transportCards()` with a `DesktopGUI` method and fields:

```go
	hysteriaStatus *widget.Label
	xrayStatus     *widget.Label
	amneziaStatus  *widget.Label
```

Add in `build()` before cards:

```go
	g.hysteriaStatus = widget.NewLabel("hysteria2: unknown")
	g.xrayStatus = widget.NewLabel("xray: unknown")
	g.amneziaStatus = widget.NewLabel("amneziawg: unknown")
```

Replace `transportCards()` call with `g.transportCards()` and add method:

```go
func (g *DesktopGUI) transportCards() fyne.CanvasObject {
	return widget.NewCard("Transport cards", "Health status from Mirage checker", container.NewGridWithColumns(3,
		widget.NewCard("Hysteria2", "UDP/443 fast path", g.hysteriaStatus),
		widget.NewCard("Xray", "TCP/443 fallback", g.xrayStatus),
		widget.NewCard("AmneziaWG", "WireGuard endpoint", g.amneziaStatus),
	))
}
```

- [ ] **Step 4: Add controller status snapshot for cards**

In `controller.go`, add to `ControllerDeps`:

```go
	TransportLabels() map[string]string
```

Add to fake deps in `controller_test.go`:

```go
func (f *fakeControllerDeps) TransportLabels() map[string]string {
	return map[string]string{
		"hysteria2": "hysteria2: healthy 55ms",
		"xray_reality_xhttp": "xray_reality_xhttp: healthy 42ms",
		"amneziawg": "amneziawg: unknown",
	}
}
```

Add method to `Controller`:

```go
func (c *Controller) TransportLabels() map[string]string {
	return c.deps.TransportLabels()
}
```

Add method to runtime deps:

```go
func (d *runtimeControllerDeps) TransportLabels() map[string]string {
	labels := map[string]string{
		"hysteria2": "hysteria2: not configured",
		"xray_reality_xhttp": "xray: not configured",
		"amneziawg": "amneziawg: not configured",
	}
	for name, status := range d.supervisor.Statuses() {
		labels[name] = status.Label()
	}
	return labels
}
```

Update `refreshLoop()` in `window.go`:

```go
func (g *DesktopGUI) refreshLoop() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for range ticker.C {
		g.controller.RefreshRuntimeState()
		labels := g.controller.TransportLabels()
		g.hysteriaStatus.SetText(labels["hysteria2"])
		g.xrayStatus.SetText(labels["xray_reality_xhttp"])
		g.amneziaStatus.SetText(labels["amneziawg"])
	}
}
```

- [ ] **Step 5: Replace no-arg launch path**

Modify `cmd/mirage-client/main.go` no-argument branch:

```go
	if len(os.Args) == 1 {
		gui.RunDesktop()
		return
	}
```

Add to `internal/gui/window.go` bottom:

```go
func RunDesktop() {
	s := getSupervisor()
	controller := NewController(s)
	desktop := NewDesktopGUI(controller)
	setDesktopWindow(desktop)
	go RunTray(controller)
	desktop.ShowAndRun()
}
```

- [ ] **Step 6: Run GUI package tests**

Run:

```powershell
go -C mirage test ./internal/gui -count=1
```

Expected: PASS. If Fyne requires Windows GUI build tags or missing C compiler, capture exact error and use Fyne's pure Go path only if available for current version; do not skip tests silently.

- [ ] **Step 7: Commit**

```powershell
git add mirage/go.mod mirage/go.sum mirage/cmd/mirage-client/main.go mirage/internal/gui/window.go
git commit -m "feat: add fyne main window"
```

---

### Task 5: Refactor tray to share controller and add Show window

**Files:**
- Modify: `internal/gui/tray.go`
- Modify: `internal/gui/tray_test.go`

- [ ] **Step 1: Write tray label helper tests**

Append to `internal/gui/tray_test.go`:

```go
func TestTrayConnectionTitles(t *testing.T) {
	connected, disconnected := trayConnectionTitles(true)
	if connected != "Connected" || disconnected != "Disconnect" {
		t.Fatalf("titles = %q/%q, want Connected/Disconnect", connected, disconnected)
	}
	connected, disconnected = trayConnectionTitles(false)
	if connected != "Connect" || disconnected != "Disconnected" {
		t.Fatalf("titles = %q/%q, want Connect/Disconnected", connected, disconnected)
	}
}
```

- [ ] **Step 2: Run test to verify failure**

Run:

```powershell
go -C mirage test ./internal/gui -run TestTrayConnectionTitles -count=1
```

Expected: FAIL with `undefined: trayConnectionTitles`.

- [ ] **Step 3: Add desktop window pointer and tray runner**

In `internal/gui/tray.go`, add globals:

```go
var (
	desktopMu     sync.RWMutex
	desktopWindow *DesktopGUI
	trayController *Controller
)
```

Add helpers:

```go
func setDesktopWindow(window *DesktopGUI) {
	desktopMu.Lock()
	desktopWindow = window
	desktopMu.Unlock()
}

func showDesktopWindow() {
	desktopMu.RLock()
	window := desktopWindow
	desktopMu.RUnlock()
	if window != nil {
		window.Show()
	}
}

func trayConnectionTitles(running bool) (string, string) {
	if running {
		return "Connected", "Disconnect"
	}
	return "Connect", "Disconnected"
}
```

- [ ] **Step 4: Add controller-aware tray runner**

Replace existing `Run()` with compatibility wrapper and new runner:

```go
func Run() {
	RunDesktop()
}

func RunTray(controller *Controller) {
	trayController = controller
	systray.Run(onReady, onExit)
}
```

- [ ] **Step 5: Add Show window menu item and controller actions**

In `onReady()`, add before Connect:

```go
	mShow := systray.AddMenuItem("Show window", "Open Mirage main window")
```

Change click loop cases:

```go
				case <-mShow.ClickedCh:
					showDesktopWindow()
				case <-mConnect.ClickedCh:
					go func() { _ = trayController.Connect() }()
				case <-mDisconnect.ClickedCh:
					go func() { _ = trayController.Disconnect() }()
				case <-mNormal.ClickedCh:
					go func() { _ = trayController.SetMode(modes.Normal) }()
				case <-mCheap.ClickedCh:
					go func() { _ = trayController.SetMode(modes.Normal) }()
				case <-mFull.ClickedCh:
					go func() { _ = trayController.SetMode(modes.Full) }()
```

Keep `Import Link...` if tray import dialog stays:

```go
				case <-mImport.ClickedCh:
					go doImportWithController()
```

Add helper:

```go
func doImportWithController() {
	f, err := dialog.File().Title("Select mirage link file").Load()
	if err != nil {
		return
	}
	if trayController != nil {
		_ = trayController.ImportProfile(f)
	}
}
```

- [ ] **Step 6: Update tray status refresh to use controller state**

Replace `refreshTrayStatus` body:

```go
func refreshTrayStatus(mConnect, mDisconnect *systray.MenuItem) {
	running := false
	if trayController != nil {
		running = trayController.State().Connection == ConnectionConnected
	} else {
		running = getSupervisor().IsRunning()
	}
	if mConnect != nil {
		mConnect.Enable()
	}
	if mDisconnect != nil {
		mDisconnect.Enable()
	}
	if running {
		if mConnect != nil {
			mConnect.Disable()
		}
	} else if mDisconnect != nil {
		mDisconnect.Disable()
	}
}
```

- [ ] **Step 7: Keep exit behavior disconnecting VPN**

Replace `onExit()` with:

```go
func onExit() {
	if trayController != nil {
		_ = trayController.Disconnect()
		return
	}
	doDisconnect()
}
```

- [ ] **Step 8: Run tray tests**

Run:

```powershell
go -C mirage test ./internal/gui -run "TestTray|TestConnectSuccess" -count=1
```

Expected: PASS.

- [ ] **Step 9: Commit**

```powershell
git add mirage/internal/gui/tray.go mirage/internal/gui/tray_test.go
git commit -m "feat: sync tray with gui controller"
```

---

### Task 6: Add error message helpers and doctor/log actions coverage

**Files:**
- Modify: `internal/gui/controller.go`
- Modify: `internal/gui/controller_test.go`

- [ ] **Step 1: Write missing profile and admin error tests**

Append to `internal/gui/controller_test.go`:

```go
func TestControllerImportFailureShowsVisibleError(t *testing.T) {
	deps := &fakeControllerDeps{}
	controller := NewControllerWithDeps(deps)
	controller.fail("Import profile first")
	state := controller.State()
	if state.Connection != ConnectionError {
		t.Fatalf("Connection = %q, want error", state.Connection)
	}
	if state.ErrorText != "Import profile first" {
		t.Fatalf("ErrorText = %q, want Import profile first", state.ErrorText)
	}
}

func TestDoctorOutputReturnedToWindow(t *testing.T) {
	deps := &fakeControllerDeps{}
	controller := NewControllerWithDeps(deps)
	out, err := controller.RunDoctor()
	if err != nil {
		t.Fatal(err)
	}
	if out != "doctor ok" {
		t.Fatalf("doctor output = %q, want doctor ok", out)
	}
}
```

- [ ] **Step 2: Run tests**

Run:

```powershell
go -C mirage test ./internal/gui -run "TestControllerImportFailure|TestDoctorOutput" -count=1
```

Expected: PASS after Task 3. If FAIL, fix controller to preserve exact error text.

- [ ] **Step 3: Confirm exact runtime messages**

Check `runtimeControllerDeps.Connect()` uses these exact messages:

```go
return fmt.Errorf("Import profile first")
return fmt.Errorf("sing-box.exe missing at %s", singBox)
return fmt.Errorf("TUN mode requires Administrator rights. Please restart Mirage as Administrator.")
```

- [ ] **Step 4: Run full GUI tests**

Run:

```powershell
go -C mirage test ./internal/gui -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit**

```powershell
git add mirage/internal/gui/controller.go mirage/internal/gui/controller_test.go
git commit -m "test: cover gui error state"
```

---

### Task 7: Build, run full tests, and manually verify GUI path

**Files:**
- No source changes expected. If failures require changes, edit exact failing files and re-run relevant tests first.

- [ ] **Step 1: Run package tests**

Run:

```powershell
go -C mirage test ./...
```

Expected: PASS.

- [ ] **Step 2: Build Windows client artifact into runtime bin folder**

Run:

```powershell
go -C mirage build -o bin\client\mirage-client.exe ./cmd/mirage-client
```

Expected: `mirage\bin\client\mirage-client.exe` exists and command exits 0.

- [ ] **Step 3: Build server artifacts into runtime bin folders**

Run:

```powershell
go -C mirage build -o bin\server\mirage-server.exe ./cmd/mirage-server
$env:GOOS='linux'; $env:GOARCH='amd64'; go -C mirage build -o bin\server\mirage-server-linux-amd64 ./cmd/mirage-server
Remove-Item Env:\GOOS
Remove-Item Env:\GOARCH
```

Expected: both server commands exit 0 and artifacts land in `mirage\bin\server\`.

- [ ] **Step 4: Launch GUI manually**

Run:

```powershell
.\mirage\bin\client\mirage-client.exe
```

Expected:
- Main window opens.
- Tray icon appears.
- Closing window hides it, app keeps running.
- Tray `Show window` reopens window.

- [ ] **Step 5: Verify disconnected UI**

In main window:
- Status label shows `Disconnected`.
- Mode shows `Normal proxy` by default.
- Protocol shows `Auto` by default.
- Primary button shows `Connect`.
- Proxy buttons visible.

- [ ] **Step 6: Verify connect error without profile**

Click `Connect` with no imported profile.

Expected:
- Window error text shows `Import profile first`.
- Error dialog shows same message.
- Status label shows `Error`.

- [ ] **Step 7: Verify normal connect path with imported profile**

Import existing Mirage profile file through `Import profile`, then click `Connect`.

Expected:
- Status changes `Connecting` then `Connected`.
- Tray Connect disabled within 1 second.
- Tray Disconnect enabled within 1 second.
- `curl -x http://127.0.0.1:2080 http://www.gstatic.com/generate_204` returns HTTP 204.

- [ ] **Step 8: Verify protocol restart flow**

While connected, select `Xray` protocol.

Expected:
- Primary button changes to `Restart to apply`.
- VPN does not restart until button clicked.
- Clicking `Restart to apply` disconnects then reconnects.
- Traffic still passes through `127.0.0.1:2080`.

- [ ] **Step 9: Verify TUN admin error**

Without Administrator rights, select `Full TUN`, then connect or restart.

Expected:
- Error text shows `TUN mode requires Administrator rights. Please restart Mirage as Administrator.`
- No false connected state.

- [ ] **Step 10: Verify disconnect cleanup**

Click `Disconnect`.

Expected:
- Status shows `Disconnected`.
- `sing-box.exe` and `xray.exe` Mirage sidecars stop.
- System proxy clears.
- Port `127.0.0.1:2080` closes.

- [ ] **Step 11: Commit verification fixes only if source changed**

If Step 1-10 required source edits in GUI files:

```powershell
git add mirage/internal/gui/state.go mirage/internal/gui/controller.go mirage/internal/gui/window.go mirage/internal/gui/tray.go mirage/internal/gui/state_test.go mirage/internal/gui/controller_test.go mirage/internal/gui/tray_test.go
git commit -m "fix: polish gui runtime behavior"
```

If Step 1-10 required source edits in app config files:

```powershell
git add mirage/internal/app/client.go mirage/internal/app/client_test.go
git commit -m "fix: polish protocol selection"
```

If no source edits, do not create an empty commit.

---

## Self-Review

**Spec coverage:**
- Launching no-arg client opens main window: Task 4.
- Close hides to tray: Task 4.
- Tray Show window / Connect / Disconnect / Status / Exit: Task 5, existing Status retained.
- Dashboard status/mode/protocol/transport: Tasks 1, 4.
- Primary action Connect/Disconnect/Restart: Tasks 1, 3, 4.
- Mode selector Normal proxy / Full TUN: Tasks 1, 3, 4.
- Protocol selector Auto/Hysteria2/Xray/AmneziaWG: Tasks 1, 2, 3, 4.
- Proxy controls: Tasks 3, 4.
- Transport cards: Task 4.
- Import profile / Run doctor / Open logs folder: Tasks 3, 4, 6.
- Connected changes require Restart to apply: Tasks 1, 3, 7.
- Connect failures visible in window and dialog: Tasks 3, 4, 6.
- Tray synced with real state: Tasks 3, 5.
- Disconnect cleans sidecars: existing `app.DisconnectClient`, used by Task 3/5, verified Task 7.
- Forced protocol config behavior: Task 2.
- Build artifacts into bin folders: Task 7.

**Known implementation risks:**
- `systray.Run` may need main-thread coordination with Fyne on Windows. If launch deadlocks, run systray first and schedule Fyne startup through `fyne.CurrentApp()` only after window created; keep controller as shared object.
- Fyne widgets should be updated from UI-safe context if current Fyne version requires it. If race or panic appears, wrap state application with `fyne.Do(func() { ... })` if available in installed Fyne version.
- Existing `Cheap` mode remains in code but main window only exposes `Normal proxy` and `Full TUN` because spec omits Cheap. Tray `Cheap` should either be hidden or map to Normal; Task 5 maps to Normal to avoid exposing out-of-scope UX.
