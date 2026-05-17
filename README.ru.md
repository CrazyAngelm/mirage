# Mirage

[English](README.md) | [Русский](README.ru.md)

Mirage — Go-based VPN manager/orchestrator для Linux-сервера и Windows portable client. Data path работает через sidecar cores (`xray`, `hysteria`, `amneziawg`, `sing-box`); Mirage управляет установкой, генерацией config, process supervision, profiles, GUI и diagnostics.

## Состав

- `mirage-server-linux-amd64` — Linux server binary для Ubuntu/Debian VPS.
- `install-server.sh` — one-command server installer.
- `mirage-windows-amd64.zip` — Windows portable GUI client с нужными sidecars `sing-box` и `xray`.
- `mirage-client.exe` — advanced/manual Windows client binary без sidecars.
- `SHA256SUMS` — checksums release artifacts.

Release page: GitHub Releases этого repository.

## Установка сервера на Ubuntu VPS

Перед публикацией release укажи release asset URLs для своего repository.

### One-command install

```bash
curl -fsSL "https://github.com/CrazyAngelm/mirage/releases/download/v0.1.0/install-server.sh" |
sudo env \
  MIRAGE_SERVER_URL="https://github.com/CrazyAngelm/mirage/releases/download/v0.1.0/mirage-server-linux-amd64" \
  MIRAGE_SHA256SUMS_URL="https://github.com/CrazyAngelm/mirage/releases/download/v0.1.0/SHA256SUMS" \
  bash -s -- --auto
```

Если public IP detection не сработал или нужен явный host:

```bash
curl -fsSL "https://github.com/CrazyAngelm/mirage/releases/download/v0.1.0/install-server.sh" |
sudo env \
  MIRAGE_SERVER_URL="https://github.com/CrazyAngelm/mirage/releases/download/v0.1.0/mirage-server-linux-amd64" \
  MIRAGE_SHA256SUMS_URL="https://github.com/CrazyAngelm/mirage/releases/download/v0.1.0/SHA256SUMS" \
  bash -s -- --auto --host SERVER_IP
```

Installer делает:

- проверяет root и базовые dependencies;
- создаёт `/opt/mirage` layout;
- скачивает server binary и sidecars;
- проверяет server checksum;
- генерирует server/client keys на server;
- создаёт systemd service;
- открывает `443/tcp`, `443/udp`, `51820/udp` best-effort;
- запускает `mirage-server.service`;
- печатает `mirage://` client link.

### Полезные server commands

```bash
sudo systemctl status mirage-server --no-pager
sudo systemctl restart mirage-server
sudo /opt/mirage/mirage-server doctor
sudo /opt/mirage/mirage-server show-link
```

## Windows client

1. Скачай `mirage-windows-amd64.zip` со страницы release.
2. Распакуй ZIP.
3. Запусти `mirage-client.exe` из распакованной папки от Administrator.
4. Импортируй `mirage://` link, напечатанный server setup.
5. Выбери server profile в GUI.
6. Нажми Connect.

GUI поддерживает:

- multi-server profile list;
- ручной выбор active server;
- mode: Normal proxy / Cheap proxy / Full TUN;
- protocol: Auto / Hysteria2 / Xray / AmneziaWG;
- camouflage site override;
- diagnostics и logs;
- tray controls.

Client runtime files лежат здесь:

```text
%ProgramData%\Mirage\
  bin\
  configs\
  state\
  logs\
```

## Локальная сборка

Из repository root на Windows:

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\build-release.ps1
```

Команда запускает tests, собирает artifacts, создаёт `release\` и пишет `SHA256SUMS`.

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

## Публикация release

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\build-release.ps1

gh release upload v0.1.0 `
  .\release\mirage-server-linux-amd64 `
  .\release\install-server.sh `
  .\release\mirage-windows-amd64.zip `
  .\release\mirage-client.exe `
  .\release\SHA256SUMS `
  --repo CrazyAngelm/mirage `
  --clobber
```

## Security notes

Никогда не commit/publish:

- реальные `mirage://` links;
- `default.link`, `profile.link`, `profiles.json`;
- server/client private keys;
- generated `mirage.yaml` с реального VPS;
- SSH key paths или private SSH keys;
- реальные server IP в docs или defaults;
- `bin/` или `release/` artifacts как source files.

Перед публикацией запусти:

```powershell
go test ./...
powershell -ExecutionPolicy Bypass -File .\scripts\build-release.ps1

git grep -n "<REAL_SERVER_IP>\|<REAL_SSH_KEY_NAME>\|BEGIN .*PRIVATE KEY\|mirage://[A-Za-z0-9_-]\{20,\}" -- .

git ls-files | Select-String -Pattern '(^|/)release/|(^|/)bin/|(^|/)state/|prod-link|\.exe$|mirage-server-linux-amd64$|default\.link$|profile\.link$|profiles\.json$'
```

Оба scans должны вернуть no findings.

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
