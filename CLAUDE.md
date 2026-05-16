# MIRAGE project guide

Дата: 2026-05-16

## Суть проекта

MIRAGE — Go-приложение для устойчивого доступа в РФ-сценарии.

```text
MIRAGE app = manager / orchestrator / control plane
transport cores = packet/data path
```

MVP-фокус:

- Linux server + Windows client.
- Простая установка, минимум настроек.
- Быстрая работа на слабом VPS.
- Sidecar binaries вместо собственной реализации transport-протоколов.
- Клиентский TUN/DNS/routing через sing-box, не через собственный TUN.

## Жёсткие правила архитектуры

Не писать заново в MVP:

- Hysteria2;
- VLESS / REALITY / XHTTP;
- AmneziaWG / WireGuard;
- custom crypto;
- custom QUIC;
- custom TUN;
- packet-level data path внутри MIRAGE manager.

Использовать upstream cores для TLS, QUIC, REALITY, WireGuard/Noise, AEAD, replay protection, key rotation internals.

MIRAGE отвечает за:

- установку и обновление sidecar binaries;
- генерацию воспроизводимых configs из MIRAGE config/state;
- запуск и остановку sidecars;
- health checks;
- transport selection;
- DNS/routing policy;
- bundle import/export;
- диагностику и логи.

## Transport set

| Transport | Роль | Реальность сейчас |
| --- | --- | --- |
| Hysteria2 | fast path over UDP/443 | QUIC handshake есть, data path на текущем ISP timeout |
| xray VLESS + REALITY + XHTTP | reliable stealth fallback over TCP/443 | основной рабочий путь |
| AmneziaWG | simple fast fallback | работает как plain WireGuard-compatible, без jitter |
| sing-box | client TUN / DNS / routing / mixed proxy / urltest | основной client core |

xray XHTTP не поддерживается sing-box напрямую, поэтому xray запускается как отдельный sidecar и даёт локальный SOCKS endpoint.

## Platforms

Server MVP:

```text
Linux x86_64
Debian 12 / Ubuntu 22.04+
1 vCPU, 512 MB RAM target
TCP/443, UDP/443, UDP/51820
```

Client MVP:

```text
Windows 10/11 x86_64
admin rights для TUN/Wintun
CLI + system tray GUI
```

Later: Linux desktop, Android.

## Layout

Server runtime target:

```text
/opt/mirage/
  mirage-server
  bin/{xray,hysteria,amneziawg}
  configs/{mirage.yaml,xray.json,hysteria.yaml}
  state/{keys.json,clients.json,runtime.json}
  logs/{mirage.log,xray.log,hysteria.log,amneziawg.log}
```

Server web camouflage сейчас генерируется Go-шаблонами, не отдельными файлами из `web/`.

Windows client:

```text
%ProgramData%\Mirage\
  mirage-client.exe
  bin\{sing-box.exe,xray.exe,hysteria.exe,wintun.dll}
  configs\{mirage.yaml,sing-box.json,xray.json}
  state\{profiles.json,runtime.json,ru_direct.txt}
  logs\{mirage.log,sing-box.log,xray.log}
```

## Commands

Implemented server commands:

```text
mirage-server setup
mirage-server start
mirage-server status
mirage-server doctor
mirage-server show-link
mirage-server download-sidecars
```

Server commands currently stubbed with `not implemented yet`:

```text
mirage-server stop
mirage-server restart
mirage-server logs
mirage-server add-client
mirage-server remove-client
mirage-server show-qr
mirage-server rotate-client
```

Implemented client commands:

```text
mirage-client import <mirage-link-or-file>
mirage-client mode normal|cheap|full
mirage-client ru-direct on|off
mirage-client connect
mirage-client disconnect
mirage-client prepare-xray
mirage-client status
mirage-client doctor
mirage-client profiles
mirage-client use-profile <profile-id>
mirage-client remove-profile <profile-id>
```

Client command currently stubbed with `not implemented yet`:

```text
mirage-client logs
```

## Config bundle

User-facing config stays one link/file:

```text
mirage://BASE64URL(JSON)
```

Bundle version is currently `1`. Server generates secrets/config values. Never log private keys or full bundles.

## Health and selection

Current client health loop:

- runs every 15s;
- checks TCP + HTTP via per-transport proxy;
- GUI shows status + latency in `Transports` submenu;
- normal/full use sing-box `urltest` with 30s interval;
- cheap mode auto-switches after 3 consecutive failures;
- xray preferred when healthy because TCP/443 reliable on current network.

Fail closed on DNS/leak risk. Do not add multipath splitting in MVP.

## Routing and DNS MVP

Default behavior:

```text
default traffic -> selected foreign transport
private/local -> direct
DNS -> through tunnel
IPv6 -> block/disable unless explicitly configured
```

`ru-direct` is implemented:

- CLI: `mirage-client ru-direct on|off`.
- GUI: checkbox/state exists.
- sing-box config adds RU direct routing via remote `geoip-ru` rule set.
- RU DNS path uses `ru-direct-dns` with detour through selected reliable outbound.
- If explicit protocol is not `auto` or `xray`, enabling `ru-direct` forces compatible config path.

System proxy is set automatically on connect in normal/cheap modes via registry + WinINet refresh.

## Server camouflage

Browser-facing HTTPS should look like normal website:

```text
GET / -> normal HTML
GET /about -> normal HTML
GET /robots.txt -> normal robots.txt
GET /favicon.ico -> normal favicon
unknown paths -> normal 404
bad auth -> normal web response, not VPN error
```

Never return distinctive VPN/proxy errors like `VPN auth failed`, `invalid UUID`, `proxy rejected`, `mirage tunnel error`.

## Security and secrets

- No custom crypto.
- Server secrets under `/opt/mirage/state`, permissions `0600`.
- Never commit concrete VPS address, private keys, full client bundles, or deployment secrets.
- Support client rotation.
- Keep logs useful for `doctor`, bounded, and non-noisy.
- `releasecheck` exists for scanning release artifacts for secret leaks.

## Sidecar versions and download reality

Pinned in code:

- xray `v26.3.27`, linux amd64 SHA256 pinned.
- hysteria `app/v2.9.1`, linux amd64 SHA256 pinned.

Not fully pinned in code:

- sing-box client binary must exist in client `bin`; no pinned downloader path currently documented in code.
- amneziawg server binary uses `AMNEZIAWG_URL`; no built-in URL/SHA in `DefaultManifest`.

## Build rules

Build artifacts directly into `bin/client/` and `bin/server/`.

Windows GUI client build:

```powershell
$mingw = "C:\Users\redmi\AppData\Local\Microsoft\WinGet\Packages\BrechtSanders.WinLibs.POSIX.UCRT_Microsoft.Winget.Source_8wekyb3d8bbwe\mingw64\bin"
$env:Path = $mingw + ";" + $env:Path
$env:CGO_ENABLED = "1"
go -C mirage build -ldflags "-H=windowsgui" -o .\bin\client\mirage-client.exe .\cmd\mirage-client
```

Linux server build:

```powershell
$env:GOOS = "linux"
$env:GOARCH = "amd64"
$env:CGO_ENABLED = "0"
go -C mirage build -o .\bin\server\mirage-server-linux-amd64 .\cmd\mirage-server
Remove-Item Env:\GOOS -ErrorAction SilentlyContinue
Remove-Item Env:\GOARCH -ErrorAction SilentlyContinue
```

Also check `build-release.ps1` and `scripts/install-server.sh` before release packaging.

## Development rules

- Prefer simple code over clever abstractions.
- Prefer clear packages over large all-in-one files.
- Keep every transport behind common interface.
- Do not introduce Docker-first design, web admin panel, CDN automation, multi-VPS autorotation, RU home node, MASQUE, NaiveProxy, WebRTC/stego, P2P relay mesh, or DNS tunnel until MVP is stable.
- Validate only at boundaries: user input, config files, external processes/APIs.
- Avoid compatibility shims for code that can be deleted cleanly.

Current package direction:

```text
cmd/{mirage-server,mirage-client}
internal/{app,bundle,config,diagnostics,downloader,gui,health,modes,platform,profiles,releasecheck,singbox,supervisor,templates,transports}
pkg/version
assets/web
scripts/install-server.sh
```

## Current MVP status

Server deployed on Ubuntu 24.04 VPS; concrete address must not be committed.

Server facts:

- xray TCP/443 running: VLESS + REALITY + XHTTP.
- hysteria UDP/443 running.
- amneziawg UDP/51820 running: `awg0` up, NAT configured.
- systemd limits: `MemoryMax=512M`, swap `1G`.
- iptables open: `443/tcp`, `443/udp`, `51820/udp`.
- amneziawg-tools `v1.0.20210914`; no jitter params support.

Windows client facts:

- `mirage-client.exe` builds.
- import / mode cheap / connect work.
- system tray GUI exists: Connect, Disconnect, Mode, Transports, Import, Status, Exit.
- sing-box mixed proxy listens on `127.0.0.1:2080`.
- xray sidecar auto-starts on connect and listens on `127.0.0.1:2081`.
- xray config generated during import and refreshed before connect.
- doctor checks binaries, configs, ports.
- End-to-end xray test passed: `curl -x socks5://127.0.0.1:2081` returns `204` from gstatic.
- TUN mode (`full`) generates sing-box TUN config with `auto_route` and `strict_route`.
- All 3 transports represented in sing-box config: hysteria2 native, xray SOCKS sidecar, amneziawg endpoint.
- `ru-direct` routing exists in CLI, GUI state, and sing-box config generation.

Known issues:

- Hysteria2 UDP data path blocked by current ISP despite QUIC handshake.
- xray XHTTP requires separate xray sidecar because sing-box does not support it.
- sing-box `1.13.11` removed wireguard outbound; current config uses wireguard endpoint only, no detour routing.
- amneziawg-tools lacks `Jc/Jf/Jd/Jmin/Jmax`; current AmneziaWG works as plain WireGuard without anti-DPI protection.
- Several intended server lifecycle/client-management commands are still stubs.
- `mirage-client logs` is still a stub.

## Done plan chunks

- `internal/modes`: mode constants and selection logic.
- `internal/singbox`: mixed proxy config generation, TUN config, urltest support, RU direct rules.
- Pinned sidecars: xray `v26.3.27`, hysteria `app/v2.9.1`, SHA256 installer checks for linux amd64.
- Client xray sidecar: config generation, auto-start/stop, SOCKS outbound, health via own proxy.
- AmneziaWG server/client integration: server peer config, client private key in bundle, endpoint config.
- Diagnostics package and client/server `doctor` commands.
- TUN mode, tray GUI, automatic system proxy.
- Background health checker, auto-select, GUI transport status.
- Profile store with active profile selection and removal.
- Release artifact secret scanning via `internal/releasecheck`.

## MVP success criteria

MVP good when:

```text
fresh Linux VPS -> one install command -> server running
server prints mirage:// client link
Windows client imports link
Windows client connects via TUN
client auto chooses xray / Hysteria2 / AmneziaWG
DNS does not leak
IPv6 does not leak
server fits 1 vCPU / 512 MB RAM target
non-technical user connects without editing JSON
```
