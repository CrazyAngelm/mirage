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
	"mirage/internal/profiles"
	"mirage/internal/sysproxy"
)

// ControllerDeps abstracts external side-effects so the Controller can be
// unit-tested with a fake implementation.
type ControllerDeps interface {
	IsRunning() bool
	Connect() error
	Disconnect() error
	SetMode(mode modes.Mode) error
	SetProtocol(protocol ProtocolSelection) error
	RussianDirect() bool
	SetRussianDirect(enabled bool) error
	ImportProfile(path string) (string, error)
	RunDoctor() (string, error)
	OpenLogsFolder() error
	SetSystemProxy() error
	ClearSystemProxy() error
	BestTransportLabel() string
	TransportLabels() map[string]string
	Profiles() ([]ProfileState, error)
	SetActiveProfile(id string) error
	RemoveProfile(id string) error
	SetCamouflageSite(site string) error
}

// Controller owns the view state and coordinates actions between the UI
// layer and the underlying supervisor / app packages.
type Controller struct {
	actionMu  sync.Mutex
	mu        sync.RWMutex
	state     ViewState
	deps      ControllerDeps
	onChange  func(ViewState)
	onError   func(string)
	onMessage func(string)
}

// NewController creates a Controller wired to the real runtime dependencies.
func NewController(supervisor *Supervisor) *Controller {
	return NewControllerWithDeps(newRuntimeControllerDeps(supervisor))
}

// NewControllerWithDeps creates a Controller with the given dependency
// implementation (real or fake).
func NewControllerWithDeps(deps ControllerDeps) *Controller {
	controller := &Controller{state: InitialViewState(), deps: deps}
	controller.RefreshRuntimeState()
	return controller
}

// SetOnChange registers a callback that fires every time the view state
// changes. It is safe to call from any goroutine but should be called
// before the controller is used from multiple goroutines.
func (c *Controller) SetOnChange(fn func(ViewState)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.onChange = fn
}

// SetOnError registers a callback for transient error messages that should
// be surfaced to the user (e.g. via a dialog or status bar).
func (c *Controller) SetOnError(fn func(string)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.onError = fn
}

// SetOnMessage registers a callback for informational messages.
func (c *Controller) SetOnMessage(fn func(string)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.onMessage = fn
}

// State returns a snapshot of the current view state.
func (c *Controller) State() ViewState {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.state
}

// RefreshRuntimeState re-reads IsRunning and BestTransportLabel from the
// dependencies and updates the state without side effects.
// Does not override transient ConnectionConnecting state to avoid races
// with connectLocked.
func (c *Controller) RefreshRuntimeState() {
	c.mu.RLock()
	currentCS := c.state.Connection
	c.mu.RUnlock()

	if currentCS == ConnectionConnecting {
		return
	}

	cs := ConnectionDisconnected
	if c.deps.IsRunning() {
		cs = ConnectionConnected
	}
	label := c.deps.BestTransportLabel()
	russianDirect := c.deps.RussianDirect()
	profileItems, _ := c.deps.Profiles()

	c.mu.Lock()
	next := c.state.WithConnection(cs).WithRussianDirect(russianDirect)
	next.Profiles = profileItems
	for _, profile := range profileItems {
		if profile.Active {
			next.ActiveProfileID = profile.ID
			next.ActiveProfileName = profile.Name
			store := profiles.NewStore(filepath.Join(platform.ClientBaseDir(), "state"))
			if active, err := store.Active(); err == nil {
				next.CamouflageSite = active.CamouflageSite()
			}
		}
	}
	if label != "" {
		next.ActiveTransport = label
	}
	c.state = next
	onChange := c.onChange
	c.mu.Unlock()

	if onChange != nil {
		onChange(next)
	}
}

// PrimaryAction connects when disconnected, disconnects when connected,
// and performs a restart when RestartRequired is true.
func (c *Controller) PrimaryAction() error {
	c.actionMu.Lock()
	defer c.actionMu.Unlock()
	return c.primaryActionLocked()
}

func (c *Controller) primaryActionLocked() error {
	c.mu.RLock()
	restart := c.state.RestartRequired
	connected := c.state.Connection == ConnectionConnected
	c.mu.RUnlock()

	if restart {
		return c.restartLocked()
	}
	if connected {
		return c.disconnectLocked()
	}
	return c.connectLocked()
}

// Connect starts the VPN and updates state on success.
// The deps layer handles sysproxy.Set for non-TUN modes; the controller
// only manages view-state transitions.
func (c *Controller) Connect() error {
	c.actionMu.Lock()
	defer c.actionMu.Unlock()
	return c.connectLocked()
}

func (c *Controller) connectLocked() error {
	c.updateState(func(s ViewState) ViewState { return s.WithConnection(ConnectionConnecting) })
	if err := c.deps.Connect(); err != nil {
		c.fail(err.Error())
		return err
	}
	label := c.deps.BestTransportLabel()
	c.updateState(func(s ViewState) ViewState {
		ns := s.WithConnection(ConnectionConnected)
		if label != "" {
			ns.ActiveTransport = label
		}
		return ns
	})
	return nil
}

// Disconnect stops the VPN and updates state on success.
// The deps layer handles supervisor shutdown and proxy cleanup.
func (c *Controller) Disconnect() error {
	c.actionMu.Lock()
	defer c.actionMu.Unlock()
	return c.disconnectLocked()
}

func (c *Controller) disconnectLocked() error {
	if err := c.deps.Disconnect(); err != nil {
		c.fail(err.Error())
		return err
	}
	c.updateState(func(s ViewState) ViewState { return s.WithConnection(ConnectionDisconnected) })
	return nil
}

// Restart disconnects and then reconnects. It is used when mode or protocol
// changes require a restart to take effect.
func (c *Controller) Restart() error {
	c.actionMu.Lock()
	defer c.actionMu.Unlock()
	return c.restartLocked()
}

func (c *Controller) restartLocked() error {
	if err := c.deps.Disconnect(); err != nil {
		c.fail(fmt.Sprintf("disconnect before restart: %v", err))
		return err
	}
	c.updateState(func(s ViewState) ViewState {
		ns := s.WithConnection(ConnectionDisconnected)
		ns.RestartRequired = false
		return ns
	})
	return c.connectLocked()
}

// SetMode updates the operating mode and reconnects immediately when needed.
func (c *Controller) SetMode(mode modes.Mode) error {
	c.actionMu.Lock()
	defer c.actionMu.Unlock()

	state := c.State()
	changedWhileConnected := state.Connection == ConnectionConnected && modes.Normalize(string(mode)) != state.Mode
	if err := c.deps.SetMode(mode); err != nil {
		c.fail(err.Error())
		return err
	}
	c.updateState(func(s ViewState) ViewState { return s.WithMode(string(mode)) })
	if changedWhileConnected {
		c.message(fmt.Sprintf("Mode set to %s, reconnecting", mode))
		return c.restartLocked()
	}
	c.message(fmt.Sprintf("Mode set to %s", mode))
	return nil
}

// SetProtocol updates the preferred transport protocol and reconnects immediately when needed.
func (c *Controller) SetProtocol(protocol ProtocolSelection) error {
	c.actionMu.Lock()
	defer c.actionMu.Unlock()

	state := c.State()
	if !validProtocol(protocol) {
		protocol = ProtocolAuto
	}
	changedWhileConnected := state.Connection == ConnectionConnected && protocol != state.Protocol
	if err := c.deps.SetProtocol(protocol); err != nil {
		c.fail(err.Error())
		return err
	}
	c.updateState(func(s ViewState) ViewState { return s.WithProtocol(protocol) })
	if changedWhileConnected {
		c.message(fmt.Sprintf("Protocol set to %s, reconnecting", protocol))
		return c.restartLocked()
	}
	c.message(fmt.Sprintf("Protocol set to %s", protocol))
	return nil
}

func (c *Controller) SetRussianDirect(enabled bool) error {
	c.actionMu.Lock()
	defer c.actionMu.Unlock()

	state := c.State()
	changedWhileConnected := state.Connection == ConnectionConnected && enabled != state.RussianDirect
	if err := c.deps.SetRussianDirect(enabled); err != nil {
		c.fail(err.Error())
		return err
	}
	c.updateState(func(s ViewState) ViewState { return s.WithRussianDirect(enabled) })
	if changedWhileConnected {
		c.message(russianDirectLabel(enabled) + ", reconnecting")
		return c.restartLocked()
	}
	c.message(russianDirectLabel(enabled))
	return nil
}

// ImportProfile imports a mirage:// link file.
func (c *Controller) ImportProfile(path string) error {
	out, err := c.deps.ImportProfile(path)
	if err != nil {
		c.fail(err.Error())
		return err
	}
	c.message(out)
	c.RefreshRuntimeState()
	return nil
}

// RunDoctor returns a diagnostics summary.
func (c *Controller) RunDoctor() (string, error) { return c.deps.RunDoctor() }

// OpenLogsFolder opens the client logs directory.
func (c *Controller) OpenLogsFolder() error { return c.deps.OpenLogsFolder() }

// SetSystemProxy enables the system HTTP/SOCKS proxy.
func (c *Controller) SetSystemProxy() error { return c.deps.SetSystemProxy() }

// ClearSystemProxy removes the system HTTP/SOCKS proxy.
func (c *Controller) ClearSystemProxy() error { return c.deps.ClearSystemProxy() }

// TransportLabels returns per-transport status labels.
func (c *Controller) TransportLabels() map[string]string { return c.deps.TransportLabels() }

func (c *Controller) Profiles() []ProfileState {
	profiles, err := c.deps.Profiles()
	if err != nil {
		return nil
	}
	return profiles
}

func (c *Controller) SetActiveProfile(id string) error {
	if err := c.deps.SetActiveProfile(id); err != nil {
		c.fail(err.Error())
		return err
	}
	c.RefreshRuntimeState()
	return nil
}

func (c *Controller) RemoveProfile(id string) error {
	if err := c.deps.RemoveProfile(id); err != nil {
		c.fail(err.Error())
		return err
	}
	c.RefreshRuntimeState()
	return nil
}

func (c *Controller) SetCamouflageSite(site string) error {
	if err := c.deps.SetCamouflageSite(site); err != nil {
		c.fail(err.Error())
		return err
	}
	c.updateState(func(s ViewState) ViewState {
		s.CamouflageSite = site
		s.ReconnectRequired = true
		s.RestartRequired = true
		return refreshLabels(s)
	})
	return nil
}

// ---------------------------------------------------------------------------
// internal helpers
// ---------------------------------------------------------------------------

// updateState atomically transforms the state under the write lock and
// fires the onChange callback (if set) outside the lock.
func (c *Controller) updateState(fn func(ViewState) ViewState) {
	c.mu.Lock()
	c.state = fn(c.state)
	onChange := c.onChange
	state := c.state
	c.mu.Unlock()

	if onChange != nil {
		onChange(state)
	}
}

func (c *Controller) fail(message string) {
	c.updateState(func(s ViewState) ViewState { return s.WithError(message) })

	c.mu.RLock()
	onError := c.onError
	c.mu.RUnlock()

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

// ---------------------------------------------------------------------------
// runtime controller deps - wired to the real Supervisor and app packages
// ---------------------------------------------------------------------------

type runtimeControllerDeps struct {
	supervisor *Supervisor
}

func newRuntimeControllerDeps(supervisor *Supervisor) *runtimeControllerDeps {
	return &runtimeControllerDeps{supervisor: supervisor}
}

func (d *runtimeControllerDeps) IsRunning() bool { return d.supervisor.IsRunning() }

func (d *runtimeControllerDeps) Connect() error {
	base := platform.ClientBaseDir()
	cfg := filepath.Join(base, "configs", "sing-box.json")
	if _, err := os.Stat(cfg); err != nil {
		return fmt.Errorf("Import profile first")
	}
	// Regenerate config to ensure it matches current state (protocol, russianDirect, mode)
	link, err := app.ReadProfileLink(base)
	if err != nil {
		return fmt.Errorf("no imported profile")
	}
	b, err := bundle.Decode(link)
	if err != nil {
		return fmt.Errorf("invalid profile")
	}
	mode := app.ReadClientMode(base)
	if err := app.RegenerateConfig(base, b, mode); err != nil {
		return fmt.Errorf("regenerate config: %w", err)
	}
	if modes.IsTun(mode) && !sysproxy.IsAdmin() {
		return fmt.Errorf("TUN mode requires Administrator rights. Please restart Mirage as Administrator.")
	}
	singBox := filepath.Join(base, "bin", "sing-box.exe")
	if _, err := os.Stat(singBox); err != nil {
		return fmt.Errorf("sing-box.exe missing at %s", singBox)
	}
	if err := d.supervisor.Start(singBox, cfg, mode); err != nil {
		return fmt.Errorf("failed to start: %v", err)
	}
	if !modes.IsTun(mode) {
		if err := sysproxy.Set("127.0.0.1:2080"); err != nil {
			// non-fatal: proxy can be configured manually
			return nil
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
	link, err := app.ReadProfileLink(base)
	if err != nil {
		return fmt.Errorf("no imported profile")
	}
	b, err := bundle.Decode(link)
	if err != nil {
		return fmt.Errorf("invalid profile")
	}
	return app.SetClientModeGUI(base, string(mode), b)
}

func (d *runtimeControllerDeps) SetProtocol(protocol ProtocolSelection) error {
	if !validProtocol(protocol) {
		return fmt.Errorf("unknown protocol: %s", protocol)
	}
	base := platform.ClientBaseDir()
	if err := app.SetClientProtocol(base, string(protocol)); err != nil {
		return err
	}
	link, err := app.ReadProfileLink(base)
	if err != nil {
		return fmt.Errorf("no imported profile")
	}
	b, err := bundle.Decode(link)
	if err != nil {
		return fmt.Errorf("invalid profile")
	}
	mode := app.ReadClientMode(base)
	return app.SetClientModeGUI(base, string(mode), b)
}

func (d *runtimeControllerDeps) RussianDirect() bool {
	return app.ReadRussianDirect(platform.ClientBaseDir())
}

func (d *runtimeControllerDeps) SetRussianDirect(enabled bool) error {
	base := platform.ClientBaseDir()
	link, err := app.ReadProfileLink(base)
	if err != nil {
		return fmt.Errorf("no imported profile")
	}
	b, err := bundle.Decode(link)
	if err != nil {
		return fmt.Errorf("invalid profile")
	}
	return app.SetRussianDirectGUI(base, enabled, b)
}

func (d *runtimeControllerDeps) ImportProfile(path string) (string, error) {
	var out strings.Builder
	if err := app.ImportProfileGUI(path, &out); err != nil {
		return "", err
	}
	return out.String(), nil
}

func (d *runtimeControllerDeps) RunDoctor() (string, error) {
	var out strings.Builder
	if err := app.RunClient([]string{"doctor"}, &out, &out); err != nil {
		return "", err
	}
	return out.String(), nil
}

func (d *runtimeControllerDeps) OpenLogsFolder() error {
	logsDir := filepath.Join(platform.ClientBaseDir(), "logs")
	_ = os.MkdirAll(logsDir, 0755)
	return exec.Command("explorer", logsDir).Start()
}

func (d *runtimeControllerDeps) SetSystemProxy() error {
	return sysproxy.Set("127.0.0.1:2080")
}

func (d *runtimeControllerDeps) ClearSystemProxy() error {
	return sysproxy.Unset()
}

func (d *runtimeControllerDeps) BestTransportLabel() string {
	best, ok := d.supervisor.BestTransport()
	if !ok {
		return ""
	}
	return best.Label()
}

func (d *runtimeControllerDeps) TransportLabels() map[string]string {
	out := make(map[string]string)
	for name, status := range d.supervisor.Statuses() {
		out[name] = status.Label()
	}
	return out
}

func (d *runtimeControllerDeps) Profiles() ([]ProfileState, error) {
	store := profiles.NewStore(filepath.Join(platform.ClientBaseDir(), "state"))
	items, err := store.List()
	if err != nil {
		return nil, err
	}
	activeID := ""
	if active, err := store.Active(); err == nil {
		activeID = active.ID
	}
	out := make([]ProfileState, 0, len(items))
	for _, item := range items {
		b, _ := item.Bundle()
		out = append(out, ProfileState{ID: item.ID, Name: item.DisplayName, Host: b.ServerHost(), Status: item.LastStatus, LatencyMS: item.LastLatencyMS, Active: item.ID == activeID})
	}
	return out, nil
}

func (d *runtimeControllerDeps) SetActiveProfile(id string) error {
	store := profiles.NewStore(filepath.Join(platform.ClientBaseDir(), "state"))
	return store.SetActive(id)
}

func (d *runtimeControllerDeps) RemoveProfile(id string) error {
	store := profiles.NewStore(filepath.Join(platform.ClientBaseDir(), "state"))
	return store.Remove(id)
}

func (d *runtimeControllerDeps) SetCamouflageSite(site string) error {
	store := profiles.NewStore(filepath.Join(platform.ClientBaseDir(), "state"))
	active, err := store.Active()
	if err != nil {
		return err
	}
	return store.SetCamouflageOverride(active.ID, site)
}
