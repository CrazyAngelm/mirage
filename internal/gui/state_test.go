package gui

import (
	"testing"

	"mirage/internal/modes"
)

func TestInitialViewStateIsDisconnectedAutoNormal(t *testing.T) {
	vs := InitialViewState()

	if vs.Connection != ConnectionDisconnected {
		t.Errorf("Initial Connection = %q, want %q", vs.Connection, ConnectionDisconnected)
	}
	if vs.Mode != modes.Normal {
		t.Errorf("Initial Mode = %q, want %q", vs.Mode, modes.Normal)
	}
	if vs.Protocol != ProtocolAuto {
		t.Errorf("Initial Protocol = %q, want %q", vs.Protocol, ProtocolAuto)
	}
	if vs.StatusLabel != "Disconnected" {
		t.Errorf("Initial StatusLabel = %q, want %q", vs.StatusLabel, "Disconnected")
	}
	if vs.ModeLabel != "Normal proxy" {
		t.Errorf("Initial ModeLabel = %q, want %q", vs.ModeLabel, "Normal proxy")
	}
	if vs.ProtocolLabel != "Auto" {
		t.Errorf("Initial ProtocolLabel = %q, want %q", vs.ProtocolLabel, "Auto")
	}
	if vs.PrimaryAction != "Connect" {
		t.Errorf("Initial PrimaryAction = %q, want %q", vs.PrimaryAction, "Connect")
	}
	if vs.RestartRequired {
		t.Error("Initial RestartRequired should be false")
	}
	if vs.ActiveTransport != "" {
		t.Errorf("Initial ActiveTransport = %q, want empty", vs.ActiveTransport)
	}
	if vs.ErrorText != "" {
		t.Errorf("Initial ErrorText = %q, want empty", vs.ErrorText)
	}
}

func TestConnectedModeChangeRequiresRestart(t *testing.T) {
	vs := InitialViewState().WithConnection(ConnectionConnected)
	if vs.RestartRequired {
		t.Error("After connecting, RestartRequired should be false")
	}
	if vs.PrimaryAction != "Disconnect" {
		t.Errorf("After connecting, PrimaryAction = %q, want %q", vs.PrimaryAction, "Disconnect")
	}
	if vs.StatusLabel != "Connected" {
		t.Errorf("After connecting, StatusLabel = %q, want %q", vs.StatusLabel, "Connected")
	}

	vs2 := vs.WithMode("full")
	if !vs2.RestartRequired {
		t.Error("After mode change while connected, RestartRequired should be true")
	}
	if vs2.PrimaryAction != "Restart to apply" {
		t.Errorf("After mode change while connected, PrimaryAction = %q, want %q",
			vs2.PrimaryAction, "Restart to apply")
	}
	if vs2.ModeLabel != "Full TUN" {
		t.Errorf("After mode change to Full, ModeLabel = %q, want %q",
			vs2.ModeLabel, "Full TUN")
	}
	if vs2.Connection != ConnectionConnected {
		t.Errorf("Mode change should preserve connected state, got %q", vs2.Connection)
	}

	// Verify that WithMode does not mutate the receiver.
	if vs.RestartRequired {
		t.Error("Original state should not be mutated by WithMode")
	}
	if vs.PrimaryAction != "Disconnect" {
		t.Errorf("Original state PrimaryAction should still be Disconnect, got %q", vs.PrimaryAction)
	}
}

func TestConnectedRussianDirectChangeRequiresRestart(t *testing.T) {
	vs := InitialViewState().WithConnection(ConnectionConnected)
	vs2 := vs.WithRussianDirect(true)
	if !vs2.RestartRequired {
		t.Fatal("After RU-direct change while connected, RestartRequired should be true")
	}
	if vs2.PrimaryAction != "Restart to apply" {
		t.Fatalf("PrimaryAction = %q, want Restart to apply", vs2.PrimaryAction)
	}
	if vs2.RussianDirectLabel != "RU sites direct" {
		t.Fatalf("RussianDirectLabel = %q", vs2.RussianDirectLabel)
	}
}

func TestConnectedProtocolChangeRequiresRestart(t *testing.T) {
	vs := InitialViewState().WithConnection(ConnectionConnected)
	if vs.RestartRequired {
		t.Error("After connecting, RestartRequired should be false")
	}

	vs2 := vs.WithProtocol(ProtocolHysteria2)
	if !vs2.RestartRequired {
		t.Error("After protocol change while connected, RestartRequired should be true")
	}
	if vs2.PrimaryAction != "Restart to apply" {
		t.Errorf("After protocol change while connected, PrimaryAction = %q, want %q",
			vs2.PrimaryAction, "Restart to apply")
	}
	if vs2.ProtocolLabel != "Hysteria2" {
		t.Errorf("After protocol change to Hysteria2, ProtocolLabel = %q, want %q",
			vs2.ProtocolLabel, "Hysteria2")
	}
	if vs2.Connection != ConnectionConnected {
		t.Errorf("Protocol change should preserve connected state, got %q", vs2.Connection)
	}

	// Verify immutability.
	if vs.RestartRequired {
		t.Error("Original state should not be mutated by WithProtocol")
	}

	// Reconnecting clears the restart flag.
	vs3 := vs2.WithConnection(ConnectionConnected)
	if vs3.RestartRequired {
		t.Error("After reconnecting, RestartRequired should be false")
	}
	if vs3.PrimaryAction != "Disconnect" {
		t.Errorf("After reconnecting, PrimaryAction = %q, want %q",
			vs3.PrimaryAction, "Disconnect")
	}

	// Invalid protocol defaults to Auto.
	vs4 := vs.WithProtocol(ProtocolSelection("bogus_protocol"))
	if vs4.Protocol != ProtocolAuto {
		t.Errorf("WithProtocol(bogus) = %q, want %q", vs4.Protocol, ProtocolAuto)
	}
}

func TestConnectingAndErrorLabels(t *testing.T) {
	vs := InitialViewState()

	vs1 := vs.WithConnection(ConnectionConnecting)
	if vs1.StatusLabel != "Connecting" {
		t.Errorf("Connecting StatusLabel = %q, want %q", vs1.StatusLabel, "Connecting")
	}
	if vs1.PrimaryAction != "Connecting" {
		t.Errorf("Connecting PrimaryAction = %q, want %q", vs1.PrimaryAction, "Connecting")
	}
	if vs1.Connection != ConnectionConnecting {
		t.Errorf("Connection = %q, want %q", vs1.Connection, ConnectionConnecting)
	}

	vs2 := vs.WithError("connection refused")
	if vs2.Connection != ConnectionError {
		t.Errorf("After WithError, Connection = %q, want %q", vs2.Connection, ConnectionError)
	}
	if vs2.StatusLabel != "Error" {
		t.Errorf("Error StatusLabel = %q, want %q", vs2.StatusLabel, "Error")
	}
	if vs2.ErrorText != "connection refused" {
		t.Errorf("Error ErrorText = %q, want %q", vs2.ErrorText, "connection refused")
	}
	if vs2.PrimaryAction != "Connect" {
		t.Errorf("Error PrimaryAction = %q, want %q", vs2.PrimaryAction, "Connect")
	}

	conn := InitialViewState().WithConnection(ConnectionConnected).WithMode("full")
	errState := conn.WithError("timeout")
	if errState.Connection != ConnectionError {
		t.Errorf("Connection = %q, want %q", errState.Connection, ConnectionError)
	}
	if errState.StatusLabel != "Error" {
		t.Errorf("StatusLabel = %q, want %q", errState.StatusLabel, "Error")
	}
	if errState.ErrorText != "timeout" {
		t.Errorf("ErrorText = %q, want %q", errState.ErrorText, "timeout")
	}
	if errState.Mode != modes.Full {
		t.Errorf("Mode should be preserved in error state, got %q", errState.Mode)
	}
	if errState.Protocol != ProtocolAuto {
		t.Errorf("Protocol should be preserved in error state, got %q", errState.Protocol)
	}

	// WithConnection from error state clears ErrorText.
	vs3 := vs2.WithConnection(ConnectionDisconnected)
	if vs3.ErrorText != "" {
		t.Errorf("ErrorText should be cleared after WithConnection from error, got %q", vs3.ErrorText)
	}
	if vs3.Connection != ConnectionDisconnected {
		t.Errorf("Connection should be disconnected after WithConnection from error, got %q", vs3.Connection)
	}
}

func TestValidProtocol(t *testing.T) {
	tests := []struct {
		p     ProtocolSelection
		valid bool
	}{
		{ProtocolAuto, true},
		{ProtocolHysteria2, true},
		{ProtocolXray, true},
		{ProtocolAmneziaWG, true},
		{ProtocolSelection("invalid"), false},
		{ProtocolSelection(""), false},
		{ProtocolSelection("hysteria3"), false},
	}
	for _, tt := range tests {
		got := validProtocol(tt.p)
		if got != tt.valid {
			t.Errorf("validProtocol(%q) = %v, want %v", tt.p, got, tt.valid)
		}
	}
}

func TestModeLabel(t *testing.T) {
	tests := []struct {
		mode modes.Mode
		want string
	}{
		{modes.Normal, "Normal proxy"},
		{modes.Cheap, "Cheap proxy"},
		{modes.Full, "Full TUN"},
		{modes.Super, "Normal proxy"},
	}
	for _, tt := range tests {
		if got := modeLabel(tt.mode); got != tt.want {
			t.Errorf("modeLabel(%q) = %q, want %q", tt.mode, got, tt.want)
		}
	}
}

func TestPrimaryActionLabel(t *testing.T) {
	tests := []struct {
		name string
		cs   ConnectionState
		want string
	}{
		{"disconnected", ConnectionDisconnected, "Connect"},
		{"connecting", ConnectionConnecting, "Connecting"},
		{"connected", ConnectionConnected, "Disconnect"},
		{"error", ConnectionError, "Connect"},
	}
	for _, tt := range tests {
		got := primaryActionLabel(tt.cs)
		if got != tt.want {
			t.Errorf("primaryActionLabel(%q) = %q, want %q",
				tt.cs, got, tt.want)
		}
	}
}

func TestWithActiveTransport(t *testing.T) {
	vs := InitialViewState().WithActiveTransport("hysteria2")
	if vs.ActiveTransport != "hysteria2" {
		t.Errorf("ActiveTransport = %q, want %q", vs.ActiveTransport, "hysteria2")
	}
	// Other fields should be unchanged.
	if vs.Connection != ConnectionDisconnected {
		t.Errorf("Connection should remain disconnected, got %q", vs.Connection)
	}
}

func TestConnectedSameModeNoRestart(t *testing.T) {
	vs := InitialViewState().WithConnection(ConnectionConnected)
	if vs.RestartRequired {
		t.Error("After connecting, RestartRequired should be false")
	}

	// Setting the same mode while connected should NOT require restart.
	vs2 := vs.WithMode(string(modes.Normal))
	if vs2.RestartRequired {
		t.Error("WithMode with same mode while connected should not set RestartRequired")
	}
	// ModeLabel should remain correct.
	if vs2.ModeLabel != "Normal proxy" {
		t.Errorf("ModeLabel = %q, want %q", vs2.ModeLabel, "Normal proxy")
	}

	// A different mode still requires restart.
	vs3 := vs.WithMode("full")
	if !vs3.RestartRequired {
		t.Error("WithMode with different mode while connected should set RestartRequired")
	}
}

func TestConnectedSameProtocolNoRestart(t *testing.T) {
	vs := InitialViewState().WithConnection(ConnectionConnected)
	if vs.RestartRequired {
		t.Error("After connecting, RestartRequired should be false")
	}

	// Setting the same protocol while connected should NOT require restart.
	vs2 := vs.WithProtocol(ProtocolAuto)
	if vs2.RestartRequired {
		t.Error("WithProtocol with same protocol while connected should not set RestartRequired")
	}
	// ProtocolLabel should remain correct.
	if vs2.ProtocolLabel != "Auto" {
		t.Errorf("ProtocolLabel = %q, want %q", vs2.ProtocolLabel, "Auto")
	}

	// A different protocol still requires restart.
	vs3 := vs.WithProtocol(ProtocolHysteria2)
	if !vs3.RestartRequired {
		t.Error("WithProtocol with different protocol while connected should set RestartRequired")
	}
}

func TestRestartRequiredOnDisconnectedShowsRestartLabel(t *testing.T) {
	// Direct struct to simulate pending restart while disconnected.
	vs := ViewState{Connection: ConnectionDisconnected, RestartRequired: true}
	vs = refreshLabels(vs)
	if vs.PrimaryAction != "Restart to apply" {
		t.Errorf("RestartRequired on disconnected should show 'Restart to apply', got %q", vs.PrimaryAction)
	}
	if vs.StatusLabel != "Disconnected" {
		t.Errorf("StatusLabel should be Disconnected, got %q", vs.StatusLabel)
	}
}
