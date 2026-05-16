# MIRAGE release install, multi-server GUI, and secret-safe packaging design

Дата: 2026-05-15

## Цель

Сделать MIRAGE переносимым для коллеги: сервер на Ubuntu ставится одной командой из private GitHub Release, Windows клиент запускается как portable EXE, импортирует `mirage://` bundle и подключается без ручного редактирования JSON. Параллельно убрать хардкод адресов прошлых deployments и keys из пользовательского пути, добавить список серверов, ручной выбор active server, latency/status, поле camouflage site и более продовый GUI.

## Выбранный подход

Release-first MVP.

- Отдельный private GitHub repo `mirage` в аккаунте пользователя.
- GitHub Release содержит готовые артефакты:
  - `mirage-server-linux-amd64`
  - `install-server.sh`
  - `mirage-client.exe`
  - `SHA256SUMS`
- Сервер ставится через private GitHub Release asset URL, который будет указан в release notes:
  ```bash
  curl -fsSL "$MIRAGE_INSTALL_URL" | sudo bash -s -- --auto
  ```
- Клиент — portable Windows EXE. MSI/installer не входит в первую итерацию.
- GUI направление: `Sidebar pro` layout + `Dark security` style.

## Не входит в первую итерацию

- MSI/NSIS installer для Windows.
- Public GitHub repo.
- Автоматический выбор лучшего сервера между несколькими VPS.
- Server-side remote config sync из GUI.
- Web admin panel.
- Новые VPN-протоколы.

## Server install flow

`install-server.sh` должен работать на Ubuntu/Debian под root.

Шаги:

1. Проверить root, OS, arch.
2. Создать `/opt/mirage/{bin,configs,state,logs,web}`.
3. Скачать `mirage-server-linux-amd64` из того же GitHub Release.
4. Проверить checksum из `SHA256SUMS`.
5. Скачать sidecars через existing downloader/default manifest или release/env URLs.
6. Создать systemd service `mirage-server.service`.
7. Открыть порты `443/tcp`, `443/udp`, `51820/udp` через ufw/iptables best effort.
8. Запустить `mirage-server setup --auto`.
9. Запустить service.
10. Напечатать `mirage://` link и путь `/opt/mirage/state/default.link`.

`mirage-server setup --auto`:

- если `--host` не передан, определяет public IP;
- не использует hardcoded address from previous deployments;
- генерирует fresh keys на сервере;
- пишет `/opt/mirage/configs/mirage.yaml` с mode `0600`;
- пишет transport configs;
- пишет `/opt/mirage/state/default.link` с mode `0600`;
- не логирует private keys и полный bundle без явной команды `show-link`.

IP-only режим является first-class: bundle использует `server.host`, а не принудительный domain. `server.domain` остаётся optional для будущего domain/TLS сценария.

## Bundle schema changes

Bundle v1 расширяется без поломки existing fields.

Новые/уточнённые поля:

```json
{
  "version": 1,
  "profile_id": "main-vps",
  "display_name": "Main VPS",
  "server": {
    "host": "203.0.113.10",
    "domain": "",
    "health_url": "https://203.0.113.10/api/mirage/health",
    "camouflage_site": "www.microsoft.com"
  },
  "client": {
    "id": "client-id",
    "name": "default-pc"
  },
  "transports": {
    "hysteria2": {},
    "xray_reality_xhttp": {},
    "amneziawg": {}
  },
  "dns": {},
  "routing": {}
}
```

Transport-specific `server` values should derive from `server.host` unless a transport explicitly overrides endpoint. Existing bundles that only have `server.domain` continue to import.

## Client profile storage

Replace single active-only storage with profile list.

Current:

```text
%ProgramData%\Mirage\state\profile.link
%ProgramData%\Mirage\state\mode.txt
%ProgramData%\Mirage\state\transport.txt
%ProgramData%\Mirage\state\ru_direct.txt
```

New:

```text
%ProgramData%\Mirage\state\profiles.json
%ProgramData%\Mirage\state\active_profile_id.txt
%ProgramData%\Mirage\state\mode.txt
%ProgramData%\Mirage\state\transport.txt
%ProgramData%\Mirage\state\ru_direct.txt
```

`profiles.json` stores imported server profiles:

```json
{
  "version": 1,
  "profiles": [
    {
      "id": "main-vps",
      "display_name": "Main VPS",
      "link": "mirage://example", 
      "camouflage_site_override": "www.microsoft.com",
      "last_latency_ms": 38,
      "last_status": "online"
    }
  ]
}
```

Import behavior:

- `import <link-or-file>` validates bundle;
- adds profile or updates same `profile_id`;
- does not delete existing profiles;
- sets active profile only when no active profile exists or user chooses imported profile in GUI.

CLI additions:

- `profiles` — list imported profiles.
- `use-profile <id>` — set active profile.
- `remove-profile <id>` — remove profile if not connected.

Existing commands (`connect`, `disconnect`, `doctor`, `mode`, `ru-direct`) use active profile.

## Ping/status design

GUI periodically measures server health with network probes, not ICMP.

Signals:

- TCP connect latency to active endpoint, default `host:443`.
- HTTPS health URL if present.
- Existing per-transport health checker for active profile.

Statuses:

- `online` — fast TCP or health URL success.
- `slow` — reachable but over threshold.
- `offline` — recent probes failed.
- `unknown` — no probe result yet.

Ping display examples:

- `38 ms`
- `slow 420 ms`
- `offline`
- `checking...`

Manual server selection only: GUI never auto-switches active server in this iteration.

## Camouflage site in GUI

GUI includes editable `Camouflage site` field for active profile.

Rules:

- Default value comes from imported bundle `server.camouflage_site`.
- User override is stored in `profiles.json` as `camouflage_site_override`.
- Changing value regenerates client configs and marks reconnect required.
- GUI warning text: `Changing camouflage site affects client-side SNI/REALITY target only. Server config remains from imported profile.`
- If server-side matching is required by a transport, imported bundle values remain authoritative unless user intentionally overrides.

## GUI design

Chosen layout: `Sidebar pro`.

Chosen style: `Dark security`.

Main window structure:

```text
┌──────────────────────────────────────────────────────┐
│ Mirage — Control Panel                               │
├───────────────┬──────────────────────────────────────┤
│ Mirage        │ Active server card                   │
│ Dashboard     │ Main VPS                             │
│ Servers       │ online • 38 ms • Xray TCP/443        │
│ Routing       │ [Connect/Disconnect]                 │
│ Diagnostics   │                                      │
│               │ Cards: Servers / Transports / Security│
│               │ Camouflage site input                │
│               │ Mode / Protocol / RU direct          │
└───────────────┴──────────────────────────────────────┘
```

Visual language:

- dark navy/black background;
- bright cyan active nav/accent;
- green online status;
- amber degraded status;
- red disconnect/error actions;
- rounded cards;
- clear spacing;
- no tray-only UX feeling.

Navigation sections:

- Dashboard — connection state, active server, quick settings.
- Servers — imported profiles, latency/status, import/set active/remove.
- Routing — mode, protocol, RU direct, camouflage site.
- Diagnostics — doctor output, logs folder, copied commands.

Tray remains secondary quick control: Connect, Disconnect, active status, Exit.

## Secret and release safety

Repo/release must not contain generated runtime secrets.

Never publish:

- `/opt/mirage/state/*`
- `%ProgramData%\Mirage\state\*`
- `default.link`
- `profile.link`
- `profiles.json`
- `mirage.yaml` generated on real server
- private keys
- full generated `mirage://` links
- addresses from previous deployments in product defaults
- SSH key paths or private SSH keys

Allowed in repo:

- source code;
- templates;
- install scripts;
- tests with fake keys/IPs;
- docs that do not expose real secrets.

Release gate must run before GitHub release:

1. `git status` clean or only expected files.
2. Search tracked files for real IP, `mirage://`, `private_key`, `client_private_key`, SSH key path patterns.
3. Search release artifacts list; ensure only binaries/scripts/checksums.
4. `go -C mirage test ./...` passes.
5. Build artifacts into `mirage/bin/client` and `mirage/bin/server`.

## Tests

Add/adjust tests for:

- server setup uses provided/detected host, no hardcoded IP;
- bundle supports `server.host` and `server.camouflage_site`;
- old bundles with `server.domain` still import;
- profile import adds/updates profile without deleting others;
- active profile chooses correct bundle for config generation;
- camouflage override changes generated client config;
- profile list commands;
- ping/status classifier;
- secret scan catches real links/private key patterns in candidate release inputs.

## Acceptance criteria

- Fresh Ubuntu VPS can run one command from private GitHub Release and end with running `mirage-server.service`.
- Server setup prints usable `mirage://` link.
- Windows portable client imports link and shows profile in GUI.
- GUI can import multiple servers and manually set active server.
- GUI shows status/latency for profiles.
- GUI can edit camouflage site for active profile and requires reconnect.
- GUI uses `Sidebar pro` + `Dark security` visual direction.
- No hardcoded current IP/keys in source defaults, release scripts, or release artifacts.
- Tests pass before release.
