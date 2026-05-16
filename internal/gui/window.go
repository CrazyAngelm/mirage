//go:build cgo

package gui

import (
	"fmt"
	"time"

	"mirage/internal/modes"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// DesktopGUI owns the Fyne main window and binds its widgets to the
// shared Controller.
type DesktopGUI struct {
	fyneApp    fyne.App
	window     fyne.Window
	controller *Controller

	statusLabel     *widget.Label
	modeLabel       *widget.Label
	protocolLabel   *widget.Label
	transportLabel  *widget.Label
	errorLabel      *widget.Label
	actionButton    *widget.Button
	modeSelect      *widget.Select
	protocolSelect  *widget.Select
	ruDirectCheck   *widget.Check
	doctorOutput    *widget.Entry
	profileList     *fyne.Container
	camouflageEntry *widget.Entry
	reconnectLabel  *widget.Label
	applyingState   bool

	hysteriaStatus *widget.Label
	xrayStatus     *widget.Label
	amneziaStatus  *widget.Label
}

// NewDesktopGUI creates a Fyne window wired to the given controller.
func NewDesktopGUI(controller *Controller) *DesktopGUI {
	fyneApp := app.NewWithID("mirage.client")
	fyneApp.Settings().SetTheme(theme.DarkTheme())
	w := fyneApp.NewWindow("Mirage")

	gui := &DesktopGUI{fyneApp: fyneApp, window: w, controller: controller}
	gui.build()

	controller.SetOnChange(func(state ViewState) {
		fyne.Do(func() { gui.applyState(state) })
	})
	controller.SetOnError(func(message string) {
		fyne.Do(func() { dialog.ShowError(fmt.Errorf("%s", message), w) })
	})
	controller.SetOnMessage(func(message string) {
		fyne.Do(func() { dialog.ShowInformation("Mirage", message, w) })
	})

	gui.applyState(controller.State())
	return gui
}

// ShowAndRun makes the window visible and starts the Fyne event loop.
// It blocks until the application exits.
func (g *DesktopGUI) ShowAndRun() {
	g.window.Resize(fyne.NewSize(860, 560))
	g.window.SetFixedSize(true)
	g.window.Show()
	go g.refreshLoop()
	g.fyneApp.Run()
}

// Show brings the window back after it has been hidden via close intercept.
func (g *DesktopGUI) Show() {
	g.window.Show()
	g.window.RequestFocus()
}

// --------------------------------------------------------------------------
// build helpers
// --------------------------------------------------------------------------

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

	g.modeSelect = widget.NewSelect([]string{"Normal proxy", "Cheap proxy", "Full TUN"}, func(value string) {
		if g.applyingState {
			return
		}
		switch value {
		case "Cheap proxy":
			go func() { _ = g.controller.SetMode(modes.Cheap) }()
		case "Full TUN":
			go func() { _ = g.controller.SetMode(modes.Full) }()
		default:
			go func() { _ = g.controller.SetMode(modes.Normal) }()
		}
	})

	g.protocolSelect = widget.NewSelect([]string{"Auto", "Hysteria2", "Xray", "AmneziaWG"}, func(value string) {
		if g.applyingState {
			return
		}
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

	g.ruDirectCheck = widget.NewCheck("RU sites direct", func(enabled bool) {
		if g.applyingState {
			return
		}
		go func() { _ = g.controller.SetRussianDirect(enabled) }()
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
			fyne.Do(func() { g.doctorOutput.SetText(out) })
		}()
	})

	g.hysteriaStatus = widget.NewLabel("hysteria2: unknown")
	g.xrayStatus = widget.NewLabel("xray: unknown")
	g.amneziaStatus = widget.NewLabel("amneziawg: unknown")
	g.profileList = container.NewVBox(widget.NewLabel("No servers imported"))
	g.reconnectLabel = widget.NewLabel("DNS OK • IPv6 blocked")
	g.camouflageEntry = widget.NewEntry()
	g.camouflageEntry.SetPlaceHolder("www.microsoft.com")
	g.camouflageEntry.OnSubmitted = func(value string) {
		if g.applyingState {
			return
		}
		go func() { _ = g.controller.SetCamouflageSite(value) }()
	}

	activeCard := widget.NewCard("Active server", "Encrypted tunnel", container.NewVBox(
		g.statusLabel,
		widget.NewForm(
			widget.NewFormItem("Server", g.transportLabel),
			widget.NewFormItem("Mode", g.modeLabel),
			widget.NewFormItem("Protocol", g.protocolLabel),
		),
		g.actionButton,
		g.errorLabel,
	))
	routingCard := widget.NewCard("Routing", "Manual control", container.NewVBox(
		widget.NewForm(
			widget.NewFormItem("Mode", g.modeSelect),
			widget.NewFormItem("Protocol", g.protocolSelect),
			widget.NewFormItem("Camouflage site", g.camouflageEntry),
		),
		g.ruDirectCheck,
		g.reconnectLabel,
	))
	serversCard := widget.NewCard("Servers", "Manual active server", g.profileList)
	securityCard := widget.NewCard("Security", "Leak protection", container.NewVBox(
		widget.NewLabel("DNS through tunnel"),
		widget.NewLabel("IPv6 leak protection"),
		container.NewHBox(setProxy, clearProxy),
	))
	utilityCard := widget.NewCard("Diagnostics", "Doctor and logs", container.NewVBox(
		container.NewHBox(importProfile, runDoctor, openLogs),
		g.doctorOutput,
	))

	sidebar := container.NewVBox(
		widget.NewLabelWithStyle("Mirage", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewButton("Dashboard", func() {}),
		widget.NewButton("Servers", func() {}),
		widget.NewButton("Routing", func() {}),
		widget.NewButton("Diagnostics", func() {}),
	)
	main := container.NewVScroll(container.NewVBox(
		activeCard,
		container.NewGridWithColumns(3, serversCard, g.transportCards(), securityCard),
		routingCard,
		utilityCard,
	))
	g.window.SetContent(container.NewBorder(nil, nil, sidebar, nil, main))
	g.window.SetCloseIntercept(func() {
		g.window.Hide()
	})
}

func (g *DesktopGUI) transportCards() fyne.CanvasObject {
	return widget.NewCard("Transports", "", container.NewVBox(
		g.hysteriaStatus,
		g.xrayStatus,
		g.amneziaStatus,
	))
}

// --------------------------------------------------------------------------
// state binding
// --------------------------------------------------------------------------

func (g *DesktopGUI) applyState(state ViewState) {
	g.applyingState = true
	defer func() { g.applyingState = false }()

	g.statusLabel.SetText(state.StatusLabel)
	g.modeLabel.SetText(state.ModeLabel)
	g.protocolLabel.SetText(state.ProtocolLabel)
	g.ruDirectCheck.SetChecked(state.RussianDirect)
	activeLabel := state.ActiveTransport
	if state.ActiveProfileName != "" {
		activeLabel = state.ActiveProfileName + " • " + activeLabel
	}
	g.transportLabel.SetText(activeLabel)
	g.errorLabel.SetText(state.ErrorText)
	g.actionButton.SetText(state.PrimaryAction)
	if g.camouflageEntry != nil {
		g.camouflageEntry.SetText(state.CamouflageSite)
	}
	if g.reconnectLabel != nil {
		if state.ReconnectRequired || state.RestartRequired {
			g.reconnectLabel.SetText("Reconnect required")
		} else {
			g.reconnectLabel.SetText("DNS OK • IPv6 blocked")
		}
	}
	if g.profileList != nil {
		g.profileList.Objects = nil
		if len(state.Profiles) == 0 {
			g.profileList.Add(widget.NewLabel("No servers imported"))
		} else {
			for _, profile := range state.Profiles {
				prefix := "○"
				if profile.Active {
					prefix = "●"
				}
				status := profile.Status
				if status == "" {
					status = "unknown"
				}
				latency := ""
				if profile.LatencyMS > 0 {
					latency = fmt.Sprintf(" • %d ms", profile.LatencyMS)
				}
				id := profile.ID
				row := widget.NewButton(fmt.Sprintf("%s %s — %s%s", prefix, profile.Name, status, latency), func() {
					go func() { _ = g.controller.SetActiveProfile(id) }()
				})
				g.profileList.Add(row)
			}
		}
		g.profileList.Refresh()
	}

	switch state.Mode {
	case modes.Cheap:
		g.modeSelect.SetSelected("Cheap proxy")
	case modes.Full:
		g.modeSelect.SetSelected("Full TUN")
	default:
		g.modeSelect.SetSelected("Normal proxy")
	}
	g.protocolSelect.SetSelected(state.ProtocolLabel)

	if state.Connection == ConnectionConnecting {
		g.actionButton.Disable()
	} else {
		g.actionButton.Enable()
	}
}

// --------------------------------------------------------------------------
// refresh loop
// --------------------------------------------------------------------------

func (g *DesktopGUI) refreshLoop() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for range ticker.C {
		g.controller.RefreshRuntimeState()
		labels := g.controller.TransportLabels()
		fyne.Do(func() {
			g.hysteriaStatus.SetText(labels["hysteria2"])
			g.xrayStatus.SetText(labels["xray_reality_xhttp"])
			g.amneziaStatus.SetText(labels["amneziawg"])
		})
	}
}

// --------------------------------------------------------------------------
// entry point
// --------------------------------------------------------------------------

// RunDesktop creates a shared controller, opens the Fyne main window, and
// starts the systray in the background.
func RunDesktop() {
	s := getSupervisor()
	controller := NewController(s)
	desktop := NewDesktopGUI(controller)
	setDesktopWindow(desktop)
	go RunTray(controller)
	desktop.ShowAndRun()
}
