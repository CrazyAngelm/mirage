# Mirage Main Window GUI Design

**Date:** 2026-05-15

## Goal

Add a normal Windows desktop GUI for Mirage client so users do not depend on tray-only controls. The main window becomes the primary control surface; tray remains as backup quick access.

## Scope

Build a Fyne-based main window inside the existing Go client. Keep current tray behavior, supervisor, profile import, mode generation, health checks, and sidecar management. Do not add a web UI, account system, server manager, or advanced config editor.

## UX model

- Launching `mirage-client.exe` opens the main window.
- Closing the window hides it to tray; VPN keeps running.
- Tray remains available with:
  - Show window
  - Connect / Disconnect
  - Status
  - Exit
- Exit from tray disconnects VPN and quits the app.

## Main window layout

The main screen is a dashboard with these sections:

1. **Connection status**
   - Large state label: `Disconnected`, `Connecting`, `Connected`, or `Error`.
   - Current mode: `Normal proxy` or `Full TUN`.
   - Current protocol selection: `Auto`, `Hysteria2`, `Xray`, or `AmneziaWG`.
   - Best/active transport if known.

2. **Primary action**
   - `Connect` when disconnected.
   - `Disconnect` when connected.
   - `Restart to apply` when user changes mode/protocol while connected.

3. **Mode selector**
   - `Normal proxy`: mixed proxy on `127.0.0.1:2080`, system proxy can be set.
   - `Full TUN`: system-wide TUN mode, requires Administrator rights.

4. **Protocol selector**
   - `Auto`: sing-box `urltest` across all available transports.
   - `Hysteria2`: force Hysteria2.
   - `Xray`: force Xray sidecar via SOCKS `127.0.0.1:2081`.
   - `AmneziaWG`: force WireGuard endpoint.

5. **Proxy controls**
   - `Set system proxy`: sets Windows proxy to `127.0.0.1:2080`.
   - `Clear system proxy`: unsets Windows proxy.
   - Buttons are enabled in normal/proxy mode and disabled or marked not applicable in TUN mode.

6. **Transport cards**
   - One card each for Hysteria2, Xray, AmneziaWG.
   - Show status label, latency, and selected/best marker.
   - Statuses come from the existing health checker.

7. **Utility actions**
   - `Import profile`: open file dialog and import `mirage://` link file.
   - `Run doctor`: run client doctor and show output in a scrollable text area.
   - `Open logs folder`: open `%ProgramData%\Mirage\logs` in Explorer.

## Behavior rules

- Changing mode/protocol while disconnected only regenerates config and updates UI.
- Changing mode/protocol while connected does not restart automatically. UI shows `Restart to apply`.
- `Restart to apply` runs disconnect then connect with the new config.
- Connect failures must show visible error text in the main window and a dialog.
- Successful connect must show visible state change in the main window and update tray menu state within 1 second.
- Tray menu state must be synchronized with real runtime state, including existing `127.0.0.1:2080` listener.
- Disconnect must clean orphan `sing-box.exe` and `xray.exe` Mirage sidecars.

## Architecture

Add a new GUI window layer using Fyne. Keep sidecar/process logic in `internal/gui/Supervisor` and app-level commands in `internal/app`.

Proposed packages/files:

- `internal/gui/window.go`
  - Creates Fyne app/window.
  - Builds dashboard UI.
  - Wires buttons to controller actions.

- `internal/gui/controller.go`
  - Owns UI state transitions.
  - Calls `Supervisor`, `app.SetClientModeGUI`, `app.ImportProfileGUI`, `app.DisconnectClient`, `sysproxy.Set`, `sysproxy.Unset`.
  - Exposes testable methods for mode/protocol changes and connect state.

- `internal/gui/state.go`
  - Defines `ViewState`, connection states, selected mode/protocol, pending restart flag, status labels.

- Existing `internal/gui/tray.go`
  - Keeps tray menu.
  - Adds `Show window` action.
  - Uses same controller/supervisor state as main window.

- Existing `internal/gui/supervisor.go`
  - Continues owning `sing-box` and `xray` processes.
  - Keeps real-state fallback via local port checks.

## Protocol selection implementation

Use existing sing-box config generation with a small extension:

- `Auto` keeps `urltest` with all transports.
- Forced Hysteria2 keeps only Hysteria2 outbound.
- Forced Xray keeps only Xray SOCKS outbound.
- Forced AmneziaWG keeps only WireGuard endpoint and routes final to `amneziawg`.

Selection persists in `%ProgramData%\Mirage\state\transport.txt`. Mode remains in `mode.txt`.

## Error handling

- Missing profile: show `Import profile first` and enable Import button.
- Missing `sing-box.exe`: show exact missing path.
- Missing `xray.exe`: show warning if Xray selected/available; other protocols can still run.
- TUN without Administrator rights: show clear message to restart as Administrator.
- Port already running: treat as connected if `127.0.0.1:2080` is listening; do not show false disconnected state.
- Connect failure: keep UI disconnected, show error panel and dialog.

## Testing

Unit tests:

- View state transitions:
  - disconnected -> connecting -> connected
  - connected + mode change -> restart required
  - connected + protocol change -> restart required
- Protocol selection config behavior:
  - auto includes all transports
  - forced modes keep only selected transport/endpoint
- Supervisor real-state detection:
  - `127.0.0.1:2080` listener means connected
  - closed port means disconnected
- Error message helpers:
  - TUN admin error
  - missing profile
  - successful connect state

Manual verification:

- Launch `bin/client/mirage-client.exe`.
- Import profile if missing.
- Connect in Normal proxy.
- Verify dashboard says Connected and tray says Disconnect enabled.
- Verify `curl -x http://127.0.0.1:2080 http://www.gstatic.com/generate_204` returns 204.
- Switch protocol to Xray while connected; verify `Restart to apply` appears.
- Restart; verify traffic passes.
- Switch to AmneziaWG; restart; verify traffic passes.
- Try Full TUN without admin; verify clear error.
- Disconnect; verify ports `2080/2081` close and system proxy clears.

## Build rule

Always build artifacts into runtime bin folders:

```powershell
go -C mirage build -o bin\client\mirage-client.exe ./cmd/mirage-client
go -C mirage build -o bin\server\mirage-server.exe ./cmd/mirage-server
$env:GOOS='linux'; $env:GOARCH='amd64'; go -C mirage build -o bin\server\mirage-server-linux-amd64 ./cmd/mirage-server
```

## Out of scope

- Server management GUI.
- Multi-profile manager.
- Advanced routing editor.
- Theme customization.
- Installer packaging.
- Auto-update.
