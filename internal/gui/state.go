package gui

import "mirage/internal/modes"

// ConnectionState represents the VPN connection status.
type ConnectionState string

const (
	ConnectionDisconnected ConnectionState = "disconnected"
	ConnectionConnecting   ConnectionState = "connecting"
	ConnectionConnected    ConnectionState = "connected"
	ConnectionError        ConnectionState = "error"
)

// ProtocolSelection represents the selected transport protocol.
type ProtocolSelection string

const (
	ProtocolAuto      ProtocolSelection = "auto"
	ProtocolHysteria2 ProtocolSelection = "hysteria2"
	ProtocolXray      ProtocolSelection = "xray_reality_xhttp"
	ProtocolAmneziaWG ProtocolSelection = "amneziawg"
)

type sidebarTarget string

const (
	sidebarDashboard   sidebarTarget = "dashboard"
	sidebarServers     sidebarTarget = "servers"
	sidebarRouting     sidebarTarget = "routing"
	sidebarDiagnostics sidebarTarget = "diagnostics"
)

type sidebarItem struct {
	Label  string
	Target sidebarTarget
}

func sidebarItems() []sidebarItem {
	return []sidebarItem{
		{Label: "Dashboard", Target: sidebarDashboard},
		{Label: "Servers", Target: sidebarServers},
		{Label: "Routing", Target: sidebarRouting},
		{Label: "Diagnostics", Target: sidebarDiagnostics},
	}
}

type ProfileState struct {
	ID        string
	Name      string
	Host      string
	Status    string
	LatencyMS int
	Active    bool
}

type ViewState struct {
	Connection         ConnectionState
	Mode               modes.Mode
	Protocol           ProtocolSelection
	RussianDirect      bool
	StatusLabel        string
	ModeLabel          string
	ProtocolLabel      string
	RussianDirectLabel string
	PrimaryAction      string
	ActiveTransport    string
	Profiles           []ProfileState
	ActiveProfileID    string
	ActiveProfileName  string
	CamouflageSite     string
	ReconnectRequired  bool
	RestartRequired    bool
	ErrorText          string
}

// InitialViewState returns the default disconnected state.
func InitialViewState() ViewState {
	vs := ViewState{
		Connection:    ConnectionDisconnected,
		Mode:          modes.Normal,
		Protocol:      ProtocolAuto,
		RussianDirect: false,
	}
	return refreshLabels(vs)
}

func (vs ViewState) WithConnection(cs ConnectionState) ViewState {
	vs.Connection = cs
	if cs == ConnectionConnected {
		vs.RestartRequired = false
	}
	if cs != ConnectionError {
		vs.ErrorText = ""
	}
	return refreshLabels(vs)
}

func (vs ViewState) WithMode(mode string) ViewState {
	nextMode := modes.Normalize(mode)
	if vs.Connection == ConnectionConnected && nextMode != vs.Mode {
		vs.RestartRequired = true
	}
	vs.Mode = nextMode
	return refreshLabels(vs)
}

func (vs ViewState) WithProtocol(p ProtocolSelection) ViewState {
	if !validProtocol(p) {
		p = ProtocolAuto
	}
	if vs.Connection == ConnectionConnected && p != vs.Protocol {
		vs.RestartRequired = true
	}
	vs.Protocol = p
	return refreshLabels(vs)
}

func (vs ViewState) WithRussianDirect(enabled bool) ViewState {
	if vs.Connection == ConnectionConnected && enabled != vs.RussianDirect {
		vs.RestartRequired = true
	}
	vs.RussianDirect = enabled
	return refreshLabels(vs)
}

// WithActiveTransport returns a copy with the active transport name set.
func (vs ViewState) WithActiveProfile(id, name string) ViewState {
	if vs.Connection == ConnectionConnected && id != vs.ActiveProfileID && vs.ActiveProfileID != "" {
		vs.RestartRequired = true
	}
	vs.ActiveProfileID = id
	vs.ActiveProfileName = name
	return refreshLabels(vs)
}

func (vs ViewState) WithActiveTransport(t string) ViewState {
	vs.ActiveTransport = t
	return refreshLabels(vs)
}

// WithError returns a copy with the connection set to Error and the given
// error text.
func (vs ViewState) WithError(err string) ViewState {
	vs.Connection = ConnectionError
	vs.ErrorText = err
	return refreshLabels(vs)
}

// validProtocol returns true when p is one of the defined protocol constants.
func validProtocol(p ProtocolSelection) bool {
	switch p {
	case ProtocolAuto, ProtocolHysteria2, ProtocolXray, ProtocolAmneziaWG:
		return true
	default:
		return false
	}
}

// refreshLabels recalculates StatusLabel, ModeLabel, ProtocolLabel, and
// PrimaryAction from the current state. If RestartRequired is true,
// PrimaryAction is unconditionally "Restart to apply".
func refreshLabels(vs ViewState) ViewState {
	vs.StatusLabel = connectionLabel(vs.Connection)
	vs.ModeLabel = modeLabel(vs.Mode)
	vs.ProtocolLabel = protocolLabel(vs.Protocol)
	vs.RussianDirectLabel = russianDirectLabel(vs.RussianDirect)
	if vs.RestartRequired {
		vs.PrimaryAction = "Restart to apply"
	} else {
		vs.PrimaryAction = primaryActionLabel(vs.Connection)
	}
	return vs
}

func connectionLabel(cs ConnectionState) string {
	switch cs {
	case ConnectionDisconnected:
		return "Disconnected"
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

func modeLabel(m modes.Mode) string {
	switch m {
	case modes.Cheap:
		return "Cheap proxy"
	case modes.Full:
		return "Full TUN"
	default:
		return "Normal proxy"
	}
}

func protocolLabel(p ProtocolSelection) string {
	switch p {
	case ProtocolAuto:
		return "Auto"
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

func russianDirectLabel(enabled bool) string {
	if enabled {
		return "RU sites direct"
	}
	return "RU sites via VPN"
}

func primaryActionLabel(cs ConnectionState) string {
	switch cs {
	case ConnectionDisconnected:
		return "Connect"
	case ConnectionConnecting:
		return "Connecting"
	case ConnectionConnected:
		return "Disconnect"
	case ConnectionError:
		return "Connect"
	default:
		return "Connect"
	}
}

