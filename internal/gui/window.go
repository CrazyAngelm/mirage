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
	"fyne.io/fyne/v2/layout"
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

	// icons mapping
	connIcon *widget.Icon
}

// NewDesktopGUI creates a Fyne window wired to the given controller.
func NewDesktopGUI(controller *Controller) *DesktopGUI {
	fyneApp := app.NewWithID("mirage.client")
	fyneApp.Settings().SetTheme(theme.DarkTheme())
	w := fyneApp.NewWindow("Mirage Client")

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
	g.window.Resize(fyne.NewSize(700, 480))
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
	g.statusLabel.Alignment = fyne.TextAlignCenter

	g.modeLabel = widget.NewLabel("Normal proxy")
	g.protocolLabel = widget.NewLabel("Auto")
	g.transportLabel = widget.NewLabel("unknown")

	g.errorLabel = widget.NewLabel("")
	g.errorLabel.Wrapping = fyne.TextWrapWord
	g.errorLabel.Alignment = fyne.TextAlignCenter

	g.actionButton = widget.NewButtonWithIcon("Connect", theme.CheckButtonCheckedIcon(), func() {
		go func() { _ = g.controller.PrimaryAction() }()
	})
	g.actionButton.Importance = widget.HighImportance

	g.connIcon = widget.NewIcon(theme.CancelIcon())

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

	setProxy := widget.NewButtonWithIcon("Set system proxy", theme.SettingsIcon(), func() {
		go func() { _ = g.controller.SetSystemProxy() }()
	})
	clearProxy := widget.NewButtonWithIcon("Clear system proxy", theme.DeleteIcon(), func() {
		go func() { _ = g.controller.ClearSystemProxy() }()
	})
		importProfile := widget.NewButtonWithIcon("Add Server", theme.ContentAddIcon(), func() {
		entry := widget.NewEntry()
		entry.SetPlaceHolder("mirage://...")
		
		fileBtn := widget.NewButtonWithIcon("Browse File", theme.FolderOpenIcon(), func() {
			dialog.ShowFileOpen(func(reader fyne.URIReadCloser, err error) {
				if err != nil || reader == nil {
					return
				}
				path := reader.URI().Path()
				_ = reader.Close()
				go func() { _ = g.controller.ImportProfile(path) }()
			}, g.window)
		})
		
		content := container.NewVBox(
			widget.NewLabel("Paste mirage:// link:"),
			entry,
			widget.NewLabel("Or import from file:"),
			fileBtn,
		)
		
		dlg := dialog.NewCustomConfirm("Add Server", "Import Link", "Cancel", content, func(confirm bool) {
			if confirm && entry.Text != "" {
				go func() { _ = g.controller.ImportProfile(entry.Text) }()
			}
		}, g.window)
		dlg.Resize(fyne.NewSize(400, 200))
		dlg.Show()
	})
	importProfile.Importance = widget.HighImportance

	openLogs := widget.NewButtonWithIcon("Open logs folder", theme.FolderOpenIcon(), func() {
		go func() { _ = g.controller.OpenLogsFolder() }()
	})

	g.doctorOutput = widget.NewMultiLineEntry()
	g.doctorOutput.SetPlaceHolder("Diagnostics output appears here")
	runDoctor := widget.NewButtonWithIcon("Run doctor", theme.SearchIcon(), func() {
		go func() {
			out, err := g.controller.RunDoctor()
			if err != nil {
				out = out + "\n" + err.Error()
			}
			fyne.Do(func() { g.doctorOutput.SetText(out) })
		}()
	})
	runDoctor.Importance = widget.HighImportance

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

	// --- Dashboard Tab ---
	statusBox := container.NewCenter(container.NewVBox(
		g.connIcon,
		g.statusLabel,
	))

	actionBox := container.NewPadded(container.NewVBox(
		layout.NewSpacer(),
		g.actionButton,
		g.errorLabel,
		layout.NewSpacer(),
	))

	infoForm := widget.NewForm(
		widget.NewFormItem("Server:", g.transportLabel),
		widget.NewFormItem("Mode:", g.modeLabel),
		widget.NewFormItem("Protocol:", g.protocolLabel),
	)

	transportsCard := widget.NewCard("Transports Health", "", container.NewVBox(
		g.hysteriaStatus,
		g.xrayStatus,
		g.amneziaStatus,
	))

	dashboardContent := container.NewBorder(
		container.NewPadded(statusBox),
		container.NewPadded(actionBox),
		nil,
		nil,
		container.NewVBox(layout.NewSpacer(), container.NewCenter(infoForm), layout.NewSpacer(), transportsCard, layout.NewSpacer()),
	)

	// --- Servers Tab ---
	serversContent := container.NewBorder(
		container.NewPadded(importProfile),
		nil, nil, nil,
		container.NewVScroll(container.NewPadded(g.profileList)),
	)

	// --- Settings Tab ---
	settingsForm := widget.NewForm(
		widget.NewFormItem("Mode", g.modeSelect),
		widget.NewFormItem("Protocol", g.protocolSelect),
		widget.NewFormItem("Camouflage", g.camouflageEntry),
	)

	securityBox := container.NewVBox(
		widget.NewLabelWithStyle("Security & Privacy", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		g.reconnectLabel,
		widget.NewLabel("• DNS through tunnel"),
		widget.NewLabel("• IPv6 leak protection"),
		g.ruDirectCheck,
		layout.NewSpacer(),
		container.NewHBox(setProxy, clearProxy),
	)

	settingsContent := container.NewVBox(
		widget.NewCard("Routing", "Tunnel behavior", settingsForm),
		widget.NewCard("Security", "Leak protection", securityBox),
	)

	// --- Diagnostics Tab ---
	diagnosticsContent := container.NewBorder(
		container.NewPadded(container.NewHBox(runDoctor, openLogs)),
		nil, nil, nil,
		container.NewPadded(g.doctorOutput),
	)

	tabs := container.NewAppTabs(
		container.NewTabItemWithIcon("Dashboard", theme.HomeIcon(), dashboardContent),
		container.NewTabItemWithIcon("Servers", theme.ListIcon(), serversContent),
		container.NewTabItemWithIcon("Settings", theme.SettingsIcon(), container.NewPadded(container.NewVScroll(settingsContent))),
		container.NewTabItemWithIcon("Diagnostics", theme.InfoIcon(), diagnosticsContent),
	)
	tabs.SetTabLocation(container.TabLocationLeading)

	g.window.SetContent(tabs)
	g.window.SetCloseIntercept(func() { g.window.Hide() })
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

	if state.Connection == ConnectionConnected {
		g.connIcon.SetResource(theme.ConfirmIcon())
		g.actionButton.SetIcon(theme.CancelIcon())
		g.actionButton.Importance = widget.DangerImportance
	} else if state.Connection == ConnectionConnecting {
		g.connIcon.SetResource(theme.ViewRefreshIcon())
		g.actionButton.SetIcon(theme.ViewRefreshIcon())
		g.actionButton.Importance = widget.WarningImportance
	} else {
		g.connIcon.SetResource(theme.ErrorIcon())
		g.actionButton.SetIcon(theme.ConfirmIcon())
		g.actionButton.Importance = widget.HighImportance
	}

	activeLabel := state.ActiveTransport
	if state.ActiveProfileName != "" {
		activeLabel = state.ActiveProfileName + " • " + activeLabel
	}
	g.transportLabel.SetText(activeLabel)

	if state.ErrorText != "" {
		g.errorLabel.SetText(state.ErrorText)
		g.errorLabel.Show()
	} else {
		g.errorLabel.SetText("")
		g.errorLabel.Hide()
	}

	g.actionButton.SetText(state.PrimaryAction)
	if g.camouflageEntry != nil {
		g.camouflageEntry.SetText(state.CamouflageSite)
	}
	if g.reconnectLabel != nil {
		if state.ReconnectRequired || state.RestartRequired {
			g.reconnectLabel.SetText("Reconnect required")
			g.reconnectLabel.TextStyle = fyne.TextStyle{Bold: true}
		} else {
			g.reconnectLabel.SetText("DNS OK • IPv6 blocked")
			g.reconnectLabel.TextStyle = fyne.TextStyle{}
		}
	}
	if g.profileList != nil {
		g.profileList.Objects = nil
		if len(state.Profiles) == 0 {
			g.profileList.Add(widget.NewLabel("No servers imported"))
		} else {
			for _, profile := range state.Profiles {
				prefix := "  "
				if profile.Active {
					prefix = "✓ "
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

				btnText := fmt.Sprintf("%s%s (%s%s)", prefix, profile.Name, status, latency)

				// Make sure we capture id for the closure
				profileID := id
				row := widget.NewButton(btnText, func() {
					go func() { _ = g.controller.SetActiveProfile(profileID) }()
				})
				if profile.Active {
					row.Importance = widget.HighImportance
				}
				
				deleteBtn := widget.NewButtonWithIcon("", theme.DeleteIcon(), func() {
					go func() { _ = g.controller.RemoveProfile(profileID) }()
				})
				
				rowContainer := container.NewBorder(nil, nil, nil, deleteBtn, row)

				g.profileList.Add(rowContainer)
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




func (g *DesktopGUI) Quit() {
	if g.fyneApp != nil {
		g.fyneApp.Quit()
	}
}

