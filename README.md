# Mirage

[English](README.md) | [Русский](README.ru.md)

Mirage is a Go-based VPN manager/orchestrator for a Linux server and Windows portable client. The data path runs through sidecar cores (`xray`, `hysteria`, `amneziawg`, `sing-box`); Mirage manages installation, config generation, process supervision, profiles, GUI, and diagnostics.

## Contents

- `mirage-server-linux-amd64` — Linux server binary for Ubuntu/Debian VPS.
- `install-server.sh` — one-command server installer.
- `mirage-client.exe` — Windows portable GUI client.
- `SHA256SUMS` — release artifact checksums.

Release page: GitHub Releases for this repository.

## Server install on Ubuntu VPS

Set release asset URLs for your own repository before publishing a release.

### One-command install

```bash
curl -fsSL "https://github.com/<owner>/<repo>/releases/download/v0.1.0/install-server.sh" |
sudo env \
  MIRAGE_SERVER_URL="https://github.com/<owner>/<repo>/releases/download/v0.1.0/mirage-server-linux-amd64" \
  MIRAGE_SHA256SUMS_URL="https://github.com/<owner>/<repo>/releases/download/v0.1.0/SHA256SUMS" \
  bash -s -- --auto
```

If public IP detection fails or you want an explicit host:

```bash
curl -fsSL "https://github.com/<owner>/<repo>/releases/download/v0.1.0/install-server.sh" |
sudo env \
  MIRAGE_SERVER_URL="https://github.com/<owner>/<repo>/releases/download/v0.1.0/mirage-server-linux-amd64" \
  MIRAGE_SHA256SUMS_URL="https://github.com/<owner>/<repo>/releases/download/v0.1.0/SHA256SUMS" \
  bash -s -- --auto --host SERVER_IP
```

Installer does:

- checks root and basic dependencies;
- creates `/opt/mirage` layout;
- downloads server binary and sidecars;
- verifies server checksum;
- generates server/client keys on server;
- creates systemd service;
- opens `443/tcp`, `443/udp`, `51820/udp` best-effort;
- starts `mirage-server.service`;
- prints `mirage://` client link.

### Useful server commands

```bash
sudo systemctl status mirage-server --no-pager
sudo systemctl restart mirage-server
sudo /opt/mirage/mirage-server doctor
sudo /opt/mirage/mirage-server show-link
```

## Windows client

1. Download `mirage-client.exe` from the release page.
2. Run as Administrator.
3. Import the `mirage://` link printed by server setup.
4. Choose a server profile in the GUI.
5. Click Connect.

GUI supports:

- multi-server profile list;
- manual active server selection;
- mode: Normal proxy / Cheap proxy / Full TUN;
- protocol: Auto / Hysteria2 / Xray / AmneziaWG;
- camouflage site override;
- diagnostics and logs;
- tray controls.

Client runtime files are stored under:

```text
%ProgramData%\Mirage\
  bin\
  configs\
  state\
  logs\
```

## Build locally

From repository root on Windows:

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\build-release.ps1
```

This runs tests, builds artifacts, creates `release\`, and writes `SHA256SUMS`.

Manual commands:

```powershell
go test ./...

$mingw = "C:\Users\redmi\AppData\Local\Microsoft\WinGet\Packages\BrechtSanders.WinLibs.POSIX.UCRT_Microsoft.Winget.Source_8wekyb3d8bbwe\mingw64\bin"
$env:Path = $mingw + ";" + $env:Path
$env:CGO_ENABLED = "1"
go build -ldflags "-H=windowsgui" -o .\bin\client\mirage-client.exe .\cmd\mirage-client

$env:GOOS = "linux"
$env:GOARCH = "amd64"
$env:CGO_ENABLED = "0"
go build -o .\bin\server\mirage-server-linux-amd64 .\cmd\mirage-server
Remove-Item Env:\GOOS -ErrorAction SilentlyContinue
Remove-Item Env:\GOARCH -ErrorAction SilentlyContinue
```

## Release publish

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\build-release.ps1

gh release upload v0.1.0 `
  .\release\mirage-server-linux-amd64 `
  .\release\install-server.sh `
  .\release\mirage-client.exe `
  .\release\SHA256SUMS `
  --repo <owner>/<repo> `
  --clobber
```

## Security notes

Never commit or publish:

- real `mirage://` links;
- `default.link`, `profile.link`, `profiles.json`;
- server/client private keys;
- generated `mirage.yaml` from a real VPS;
- SSH key paths or private SSH keys;
- real server IPs in docs or defaults;
- `bin/` or `release/` artifacts as source files.

Before publishing, run:

```powershell
go test ./...
powershell -ExecutionPolicy Bypass -File .\scripts\build-release.ps1

git grep -n "<REAL_SERVER_IP>\|<REAL_SSH_KEY_NAME>\|BEGIN .*PRIVATE KEY\|mirage://[A-Za-z0-9_-]\{20,\}" -- .

git ls-files | Select-String -Pattern '(^|/)release/|(^|/)bin/|(^|/)state/|prod-link|\.exe$|mirage-server-linux-amd64$|default\.link$|profile\.link$|profiles\.json$'
```

Both scans should return no findings.

## Project layout

```text
cmd/                 CLI entrypoints
internal/app/        client/server app flows
internal/gui/        Fyne GUI + tray controller
internal/profiles/   multi-server client profiles
internal/probe/      latency/status probes
internal/transports/ server/client transport config generation
internal/singbox/    sing-box config generation
scripts/             installer and release build scripts
```
