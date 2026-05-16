package gui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"mirage/internal/app"
	"mirage/internal/bundle"
	"mirage/internal/modes"
	"mirage/internal/platform"
	"mirage/internal/sysproxy"

	"github.com/getlantern/systray"
	"github.com/sqweek/dialog"
)

var (
	supervisor     *Supervisor
	supervisorOnce sync.Once
	mConnectRef    *systray.MenuItem
	mDisconnectRef *systray.MenuItem

	desktopMu        sync.RWMutex
	desktopWindow    *DesktopGUI
	trayControllerMu sync.RWMutex
	trayController   *Controller
)

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

func setTrayController(controller *Controller) {
	trayControllerMu.Lock()
	trayController = controller
	trayControllerMu.Unlock()
}

func getTrayController() *Controller {
	trayControllerMu.RLock()
	controller := trayController
	trayControllerMu.RUnlock()
	return controller
}

func trayConnectionTitles(running bool) (string, string) {
	if running {
		return "Connected", "Disconnect"
	}
	return "Connect", "Disconnected"
}

func trayModeForMenu(name string) modes.Mode {
	switch name {
	case "cheap":
		return modes.Cheap
	case "full":
		return modes.Full
	default:
		return modes.Normal
	}
}

func getSupervisor() *Supervisor {
	supervisorOnce.Do(func() {
		supervisor = &Supervisor{}
	})
	return supervisor
}

// Run starts the system tray GUI and blocks until Exit is selected.
// When the desktop window is available, it delegates to RunDesktop so both
// the Fyne window and the systray run together.
func Run() {
	RunDesktop()
}

// RunTray starts the systray event loop with the shared controller.
// It keeps the tray in sync with the window state.
func RunTray(controller *Controller) {
	setTrayController(controller)
	systray.Run(onReady, onExit)
}

func onReady() {
	systray.SetTitle("Mirage")
	systray.SetTooltip("Mirage VPN")
	// No custom icon for MVP; systray shows default gear icon

	mShow := systray.AddMenuItem("Show window", "Open Mirage main window")
	mConnect := systray.AddMenuItem("Connect", "Start VPN")
	mDisconnect := systray.AddMenuItem("Disconnect", "Stop VPN")
	mConnectRef = mConnect
	mDisconnectRef = mDisconnect
	systray.AddSeparator()
	mMode := systray.AddMenuItem("Mode", "Change connection mode")
	mNormal := mMode.AddSubMenuItem("Normal (auto)", "All transports, auto-select best")
	mCheap := mMode.AddSubMenuItem("Cheap (single)", "Best transport only, SOCKS proxy")
	mFull := mMode.AddSubMenuItem("Full (TUN)", "All transports, system-wide VPN")
	systray.AddSeparator()
	mTransports := systray.AddMenuItem("Transports", "Transport health status")
	mHysteria := mTransports.AddSubMenuItem("hysteria2: unknown", "Hysteria2 status")
	mXray := mTransports.AddSubMenuItem("xray: unknown", "Xray status")
	mAmnezia := mTransports.AddSubMenuItem("amneziawg: unknown", "AmneziaWG status")
	systray.AddSeparator()
	mImport := systray.AddMenuItem("Import Link...", "Import mirage:// link from file")
	mStatus := systray.AddMenuItem("Show Status", "Show current status")
	systray.AddSeparator()
	mExit := systray.AddMenuItem("Exit", "Quit Mirage")

	// Initial state
	updateModeChecks(mNormal, mCheap, mFull)
	refreshTrayStatus(mConnect, mDisconnect)

	// Background transport status updater
	go transportStatusUpdater(mHysteria, mXray, mAmnezia)

	go func() {
		for {
			select {
			case <-mShow.ClickedCh:
				showDesktopWindow()
			case <-mConnect.ClickedCh:
				go doControllerConnect()
			case <-mDisconnect.ClickedCh:
				go doControllerDisconnect()
			case <-mNormal.ClickedCh:
				go doControllerSetMode(trayModeForMenu("normal"), mNormal, mCheap, mFull)
			case <-mCheap.ClickedCh:
				go doControllerSetMode(trayModeForMenu("cheap"), mNormal, mCheap, mFull)
			case <-mFull.ClickedCh:
				go doControllerSetMode(trayModeForMenu("full"), mNormal, mCheap, mFull)
			case <-mImport.ClickedCh:
				go doImportWithController()
			case <-mStatus.ClickedCh:
				go doStatus()
			case <-mExit.ClickedCh:
				systray.Quit()
				return
			}
		}
	}()
}

func onExit() {
	controller := getTrayController()
	if controller != nil {
		_ = controller.Disconnect()
		return
	}
	doDisconnect()
}

func doControllerConnect() {
	controller := getTrayController()
	if controller == nil {
		doConnect()
		return
	}
	if err := controller.Connect(); err != nil {
		dialog.Message("Connect failed: %v", err).Title("Mirage").Error()
		return
	}
	refreshTrayStatus(mConnectRef, mDisconnectRef)
	updateTooltip()
}

func doControllerDisconnect() {
	controller := getTrayController()
	if controller == nil {
		doDisconnect()
		return
	}
	if err := controller.Disconnect(); err != nil {
		dialog.Message("Disconnect failed: %v", err).Title("Mirage").Error()
		return
	}
	refreshTrayStatus(mConnectRef, mDisconnectRef)
	systray.SetTooltip("Mirage: Disconnected")
}

func doControllerSetMode(mode modes.Mode, mNormal, mCheap, mFull *systray.MenuItem) {
	controller := getTrayController()
	if controller == nil {
		doSetMode(mode, mNormal, mCheap, mFull)
		return
	}
	if err := controller.SetMode(mode); err != nil {
		dialog.Message("Failed to set mode: %v", err).Title("Mirage").Error()
		return
	}
	updateModeChecks(mNormal, mCheap, mFull)
	refreshTrayStatus(mConnectRef, mDisconnectRef)
	systray.SetTooltip(fmt.Sprintf("Mirage: mode set to %s", mode))
}

func doImportWithController() {
	f, err := dialog.File().Title("Select mirage link file").Load()
	if err != nil {
		return
	}
	controller := getTrayController()
	if controller == nil {
		doImport()
		return
	}
	if err := controller.ImportProfile(f); err != nil {
		dialog.Message("Import failed: %v", err).Title("Mirage").Error()
		return
	}
	refreshTrayStatus(mConnectRef, mDisconnectRef)
}

func connectSuccessMessage(mode modes.Mode, proxyEnabled bool) string {
	if proxyEnabled {
		return fmt.Sprintf("Connected. Mode: %s, system proxy enabled.", mode)
	}
	return fmt.Sprintf("Connected. Mode: %s.", mode)
}

func doConnect() {
	s := getSupervisor()
	if s.IsRunning() {
		dialog.Message("Already connected.").Title("Mirage").Info()
		return
	}

	base := platform.ClientBaseDir()
	cfg := filepath.Join(base, "configs", "sing-box.json")
	if _, err := os.Stat(cfg); err != nil {
		dialog.Message("No config found. Import a profile first.").Title("Mirage").Error()
		return
	}

	mode := readClientMode(base)
	if modes.IsTun(mode) && !sysproxy.IsAdmin() {
		dialog.Message("TUN mode requires Administrator rights.\nPlease restart Mirage as Administrator.").Title("Mirage").Error()
		return
	}

	singBox := filepath.Join(base, "bin", "sing-box.exe")
	if _, err := os.Stat(singBox); err != nil {
		dialog.Message("sing-box.exe not found in bin directory.").Title("Mirage").Error()
		return
	}

	if err := s.Start(singBox, cfg, mode); err != nil {
		dialog.Message("Failed to start: %v", err).Title("Mirage").Error()
		return
	}

	proxyEnabled := false
	if !modes.IsTun(mode) {
		if err := sysproxy.Set("127.0.0.1:2080"); err != nil {
			// Non-fatal: proxy still works if manually configured
			fmt.Fprintf(os.Stderr, "sysproxy set warning: %v\n", err)
		} else {
			proxyEnabled = true
		}
	}

	refreshTrayStatus(mConnectRef, mDisconnectRef)
	updateTooltip()
	dialog.Message("%s", connectSuccessMessage(mode, proxyEnabled)).Title("Mirage").Info()
}

func doDisconnect() {
	s := getSupervisor()
	s.Stop()
	var out strings.Builder
	if err := app.DisconnectClient(&out); err != nil {
		dialog.Message("Disconnect failed: %v", err).Title("Mirage").Error()
		return
	}

	refreshTrayStatus(mConnectRef, mDisconnectRef)
	systray.SetTooltip("Mirage: Disconnected")
}

func doSetMode(mode modes.Mode, mNormal, mCheap, mFull *systray.MenuItem) {
	base := platform.ClientBaseDir()
	link, err := readProfileLink(base)
	if err != nil {
		dialog.Message("No imported profile.").Title("Mirage").Error()
		return
	}
	b, err := bundle.Decode(link)
	if err != nil {
		dialog.Message("Invalid profile.").Title("Mirage").Error()
		return
	}
	if err := app.SetClientModeGUI(base, string(mode), b); err != nil {
		dialog.Message("Failed to set mode: %v", err).Title("Mirage").Error()
		return
	}
	updateModeChecks(mNormal, mCheap, mFull)
	refreshTrayStatus(mConnectRef, mDisconnectRef)
	systray.SetTooltip(fmt.Sprintf("Mirage: mode set to %s", mode))
}

func doImport() {
	f, err := dialog.File().Title("Select mirage link file").Load()
	if err != nil {
		return // cancelled
	}
	var out strings.Builder
	if err := app.ImportProfileGUI(f, &out); err != nil {
		dialog.Message("Import failed: %v", err).Title("Mirage").Error()
		return
	}
	dialog.Message("%s", out.String()).Title("Mirage").Info()
	refreshTrayStatus(mConnectRef, mDisconnectRef)
}

func doStatus() {
	var out strings.Builder
	_ = app.ClientStatusGUI(&out)
	mode := readClientMode(platform.ClientBaseDir())
	s := getSupervisor()
	status := fmt.Sprintf("%s\nMode: %s\nConnected: %v", out.String(), mode, s.IsRunning())
	if best, ok := s.BestTransport(); ok {
		status += fmt.Sprintf("\nBest: %s (%dms)", best.Name, best.LatencyMS)
	}
	dialog.Message("%s", status).Title("Mirage Status").Info()
}

func updateModeChecks(mNormal, mCheap, mFull *systray.MenuItem) {
	mode := readClientMode(platform.ClientBaseDir())
	// Uncheck all
	mNormal.Uncheck()
	mCheap.Uncheck()
	mFull.Uncheck()
	switch mode {
	case modes.Normal:
		mNormal.Check()
	case modes.Cheap:
		mCheap.Check()
	case modes.Full:
		mFull.Check()
	}
}

func refreshTrayStatus(mConnect, mDisconnect *systray.MenuItem) {
	running := getSupervisor().IsRunning()
	if controller := getTrayController(); controller != nil {
		running = controller.State().Connection == ConnectionConnected
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

func updateTooltip() {
	s := getSupervisor()
	if !s.IsRunning() {
		systray.SetTooltip("Mirage: Disconnected")
		return
	}
	if best, ok := s.BestTransport(); ok {
		systray.SetTooltip(fmt.Sprintf("Mirage: %s (%dms)", best.Name, best.LatencyMS))
	} else {
		systray.SetTooltip("Mirage: Connected")
	}
}

func transportStatusUpdater(mHysteria, mXray, mAmnezia *systray.MenuItem) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		s := getSupervisor()
		refreshTrayStatus(mConnectRef, mDisconnectRef)
		if !s.IsRunning() {
			mHysteria.SetTitle("hysteria2: disconnected")
			mXray.SetTitle("xray: disconnected")
			mAmnezia.SetTitle("amneziawg: disconnected")
			continue
		}
		statuses := s.Statuses()
		if st, ok := statuses["hysteria2"]; ok {
			mHysteria.SetTitle(st.Label())
		} else {
			mHysteria.SetTitle("hysteria2: not configured")
		}
		if st, ok := statuses["xray_reality_xhttp"]; ok {
			mXray.SetTitle(st.Label())
		} else {
			mXray.SetTitle("xray: not configured")
		}
		if st, ok := statuses["amneziawg"]; ok {
			mAmnezia.SetTitle(st.Label())
		} else {
			mAmnezia.SetTitle("amneziawg: not configured")
		}
		updateTooltip()
	}
}

func readProfileLink(base string) (string, error) {
	return app.ReadProfileLink(base)
}

func readClientMode(base string) modes.Mode {
	payload, err := os.ReadFile(filepath.Join(base, "state", "mode.txt"))
	if err != nil {
		return modes.Normal
	}
	return modes.Normalize(strings.TrimSpace(string(payload)))
}
