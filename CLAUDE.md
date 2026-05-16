# MIRAGE project guide

Дата: 2026-05-14

## Цель проекта

MIRAGE — простое клиент-серверное приложение для устойчивого доступа в РФ-сценарии.

Главная цель MVP:

```text
Linux server + Windows client
простая установка
минимум настроек
быстрая работа на слабом VPS
готовые проверенные transport cores как sidecar binaries
наш код = manager/orchestrator/control plane
```

Будущие цели:

```text
Linux client
Android client
RU home node через reverse tunnel
multi-server profiles
optional new transports
```

## Главный архитектурный принцип

Не писать Hysteria2, VLESS/REALITY/XHTTP и AmneziaWG заново в MVP.

Причины:

- протоколы сложные и имеют много edge cases;
- любая неточная реализация создаёт новый fingerprint;
- crypto, QUIC, TLS, WireGuard timers и replay logic опасно переписывать без аудита;
- upstream быстрее реагирует на сетевые изменения;
- MVP должен быть простым, рабочим и быстрым.

Правильное разделение:

```text
MIRAGE app = мозг
transport cores = мышцы
```

MIRAGE отвечает за:

- установку;
- генерацию конфигов;
- запуск transport cores;
- health checks;
- transport selection;
- DNS/routing policy;
- client bundle;
- простой UX;
- диагностику.

Transport cores отвечают за packet/data path.

## Язык и базовый стек

Рекомендуемый язык MVP: Go.

Причины:

- простой subprocess management;
- хороший cross-compile;
- проще networking/system code;
- проще будущий Android path;
- ниже сложность, чем Rust для manager-приложения;
- достаточно быстро для control plane;
- слабый сервер не страдает, потому что heavy data path в sidecar cores.

Rust можно рассмотреть позже для отдельных low-level компонентов, если появится явная причина.

## MVP platforms

### Server MVP

```text
Linux x86_64
Debian 12 / Ubuntu 22.04+
1 vCPU
512 MB RAM
10 GB disk
TCP/443
UDP/443
```

### Client MVP

```text
Windows 10/11 x86_64
admin rights для TUN/Wintun
simple GUI or CLI first
```

### Later clients

```text
Linux desktop
Android
```

## MVP transport set

Основные transport cores:

```text
Hysteria2
VLESS + REALITY + XHTTP via xray-core
AmneziaWG 2 / WireGuard-compatible userspace
sing-box on client for TUN/routing/DNS
```

### Hysteria2

Роль:

```text
fast path
```

Использовать когда:

- UDP/443 жив;
- нужна максимальная скорость;
- network loss есть, но QUIC работает нормально.

### VLESS + REALITY + XHTTP

Роль:

```text
stealth/default fallback
```

Использовать когда:

- UDP режется или деградирует;
- нужен web-like TCP/443;
- нужна защита от active probing;
- нужно выглядеть как обычный HTTPS/web transport.

### AmneziaWG 2

Роль:

```text
simple fast fallback
```

Использовать когда:

- WireGuard-like traffic проходит;
- нужен low CPU;
- нужен простой emergency/speed route.

### sing-box client core

Роль:

```text
TUN
DNS policy
routing policy
outbound management
local SOCKS/HTTP fallback
```

MIRAGE client не должен сам реализовывать TUN/routing в MVP. Он генерирует config, запускает sing-box и управляет состоянием.

## Что не входит в MVP

Не делать в MVP:

```text
custom VPN protocol
custom crypto
custom TUN implementation
custom QUIC implementation
embedded xray/hysteria as libraries
Docker/Kubernetes/Terraform
web admin panel
multi-VPS autorotation
CDN automation
RU home node
MASQUE
NaiveProxy
WebRTC/stego
traffic mimicry profiles
P2P relay mesh
DNS tunnel
```

Причина: всё это увеличивает код, риски и сроки. MVP должен быть маленьким и рабочим.

## Sidecar binary model

MIRAGE поставляет или скачивает transport binaries и управляет ими.

Server layout:

```text
/opt/mirage/
  mirage-server
  bin/
    xray
    hysteria
    amneziawg
  configs/
    mirage.yaml
    xray.json
    hysteria.yaml
    amneziawg.conf
  web/
    index.html
    assets/
    robots.txt
    favicon.ico
  state/
    keys.json
    clients.json
    runtime.json
  logs/
    mirage.log
    xray.log
    hysteria.log
    amneziawg.log
```

Client layout Windows:

```text
%ProgramData%\Mirage\
  mirage-client.exe
  bin\
    sing-box.exe
    wintun.dll
  configs\
    mirage.yaml
    sing-box.json
  state\
    profiles.json
    runtime.json
  logs\
    mirage.log
    sing-box.log
```

## Process model

MVP: one MIRAGE process supervises sidecars.

Server:

```text
mirage-server.service
  ├─ xray child process
  ├─ hysteria child process
  └─ amneziawg child process
```

Client:

```text
mirage-client.exe
  └─ sing-box.exe child process
```

Avoid multiple systemd services in first MVP unless needed. One service reduces install/debug complexity.

## Install UX

### Server install

Target UX:

```bash
curl -fsSL https://example.com/install.sh | sudo bash
sudo mirage-server setup
```

`mirage-server setup` asks minimal questions:

```text
Domain: vpn.example.com
Admin password: ********
Enable Hysteria2: yes
Enable VLESS REALITY XHTTP: yes
Enable AmneziaWG: yes
```

Then it does:

```text
check OS
check root
create /opt/mirage
install mirage-server
install/download sidecar binaries
generate keys
generate sidecar configs
create real web camouflage
open firewall hints
install systemd service
start service
print client link
print QR
```

### Client install

Target UX:

```text
Download mirage-client.exe
Run as Administrator
Paste mirage:// link or import file
Click Connect
```

Client does:

```text
check admin rights
install/copy sing-box.exe
install/copy wintun.dll
parse profile bundle
generate sing-box config
start sing-box
run health checks
select best transport
show connected status
```

## CLI commands

Server commands:

```text
mirage-server setup
mirage-server start
mirage-server stop
mirage-server restart
mirage-server status
mirage-server add-client <name>
mirage-server remove-client <name>
mirage-server show-link <name>
mirage-server show-qr <name>
mirage-server rotate-client <name>
mirage-server doctor
mirage-server logs
```

Client commands:

```text
mirage-client import <mirage-link-or-file>
mirage-client connect
mirage-client disconnect
mirage-client status
mirage-client doctor
mirage-client logs
mirage-client profiles
mirage-client use-profile <name>
```

GUI can wrap same commands later.

## Config bundle

User-facing config should be one link or one file.

Format:

```text
mirage://BASE64URL(JSON)
```

Bundle content shape:

```json
{
  "version": 1,
  "profile_id": "default",
  "server": {
    "domain": "vpn.example.com",
    "health_url": "https://vpn.example.com/api/mirage/health"
  },
  "client": {
    "id": "client-id",
    "name": "my-pc"
  },
  "transports": {
    "hysteria2": {},
    "xray_reality_xhttp": {},
    "amneziawg": {}
  },
  "dns": {
    "mode": "secure",
    "block_system_dns": true,
    "block_ipv6_leaks": true
  },
  "routing": {
    "mode": "auto",
    "default_exit": "foreign"
  }
}
```

Actual secrets/config values are generated by server. Keep schema stable and versioned.

## Transport plugin interface

Code should make transports easy to add.

Conceptual interface:

```go
type Transport interface {
    Name() string
    Role() TransportRole
    ServerConfig(ctx ConfigContext) ([]GeneratedFile, error)
    ClientOutbound(ctx ClientContext) (SingBoxOutbound, error)
    Start(ctx context.Context) error
    Stop(ctx context.Context) error
    HealthCheck(ctx context.Context) (HealthResult, error)
    ExportBundle(ctx BundleContext) (map[string]any, error)
}
```

Transport roles:

```text
speed
stealth
fallback
emergency
```

Suggested package layout:

```text
internal/transports/
  transport.go
  hysteria2/
  xrayrealityxhttp/
  amneziawg/
  registry.go
```

Client should not care about transport internals. Client consumes:

```text
name
role
local endpoint
health score
capabilities
```

## Health scoring

MVP health loop:

```text
every 15 seconds:
  check hysteria2
  check xray_reality_xhttp
  check amneziawg
  compute score
  switch if active transport unhealthy or much worse
```

Basic score:

```text
score = 0 if unreachable
score = 100
  - latency_penalty
  - packet_loss_penalty
  - recent_failure_penalty
  - udp_block_penalty
```

Initial selection:

```text
if hysteria2 healthy and latency acceptable:
  use hysteria2
elif xray_reality_xhttp healthy:
  use xray_reality_xhttp
elif amneziawg healthy:
  use amneziawg
else:
  fail closed
```

Do not implement multipath frame splitting in MVP. Use one active transport plus health-checked fallbacks.

## Routing and DNS MVP

MVP routing:

```text
default -> selected foreign transport
private/local -> direct
DNS -> through tunnel
```

MVP DNS:

```text
block system DNS while connected
use secure DNS through active tunnel
block or disable IPv6 unless configured
fail closed on DNS leak risk
```

RU/foreign split routing comes later.

## Server web camouflage

Server must look like normal HTTPS site on browser requests.

MVP web assets:

```text
GET / -> normal HTML
GET /about -> normal HTML
GET /robots.txt -> normal robots.txt
GET /favicon.ico -> normal favicon
unknown paths -> normal 404
bad auth -> normal web response, not VPN error
```

Do not return unique errors like:

```text
VPN auth failed
invalid UUID
proxy rejected
mirage tunnel error
```

## Security rules

Do not implement custom crypto.

Use upstream cores for:

```text
TLS
QUIC
REALITY
WireGuard/Noise
AEAD
key rotation internals
replay protection internals
```

MIRAGE secrets:

```text
store under /opt/mirage/state on server
restrict permissions 0600
never log private keys or full client bundles
short client tokens where applicable
support client rotation
```

Fail closed for leak protection.

## Performance goals

Server should work on:

```text
1 vCPU
512 MB RAM
```

Design constraints:

- do not proxy data through MIRAGE manager process if sidecar can do it;
- do not keep unnecessary heavy services active beyond listeners and health;
- avoid packet-level processing in MIRAGE manager;
- prefer config generation + process supervision;
- keep logs bounded/rotated;
- avoid large runtime dependencies.

## Implementation phases

### Phase 1: Server MVP

Deliver:

```text
mirage-server setup
sidecar binary management
config generation for xray/hysteria/amneziawg
service supervision
client generation
health endpoint
status/doctor/logs
```

### Phase 2: Windows Client MVP

Deliver:

```text
import mirage:// bundle
generate sing-box config
start/stop sing-box
TUN mode via Wintun
health checks
auto transport selection
status/doctor/logs
```

### Phase 3: Simple GUI

Deliver:

```text
profile import
connect/disconnect
current transport
latency/status
logs export
```

### Phase 4: Linux Client

Reuse client manager with Linux-specific service/TUN handling.

### Phase 5: Android Client

Reuse profile/config model and transport selection logic. Android likely needs different packaging and VPNService integration.

### Phase 6: Advanced features

Consider only after MVP works:

```text
RU home node
multi-server pool
server rotation
MASQUE
NaiveProxy
CDN automation
controller panel
```

## Repository direction

Suggested initial structure:

```text
cmd/
  mirage-server/
  mirage-client/
internal/
  config/
  bundle/
  supervisor/
  transports/
  health/
  server/
  client/
  dns/
  routing/
  platform/
pkg/
  version/
assets/
  web/
  templates/
scripts/
  install-server.sh
```

Keep files small. Prefer clear packages over large all-in-one files.

## Development rules

- Build client artifacts directly into `bin/client/` and server artifacts directly into `bin/server/`.
- Windows GUI client must be built with CGO and WinLibs MinGW in PATH; use `-H=windowsgui` so no console window opens next to the GUI:

```powershell
$mingw = "C:\Users\redmi\AppData\Local\Microsoft\WinGet\Packages\BrechtSanders.WinLibs.POSIX.UCRT_Microsoft.Winget.Source_8wekyb3d8bbwe\mingw64\bin"
$env:Path = $mingw + ";" + $env:Path
$env:CGO_ENABLED = "1"
go -C mirage build -ldflags "-H=windowsgui" -o .\bin\client\mirage-client.exe .\cmd\mirage-client
```

- Server build command:

```powershell
go -C mirage build -o .\bin\server\mirage-server.exe .\cmd\mirage-server
```

- Prefer simple code over clever abstractions.
- No custom crypto.
- No custom protocol in MVP.
- No custom TUN in MVP.
- No Docker-first design.
- No web panel until CLI works.
- Every transport must be behind common interface.
- Every generated config must be reproducible from MIRAGE config/state.
- Logs must be useful for `doctor`, not noisy.
- User setup must require minimal questions.

## Product success criteria

MVP is successful when:

```text
fresh Linux VPS -> one install command -> server running
server prints mirage:// client link
Windows client imports link
Windows client connects via TUN
client auto chooses Hysteria2 / XHTTP / AmneziaWG
DNS does not leak
IPv6 does not leak
server runs on 1 vCPU / 512 MB RAM
non-technical user can connect without editing JSON
```

## Current MVP Status

```text
Server: deployed on Ubuntu 24.04 VPS (1 vCPU, 1 GB RAM); concrete address must not be committed
  - xray: TCP/443 running (VLESS + REALITY + XHTTP)
  - hysteria: UDP/443 running
  - amneziawg: UDP/51820 running (awg0 interface up, NAT configured, WireGuard-compatible)
  - systemd limits: MemoryMax=512M, swap=1G
  - iptables rules: 443/tcp, 443/udp, 51820/udp open
  - pinned sidecar versions with SHA256 checksums
  - amneziawg-tools v1.0.20210914 (no jitter param support)

Client (Windows):
  - mirage-client.exe builds
  - import / mode cheap / connect work
  - sing-box mixed proxy on 127.0.0.1:2080
  - xray sidecar auto-started on connect (127.0.0.1:2081)
  - xray auto-config generated during import
  - doctor checks all binaries + configs + ports
  - end-to-end xray test PASSED: curl -x socks5://127.0.0.1:2081 returns 204 (gstatic)
  - hysteria2 UDP data path currently blocked by ISP; TCP/xray works as fallback
  - TUN mode (full) generates sing-box TUN config with auto_route/strict_route
  - System proxy auto-set on connect (normal/cheap modes) via registry + WinINet refresh
  - System tray GUI with Connect/Disconnect/Mode/Transports/Import/Status/Exit
  - Health check loop: TCP + HTTP checks every 15s with per-transport proxy
  - GUI shows per-transport status and latency in "Transports" submenu
  - sing-box urltest outbound in normal/full mode for auto transport selection
  - urltest interval set to 30s for faster failover
  - Auto-switch in cheap mode after 3 consecutive failures
  - All 3 transports in sing-box: hysteria2 (native), xray (SOCKS sidecar), amneziawg (endpoint)
  - Auto-select prefers xray when healthy (TCP/443 reliable)

Implemented plan tasks:
  - Task 1: internal/modes package with mode constants and selection logic
  - Task 2: internal/singbox package for mixed proxy config generation + urltest support
  - Task 3: pinned versions (xray v26.3.27, hysteria app/v2.9.1) + SHA256 + installer checksum support
  - Task 4: client-side xray sidecar config generation (prepare-xray) + auto-start on connect
  - Task 5: AmneziaWG server + client integration (WireGuard endpoint, no jitter)
  - Task 7: diagnostics package + client/server doctor commands
  - Task 8: docs/mvp-usage.md + CLAUDE.md updated
  - Task TUN/GUI: TUN mode + system tray GUI + auto system proxy
  - Task Health: background health checker + auto-select + GUI status display
  - Task Xray Integration: auto-start/stop sidecar, SOCKS outbound in sing-box, health check via own proxy
  - Task AmneziaWG: server config with [Peer], client private key in bundle, wireguard endpoint

Known issues:
  - sing-box does not support xray xhttp transport; xray runs as separate sidecar
  - sing-box 1.13.11 removed wireguard outbound; uses wireguard endpoint only (no detour routing)
  - hysteria2 UDP data path blocked on current ISP (QUIC handshake OK, data flows timeout)
  - amneziawg-tools v1.0.20210914 does not support jitter params (Jc/Jf/Jd/Jmin/Jmax)
  - amneziawg works as plain WireGuard without anti-DPI protection
```
