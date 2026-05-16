package gui

import (
	"errors"
	"testing"

	"mirage/internal/modes"
)

// ---------------------------------------------------------------------------
// fake controller deps
// ---------------------------------------------------------------------------

type fakeControllerDeps struct {
	running         bool
	connectErr      error
	disconnectErr   error
	modeSet         modes.Mode
	protocolSet     ProtocolSelection
	russianDirect   bool
	camouflageSite  string
	connectCount    int
	disconnectCount int
}

func (f *fakeControllerDeps) IsRunning() bool { return f.running }

func (f *fakeControllerDeps) Connect() error {
	f.connectCount++
	if f.connectErr != nil {
		return f.connectErr
	}
	f.running = true
	return nil
}

func (f *fakeControllerDeps) Disconnect() error {
	f.disconnectCount++
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

func (f *fakeControllerDeps) RussianDirect() bool { return f.russianDirect }

func (f *fakeControllerDeps) SetRussianDirect(enabled bool) error {
	f.russianDirect = enabled
	return nil
}

func (f *fakeControllerDeps) ImportProfile(path string) (string, error) {
	return "profile imported: default", nil
}

func (f *fakeControllerDeps) RunDoctor() (string, error) { return "doctor ok", nil }

func (f *fakeControllerDeps) OpenLogsFolder() error { return nil }

func (f *fakeControllerDeps) SetSystemProxy() error { return nil }

func (f *fakeControllerDeps) ClearSystemProxy() error { return nil }

func (f *fakeControllerDeps) BestTransportLabel() string { return "xray_reality_xhttp (42ms)" }

func (f *fakeControllerDeps) TransportLabels() map[string]string {
	return map[string]string{"xray_reality_xhttp": "xray: reachable (42ms)"}
}

func (f *fakeControllerDeps) Profiles() ([]ProfileState, error) {
	return []ProfileState{{ID: "main", Name: "Main VPS", Host: "203.0.113.10", Status: "online", LatencyMS: 42, Active: true}}, nil
}

func (f *fakeControllerDeps) SetActiveProfile(id string) error { return nil }

func (f *fakeControllerDeps) RemoveProfile(id string) error { return nil }

func (f *fakeControllerDeps) SetCamouflageSite(site string) error {
	f.camouflageSite = site
	return nil
}

// ---------------------------------------------------------------------------
// tests
// ---------------------------------------------------------------------------

func TestControllerConnectUpdatesState(t *testing.T) {
	deps := &fakeControllerDeps{}
	controller := NewControllerWithDeps(deps)
	if err := controller.Connect(); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	state := controller.State()
	if state.Connection != ConnectionConnected {
		t.Fatalf("Connection = %v, want %v", state.Connection, ConnectionConnected)
	}
	if state.StatusLabel != "Connected" {
		t.Fatalf("StatusLabel = %q, want Connected", state.StatusLabel)
	}
	if state.PrimaryAction != "Disconnect" {
		t.Fatalf("PrimaryAction = %q, want Disconnect", state.PrimaryAction)
	}
	if state.ActiveTransport != "xray_reality_xhttp (42ms)" {
		t.Fatalf("ActiveTransport = %q, want xray_reality_xhttp (42ms)", state.ActiveTransport)
	}
}

func TestControllerConnectFailureShowsError(t *testing.T) {
	deps := &fakeControllerDeps{
		connectErr: errors.New("sing-box missing at C:\\Mirage\\bin\\sing-box.exe"),
	}
	controller := NewControllerWithDeps(deps)
	if err := controller.Connect(); err == nil {
		t.Fatal("expected Connect to fail")
	}
	state := controller.State()
	if state.Connection != ConnectionError {
		t.Fatalf("Connection = %v, want %v", state.Connection, ConnectionError)
	}
	if state.ErrorText != deps.connectErr.Error() {
		t.Fatalf("ErrorText = %q, want %q", state.ErrorText, deps.connectErr.Error())
	}
}

func TestControllerModeChangeWhileConnectedRestartsImmediately(t *testing.T) {
	deps := &fakeControllerDeps{running: true}
	controller := NewControllerWithDeps(deps)
	controller.RefreshRuntimeState()
	if err := controller.SetMode(modes.Full); err != nil {
		t.Fatalf("SetMode: %v", err)
	}
	state := controller.State()
	if state.RestartRequired {
		t.Fatal("RestartRequired should be cleared after immediate reconnect")
	}
	if state.Connection != ConnectionConnected {
		t.Fatalf("Connection = %v, want connected", state.Connection)
	}
	if deps.disconnectCount != 1 || deps.connectCount != 1 {
		t.Fatalf("restart calls disconnect/connect = %d/%d, want 1/1", deps.disconnectCount, deps.connectCount)
	}
}

func TestControllerProtocolChangeWhileConnectedRestartsImmediately(t *testing.T) {
	deps := &fakeControllerDeps{running: true}
	controller := NewControllerWithDeps(deps)
	controller.RefreshRuntimeState()
	if err := controller.SetProtocol(ProtocolXray); err != nil {
		t.Fatalf("SetProtocol: %v", err)
	}
	state := controller.State()
	if state.Connection != ConnectionConnected {
		t.Fatalf("Connection = %v, want %v after restart", state.Connection, ConnectionConnected)
	}
	if state.RestartRequired {
		t.Fatal("expected RestartRequired cleared after restart")
	}
	if state.Protocol != ProtocolXray {
		t.Fatalf("Protocol = %q, want %q", state.Protocol, ProtocolXray)
	}
	if deps.disconnectCount != 1 || deps.connectCount != 1 {
		t.Fatalf("restart calls disconnect/connect = %d/%d, want 1/1", deps.disconnectCount, deps.connectCount)
	}
}

func TestControllerDisconnectUpdatesState(t *testing.T) {
	deps := &fakeControllerDeps{running: true}
	controller := NewControllerWithDeps(deps)
	controller.RefreshRuntimeState()
	state := controller.State()
	if state.Connection != ConnectionConnected {
		t.Fatalf("pre-condition Connection = %v, want %v", state.Connection, ConnectionConnected)
	}
	if err := controller.Disconnect(); err != nil {
		t.Fatalf("Disconnect: %v", err)
	}
	state = controller.State()
	if state.Connection != ConnectionDisconnected {
		t.Fatalf("Connection = %v, want %v", state.Connection, ConnectionDisconnected)
	}
	if state.PrimaryAction != "Connect" {
		t.Fatalf("PrimaryAction = %q, want Connect", state.PrimaryAction)
	}
}

func TestControllerPrimaryActionConnectsWhenDisconnected(t *testing.T) {
	deps := &fakeControllerDeps{}
	controller := NewControllerWithDeps(deps)
	state := controller.State()
	if state.Connection != ConnectionDisconnected {
		t.Fatalf("pre-condition Connection = %v", state.Connection)
	}
	if err := controller.PrimaryAction(); err != nil {
		t.Fatalf("PrimaryAction: %v", err)
	}
	state = controller.State()
	if state.Connection != ConnectionConnected {
		t.Fatalf("Connection = %v, want %v", state.Connection, ConnectionConnected)
	}
}

func TestControllerPrimaryActionDisconnectsWhenConnected(t *testing.T) {
	deps := &fakeControllerDeps{running: true}
	controller := NewControllerWithDeps(deps)
	controller.RefreshRuntimeState()
	state := controller.State()
	if state.Connection != ConnectionConnected {
		t.Fatalf("pre-condition Connection = %v", state.Connection)
	}
	if err := controller.PrimaryAction(); err != nil {
		t.Fatalf("PrimaryAction: %v", err)
	}
	state = controller.State()
	if state.Connection != ConnectionDisconnected {
		t.Fatalf("Connection = %v, want %v", state.Connection, ConnectionDisconnected)
	}
}

func TestControllerImportFailureShowsVisibleError(t *testing.T) {
	deps := &fakeControllerDeps{}
	controller := NewControllerWithDeps(deps)
	// Simulate an error message being surfaced through the fail pathway.
	controller.fail("Import profile first")
	state := controller.State()
	if state.Connection != ConnectionError {
		t.Fatalf("Connection = %v, want %v", state.Connection, ConnectionError)
	}
	if state.ErrorText != "Import profile first" {
		t.Fatalf("ErrorText = %q, want %q", state.ErrorText, "Import profile first")
	}
}

func TestControllerRunDoctorReturnsOutput(t *testing.T) {
	deps := &fakeControllerDeps{}
	controller := NewControllerWithDeps(deps)
	out, err := controller.RunDoctor()
	if err != nil {
		t.Fatalf("RunDoctor: %v", err)
	}
	if out != "doctor ok" {
		t.Fatalf("RunDoctor = %q, want doctor ok", out)
	}
}

func TestControllerSetsCamouflageSite(t *testing.T) {
	deps := &fakeControllerDeps{}
	controller := NewControllerWithDeps(deps)
	if err := controller.SetCamouflageSite("www.cloudflare.com"); err != nil {
		t.Fatal(err)
	}
	if deps.camouflageSite != "www.cloudflare.com" {
		t.Fatalf("site = %q", deps.camouflageSite)
	}
	state := controller.State()
	if !state.ReconnectRequired {
		t.Fatal("ReconnectRequired = false")
	}
}

func TestControllerTransportLabels(t *testing.T) {
	deps := &fakeControllerDeps{}
	controller := NewControllerWithDeps(deps)
	labels := controller.TransportLabels()
	if label, ok := labels["xray_reality_xhttp"]; !ok || label != "xray: reachable (42ms)" {
		t.Fatalf("TransportLabels xray_reality_xhttp = %q (ok=%v), want xray: reachable (42ms)", label, ok)
	}
}

func TestControllerOnChangeCallback(t *testing.T) {
	deps := &fakeControllerDeps{}
	controller := NewControllerWithDeps(deps)
	var capturedState *ViewState
	controller.SetOnChange(func(vs ViewState) {
		capturedState = &vs
	})
	_ = controller.Connect()
	if capturedState == nil {
		t.Fatal("expected onChange to fire")
	}
	if capturedState.Connection != ConnectionConnected {
		t.Fatalf("Connection = %v, want %v", capturedState.Connection, ConnectionConnected)
	}
}

func TestControllerOnErrorCallback(t *testing.T) {
	deps := &fakeControllerDeps{}
	controller := NewControllerWithDeps(deps)
	var capturedErr string
	controller.SetOnError(func(msg string) {
		capturedErr = msg
	})
	controller.fail("something went wrong")
	if capturedErr != "something went wrong" {
		t.Fatalf("onError = %q, want something went wrong", capturedErr)
	}
}

func TestControllerOnMessageCallback(t *testing.T) {
	deps := &fakeControllerDeps{}
	controller := NewControllerWithDeps(deps)
	var capturedMsg string
	controller.SetOnMessage(func(msg string) {
		capturedMsg = msg
	})
	controller.message("hello world")
	if capturedMsg != "hello world" {
		t.Fatalf("onMessage = %q, want hello world", capturedMsg)
	}
}

func TestControllerClearErrorOnReconnect(t *testing.T) {
	deps := &fakeControllerDeps{}
	controller := NewControllerWithDeps(deps)
	controller.fail("temporary error")
	state := controller.State()
	if state.Connection != ConnectionError {
		t.Fatalf("pre-condition Connection = %v", state.Connection)
	}
	// Connecting should clear the error state.
	if err := controller.Connect(); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	state = controller.State()
	if state.Connection != ConnectionConnected {
		t.Fatalf("Connection = %v, want %v", state.Connection, ConnectionConnected)
	}
	if state.ErrorText != "" {
		t.Fatalf("ErrorText = %q, want empty", state.ErrorText)
	}
}

// ---------------------------------------------------------------------------
// deps call-through tests
// ---------------------------------------------------------------------------

func TestControllerSetModeCallsDeps(t *testing.T) {
	deps := &fakeControllerDeps{}
	controller := NewControllerWithDeps(deps)
	if err := controller.SetMode(modes.Cheap); err != nil {
		t.Fatalf("SetMode: %v", err)
	}
	if deps.modeSet != modes.Cheap {
		t.Fatalf("deps.modeSet = %q, want %q", deps.modeSet, modes.Cheap)
	}
}

func TestControllerSetProtocolCallsDeps(t *testing.T) {
	deps := &fakeControllerDeps{}
	controller := NewControllerWithDeps(deps)
	if err := controller.SetProtocol(ProtocolHysteria2); err != nil {
		t.Fatalf("SetProtocol: %v", err)
	}
	if deps.protocolSet != ProtocolHysteria2 {
		t.Fatalf("deps.protocolSet = %q, want %q", deps.protocolSet, ProtocolHysteria2)
	}
}

func TestControllerSetRussianDirectCallsDeps(t *testing.T) {
	deps := &fakeControllerDeps{russianDirect: true}
	controller := NewControllerWithDeps(deps)
	if err := controller.SetRussianDirect(false); err != nil {
		t.Fatalf("SetRussianDirect: %v", err)
	}
	if deps.russianDirect {
		t.Fatal("deps.russianDirect = true, want false")
	}
	if controller.State().RussianDirect {
		t.Fatal("state RussianDirect = true, want false")
	}
}

func TestControllerDisconnectError(t *testing.T) {
	deps := &fakeControllerDeps{
		running:       true,
		disconnectErr: errors.New("taskkill failed"),
	}
	controller := NewControllerWithDeps(deps)
	controller.RefreshRuntimeState()
	if err := controller.Disconnect(); err == nil {
		t.Fatal("expected Disconnect to fail")
	}
	state := controller.State()
	if state.Connection != ConnectionError {
		t.Fatalf("Connection = %v, want %v", state.Connection, ConnectionError)
	}
	if state.ErrorText != "taskkill failed" {
		t.Fatalf("ErrorText = %q, want taskkill failed", state.ErrorText)
	}
}

func TestControllerRestartClearsError(t *testing.T) {
	deps := &fakeControllerDeps{running: true}
	controller := NewControllerWithDeps(deps)
	controller.RefreshRuntimeState()
	controller.fail("previous error")
	if err := controller.Restart(); err != nil {
		t.Fatalf("Restart: %v", err)
	}
	state := controller.State()
	if state.Connection != ConnectionConnected {
		t.Fatalf("Connection = %v, want %v", state.Connection, ConnectionConnected)
	}
	if state.RestartRequired {
		t.Fatal("expected RestartRequired cleared after restart")
	}
	if state.ErrorText != "" {
		t.Fatalf("ErrorText = %q, want empty", state.ErrorText)
	}
}
