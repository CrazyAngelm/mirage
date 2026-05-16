# MIRAGE Release Install and Multi-Server GUI Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a release-ready MIRAGE MVP where Ubuntu server installs from a private GitHub Release with one command, Windows portable GUI imports multiple servers, lets user choose active server manually, shows latency/status, supports camouflage-site override, and release checks prevent secrets from entering repo/artifacts.

**Architecture:** Keep MIRAGE as Go control plane. Add small focused packages for server host detection, client profile storage, probe status, and release secret scanning; wire them into existing `internal/app` and `internal/gui` without replacing transport sidecars. GUI remains Fyne-based but moves from simple stacked cards to a dark sidebar dashboard.

**Tech Stack:** Go 1.25, Fyne v2, PowerShell build commands on Windows, Bash install script for Ubuntu/Debian, existing sidecar model for xray/hysteria/amneziawg/sing-box.

---

## File structure

Create:

- `internal/serverhost/host.go` — public host detection and explicit host normalization for server setup.
- `internal/serverhost/host_test.go` — tests for IP/domain normalization and detector fallback behavior.
- `internal/profiles/store.go` — client profile store, active profile selection, import/update/remove, camouflage override.
- `internal/profiles/store_test.go` — TDD tests for multi-server profile behavior.
- `internal/probe/probe.go` — TCP/HTTP probe and status classifier for GUI latency/status.
- `internal/probe/probe_test.go` — tests with local TCP/HTTP servers.
- `internal/releasecheck/scan.go` — repo/release secret scanner reusable from CLI and tests.
- `internal/releasecheck/scan_test.go` — tests for secret patterns and allowed fake values.
- `scripts/build-release.ps1` — build Windows client, Linux server, copy installer, generate `SHA256SUMS`, run secret scan.

Modify:

- `internal/bundle/bundle.go` — add typed helpers for `display_name`, `server.host`, `server.camouflage_site`, compatibility with `server.domain`.
- `internal/bundle/bundle_test.go` — tests for new bundle fields and old domain-only bundles.
- `internal/app/server.go` — parse `setup --auto --host`, use `serverhost`, write bundle with host/camouflage, suppress full link except explicit output path as needed.
- `internal/app/server_test.go` — setup/bundle tests for host and no hardcoded previous deployment address.
- `scripts/install-server.sh` — release-aware one-command installer.
- `bin/server/install-server.sh` — keep runtime copy in sync with `scripts/install-server.sh`.
- `internal/app/client.go` — use `profiles.Store` for import/connect/mode/doctor/profile commands.
- `internal/app/client_test.go` — tests for profile commands and active-profile config generation.
- `internal/gui/state.go` — add active profile, profile list, probe statuses, camouflage fields, reconnect-required state.
- `internal/gui/controller.go` — expose profile import/list/activate/remove/camouflage actions and probe labels.
- `internal/gui/controller_test.go` — controller tests for profile and camouflage behavior.
- `internal/gui/supervisor.go` — use active profile from profile store and probe status loop.
- `internal/gui/window.go` — implement dark sidebar dashboard.
- `internal/gui/window_nocgo.go` — keep no-CGO stub unchanged except compile compatibility if signatures change.
- `internal/healthcheck/healthcheck.go` — ensure active profile link flows into health checks when needed.
- `.gitignore` — ignore `.superpowers/`, release zips/checksums if generated outside `bin` policy, runtime profile/state files.

---

## Task 1: Bundle helpers for host, display name, and camouflage site

**Files:**
- Modify: `internal/bundle/bundle.go`
- Modify: `internal/bundle/bundle_test.go`

- [ ] **Step 1: Write failing bundle tests**

Add tests to `internal/bundle/bundle_test.go`:

```go
func TestBundleServerHostPrefersHostThenDomain(t *testing.T) {
	b := Bundle{Server: map[string]any{"host": "203.0.113.10", "domain": "vpn.example.com"}}
	if got := b.ServerHost(); got != "203.0.113.10" {
		t.Fatalf("ServerHost() = %q", got)
	}

	b = Bundle{Server: map[string]any{"domain": "vpn.example.com"}}
	if got := b.ServerHost(); got != "vpn.example.com" {
		t.Fatalf("ServerHost() fallback = %q", got)
	}
}

func TestBundleDisplayNameAndCamouflageSite(t *testing.T) {
	b := Bundle{
		ProfileID:   "main-vps",
		DisplayName: "Main VPS",
		Server:      map[string]any{"camouflage_site": "www.microsoft.com"},
	}
	if b.Name() != "Main VPS" {
		t.Fatalf("Name() = %q", b.Name())
	}
	if b.CamouflageSite() != "www.microsoft.com" {
		t.Fatalf("CamouflageSite() = %q", b.CamouflageSite())
	}
}
```

- [ ] **Step 2: Run test to verify failure**

Run: `go -C mirage test ./internal/bundle`

Expected: FAIL with missing `DisplayName`, `ServerHost`, `Name`, or `CamouflageSite`.

- [ ] **Step 3: Implement bundle helpers**

Update `internal/bundle/bundle.go`:

```go
type Bundle struct {
	Version     int            `json:"version"`
	ProfileID   string         `json:"profile_id"`
	DisplayName string         `json:"display_name,omitempty"`
	Server      map[string]any `json:"server"`
	Client      map[string]any `json:"client"`
	Transports  map[string]any `json:"transports"`
	DNS         map[string]any `json:"dns"`
	Routing     map[string]any `json:"routing"`
}

func (b Bundle) Name() string {
	if b.DisplayName != "" {
		return b.DisplayName
	}
	if b.ProfileID != "" {
		return b.ProfileID
	}
	return b.ServerHost()
}

func (b Bundle) ServerHost() string {
	if b.Server == nil {
		return ""
	}
	if value, _ := b.Server["host"].(string); value != "" {
		return value
	}
	if value, _ := b.Server["domain"].(string); value != "" {
		return value
	}
	return ""
}

func (b Bundle) CamouflageSite() string {
	if b.Server == nil {
		return ""
	}
	value, _ := b.Server["camouflage_site"].(string)
	return value
}
```

- [ ] **Step 4: Verify bundle tests pass**

Run: `go -C mirage test ./internal/bundle`

Expected: PASS.

- [ ] **Step 5: Commit**

```powershell
git add -- mirage/internal/bundle/bundle.go mirage/internal/bundle/bundle_test.go
git commit -m @'
Add bundle host and camouflage helpers

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
'@
```

---

## Task 2: Server host detection and setup args

**Files:**
- Create: `internal/serverhost/host.go`
- Create: `internal/serverhost/host_test.go`
- Modify: `internal/app/server.go`
- Modify: `internal/app/server_test.go`

- [ ] **Step 1: Write failing serverhost tests**

Create `internal/serverhost/host_test.go`:

```go
package serverhost

import "testing"

func TestNormalizeHost(t *testing.T) {
	cases := map[string]string{
		" 203.0.113.10 ":        "203.0.113.10",
		"https://vpn.example.com": "vpn.example.com",
		"http://example.com/":     "example.com",
	}
	for input, want := range cases {
		if got := Normalize(input); got != want {
			t.Fatalf("Normalize(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestResolveUsesExplicitHost(t *testing.T) {
	host, err := Resolve("203.0.113.10", func() (string, error) { return "198.51.100.2", nil })
	if err != nil {
		t.Fatal(err)
	}
	if host != "203.0.113.10" {
		t.Fatalf("host = %q", host)
	}
}

func TestResolveUsesDetector(t *testing.T) {
	host, err := Resolve("", func() (string, error) { return "198.51.100.2", nil })
	if err != nil {
		t.Fatal(err)
	}
	if host != "198.51.100.2" {
		t.Fatalf("host = %q", host)
	}
}
```

- [ ] **Step 2: Run test to verify failure**

Run: `go -C mirage test ./internal/serverhost`

Expected: FAIL because package does not exist.

- [ ] **Step 3: Implement serverhost package**

Create `internal/serverhost/host.go`:

```go
package serverhost

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Detector func() (string, error)

func Normalize(value string) string {
	value = strings.TrimSpace(value)
	value = strings.TrimPrefix(value, "https://")
	value = strings.TrimPrefix(value, "http://")
	value = strings.TrimSuffix(value, "/")
	return value
}

func Resolve(explicit string, detector Detector) (string, error) {
	if host := Normalize(explicit); host != "" {
		return host, nil
	}
	if detector == nil {
		detector = DetectPublicIP
	}
	host, err := detector()
	if err != nil {
		return "", err
	}
	host = Normalize(host)
	if host == "" {
		return "", fmt.Errorf("public host detection returned empty value")
	}
	return host, nil
}

func DetectPublicIP() (string, error) {
	client := http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get("https://api.ipify.org")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return "", fmt.Errorf("public host detection failed: %s", resp.Status)
	}
	payload, err := io.ReadAll(io.LimitReader(resp.Body, 128))
	if err != nil {
		return "", err
	}
	return Normalize(string(payload)), nil
}
```

- [ ] **Step 4: Verify serverhost tests pass**

Run: `go -C mirage test ./internal/serverhost`

Expected: PASS.

- [ ] **Step 5: Add server setup parser test**

Add to `internal/app/server_test.go`:

```go
func TestRunServerSetupAcceptsAutoHost(t *testing.T) {
	args := parseServerSetupArgs([]string{"--auto", "--host", "203.0.113.10"})
	if !args.Auto {
		t.Fatal("Auto = false")
	}
	if args.Host != "203.0.113.10" {
		t.Fatalf("Host = %q", args.Host)
	}
}
```

- [ ] **Step 6: Implement setup args parser**

Add to `internal/app/server.go` near `RunServer`:

```go
type serverSetupArgs struct {
	Auto bool
	Host string
}

func parseServerSetupArgs(args []string) serverSetupArgs {
	parsed := serverSetupArgs{}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--auto":
			parsed.Auto = true
		case "--host":
			if i+1 < len(args) {
				parsed.Host = args[i+1]
				i++
			}
		default:
			if parsed.Host == "" {
				parsed.Host = args[i]
			}
		}
	}
	return parsed
}
```

Update `RunServer` setup branch:

```go
case "setup":
	parsed := parseServerSetupArgs(args[1:])
	return setupServer(parsed.Host, parsed.Auto, stdout)
```

Change signature:

```go
func setupServer(host string, auto bool, stdout io.Writer) error
```

Inside `setupServer`, resolve host before config:

```go
resolvedHost, err := serverhost.Resolve(host, nil)
if err != nil {
	return err
}
cfg := config.DefaultServerConfig(resolvedHost, base)
```

- [ ] **Step 7: Verify server app tests**

Run: `go -C mirage test ./internal/app`

Expected: PASS.

- [ ] **Step 8: Commit**

```powershell
git add -- mirage/internal/serverhost/host.go mirage/internal/serverhost/host_test.go mirage/internal/app/server.go mirage/internal/app/server_test.go
git commit -m @'
Add server host detection for setup

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
'@
```

---

## Task 3: Server bundle host and camouflage fields

**Files:**
- Modify: `internal/app/server.go`
- Modify: `internal/app/server_test.go`

- [ ] **Step 1: Write failing bundle test**

Add to `internal/app/server_test.go`:

```go
func TestBuildDefaultBundleIncludesHostDisplayNameAndCamouflage(t *testing.T) {
	cfg := config.DefaultServerConfig("203.0.113.10", t.TempDir())
	cfg.Clients = []config.ClientRecord{{ID: "client-id", Name: "default-pc"}}
	cfg.Transports.Hysteria2.Password = "hysteria-password"
	cfg.Transports.XrayRealityXHTTP.UUID = "uuid"
	cfg.Transports.XrayRealityXHTTP.PublicKey = "public-key"
	cfg.Transports.XrayRealityXHTTP.ShortID = "short-id"
	cfg.Transports.AmneziaWG.PublicKey = "server-public"
	cfg.Transports.AmneziaWG.ClientPrivateKey = "client-private"

	link, err := buildDefaultBundle(cfg)
	if err != nil {
		t.Fatal(err)
	}
	b, err := bundle.Decode(link)
	if err != nil {
		t.Fatal(err)
	}
	if b.DisplayName != "Main VPS" {
		t.Fatalf("DisplayName = %q", b.DisplayName)
	}
	if b.ServerHost() != "203.0.113.10" {
		t.Fatalf("ServerHost = %q", b.ServerHost())
	}
	if b.CamouflageSite() != "www.microsoft.com" {
		t.Fatalf("CamouflageSite = %q", b.CamouflageSite())
	}
}
```

- [ ] **Step 2: Run test to verify failure**

Run: `go -C mirage test ./internal/app -run TestBuildDefaultBundleIncludesHostDisplayNameAndCamouflage -v`

Expected: FAIL because `DisplayName`, `host`, or `camouflage_site` missing.

- [ ] **Step 3: Update buildDefaultBundle**

In `internal/app/server.go`, update `buildDefaultBundle` bundle literal:

```go
return bundle.Encode(bundle.Bundle{
	Version:     1,
	ProfileID:   "main-vps",
	DisplayName: "Main VPS",
	Server: map[string]any{
		"host":             cfg.Server.Domain,
		"domain":           "",
		"health_url":       "https://" + cfg.Server.Domain + "/api/mirage/health",
		"camouflage_site": "www.microsoft.com",
	},
	Client: map[string]any{"id": cfg.Clients[0].ID, "name": cfg.Clients[0].Name},
	Transports: map[string]any{
		"hysteria2":          map[string]any{"server": cfg.Server.Domain, "port": cfg.Transports.Hysteria2.Port, "password": cfg.Transports.Hysteria2.Password},
		"xray_reality_xhttp": map[string]any{"server": cfg.Server.Domain, "port": cfg.Transports.XrayRealityXHTTP.Port, "uuid": cfg.Transports.XrayRealityXHTTP.UUID, "public_key": cfg.Transports.XrayRealityXHTTP.PublicKey, "short_id": cfg.Transports.XrayRealityXHTTP.ShortID, "path": cfg.Transports.XrayRealityXHTTP.Path, "server_name": cfg.Transports.XrayRealityXHTTP.ServerName},
		"amneziawg":          map[string]any{"server": cfg.Server.Domain, "port": cfg.Transports.AmneziaWG.Port, "public_key": cfg.Transports.AmneziaWG.PublicKey, "address": "10.77.0.2/32", "client_private_key": cfg.Transports.AmneziaWG.ClientPrivateKey, "jc": cfg.Transports.AmneziaWG.Jc, "jf": cfg.Transports.AmneziaWG.Jf, "jd": cfg.Transports.AmneziaWG.Jd, "jmin": cfg.Transports.AmneziaWG.Jmin, "jmax": cfg.Transports.AmneziaWG.Jmax},
	},
	DNS:     map[string]any{"mode": cfg.DNS.Mode, "block_system_dns": cfg.DNS.BlockSystemDNS, "block_ipv6_leaks": cfg.DNS.BlockIPv6Leaks},
	Routing: map[string]any{"mode": cfg.Routing.Mode, "default_exit": cfg.Routing.DefaultExit, "client_mode": "normal"},
})
```

- [ ] **Step 4: Verify app tests**

Run: `go -C mirage test ./internal/app`

Expected: PASS.

- [ ] **Step 5: Commit**

```powershell
git add -- mirage/internal/app/server.go mirage/internal/app/server_test.go
git commit -m @'
Include host metadata in server bundles

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
'@
```

---

## Task 4: Release-aware server installer

**Files:**
- Modify: `scripts/install-server.sh`
- Modify: `bin/server/install-server.sh`

- [ ] **Step 1: Add installer shellcheck-safe structure manually**

Update `scripts/install-server.sh` to accept release variables:

```bash
MIRAGE_RELEASE_BASE_URL="${MIRAGE_RELEASE_BASE_URL:-}"
MIRAGE_SERVER_URL="${MIRAGE_SERVER_URL:-}"
MIRAGE_SHA256SUMS_URL="${MIRAGE_SHA256SUMS_URL:-}"
AUTO_SETUP="no"
SETUP_HOST=""

while [ "$#" -gt 0 ]; do
  case "$1" in
    --auto) AUTO_SETUP="yes" ;;
    --host) shift; SETUP_HOST="${1:-}" ;;
    --release-base-url) shift; MIRAGE_RELEASE_BASE_URL="${1:-}" ;;
    *) echo "unknown argument: $1" >&2; exit 2 ;;
  esac
  shift
done

if [ -n "$MIRAGE_RELEASE_BASE_URL" ]; then
  MIRAGE_SERVER_URL="${MIRAGE_SERVER_URL:-$MIRAGE_RELEASE_BASE_URL/mirage-server-linux-amd64}"
  MIRAGE_SHA256SUMS_URL="${MIRAGE_SHA256SUMS_URL:-$MIRAGE_RELEASE_BASE_URL/SHA256SUMS}"
fi
```

Add checksum verification function:

```bash
verify_checksum() {
  local file="$1"
  local name="$2"
  if [ -z "$MIRAGE_SHA256SUMS_URL" ]; then
    return 0
  fi
  local sums
  sums="$(mktemp)"
  curl -fsSL "$MIRAGE_SHA256SUMS_URL" -o "$sums"
  (cd "$(dirname "$file")" && grep "  $name$" "$sums" | sha256sum -c -)
  rm -f "$sums"
}
```

After binary install, run auto setup/start when requested:

```bash
if [ "$AUTO_SETUP" = "yes" ]; then
  if [ -n "$SETUP_HOST" ]; then
    /opt/mirage/mirage-server setup --auto --host "$SETUP_HOST"
  else
    /opt/mirage/mirage-server setup --auto
  fi
  systemctl restart mirage-server.service
  /opt/mirage/mirage-server show-link
fi
```

- [ ] **Step 2: Sync runtime installer copy**

Run: `Copy-Item -Force .\mirage\scripts\install-server.sh .\mirage\bin\server\install-server.sh`

Expected: no output and destination exists.

- [ ] **Step 3: Syntax check installer**

Run: `bash -n mirage/scripts/install-server.sh && bash -n mirage/bin/server/install-server.sh`

Expected: exit 0.

- [ ] **Step 4: Commit**

```powershell
git add -- mirage/scripts/install-server.sh mirage/bin/server/install-server.sh
git commit -m @'
Make server installer release-aware

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
'@
```

---

## Task 5: Multi-server profile store

**Files:**
- Create: `internal/profiles/store.go`
- Create: `internal/profiles/store_test.go`

- [ ] **Step 1: Write failing profile store tests**

Create `internal/profiles/store_test.go`:

```go
package profiles

import (
	"path/filepath"
	"testing"

	"mirage/internal/bundle"
)

func TestImportAddsProfilesAndKeepsExisting(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "state"))
	first := testLink(t, "main", "Main VPS", "203.0.113.10")
	second := testLink(t, "backup", "Backup EU", "198.51.100.2")

	if err := store.ImportLink(first); err != nil { t.Fatal(err) }
	if err := store.ImportLink(second); err != nil { t.Fatal(err) }

	profiles, err := store.List()
	if err != nil { t.Fatal(err) }
	if len(profiles) != 2 { t.Fatalf("len = %d", len(profiles)) }
	active, err := store.Active()
	if err != nil { t.Fatal(err) }
	if active.ID != "main" { t.Fatalf("active = %q", active.ID) }
}

func TestSetActiveAndCamouflageOverride(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "state"))
	if err := store.ImportLink(testLink(t, "main", "Main VPS", "203.0.113.10")); err != nil { t.Fatal(err) }
	if err := store.SetCamouflageOverride("main", "www.cloudflare.com"); err != nil { t.Fatal(err) }
	active, err := store.Active()
	if err != nil { t.Fatal(err) }
	if active.CamouflageSite() != "www.cloudflare.com" { t.Fatalf("camouflage = %q", active.CamouflageSite()) }
}

func testLink(t *testing.T, id, name, host string) string {
	t.Helper()
	link, err := bundle.Encode(bundle.Bundle{
		Version: 1,
		ProfileID: id,
		DisplayName: name,
		Server: map[string]any{"host": host, "camouflage_site": "www.microsoft.com"},
		Transports: map[string]any{},
		DNS: map[string]any{},
		Routing: map[string]any{},
	})
	if err != nil { t.Fatal(err) }
	return link
}
```

- [ ] **Step 2: Run test to verify failure**

Run: `go -C mirage test ./internal/profiles`

Expected: FAIL because package does not exist.

- [ ] **Step 3: Implement profile store**

Create `internal/profiles/store.go`:

```go
package profiles

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"mirage/internal/bundle"
)

type Store struct { stateDir string }

type File struct {
	Version  int       `json:"version"`
	Profiles []Profile `json:"profiles"`
}

type Profile struct {
	ID                       string `json:"id"`
	DisplayName              string `json:"display_name"`
	Link                     string `json:"link"`
	CamouflageSiteOverride   string `json:"camouflage_site_override,omitempty"`
	LastLatencyMS            int    `json:"last_latency_ms,omitempty"`
	LastStatus               string `json:"last_status,omitempty"`
}

func NewStore(stateDir string) Store { return Store{stateDir: stateDir} }

func (s Store) ImportLink(link string) error {
	link = strings.TrimSpace(strings.TrimPrefix(link, "﻿"))
	b, err := bundle.Decode(link)
	if err != nil { return err }
	id := b.ProfileID
	if id == "" { id = b.ServerHost() }
	if id == "" { return fmt.Errorf("profile id or server host required") }
	file, err := s.read()
	if err != nil { return err }
	profile := Profile{ID: id, DisplayName: b.Name(), Link: link}
	updated := false
	for i := range file.Profiles {
		if file.Profiles[i].ID == id {
			profile.CamouflageSiteOverride = file.Profiles[i].CamouflageSiteOverride
			profile.LastLatencyMS = file.Profiles[i].LastLatencyMS
			profile.LastStatus = file.Profiles[i].LastStatus
			file.Profiles[i] = profile
			updated = true
		}
	}
	if !updated { file.Profiles = append(file.Profiles, profile) }
	if err := s.write(file); err != nil { return err }
	if _, err := os.Stat(s.activePath()); os.IsNotExist(err) {
		return s.SetActive(id)
	}
	return nil
}

func (s Store) List() ([]Profile, error) {
	file, err := s.read()
	if err != nil { return nil, err }
	return file.Profiles, nil
}

func (s Store) Active() (Profile, error) {
	idBytes, err := os.ReadFile(s.activePath())
	if err != nil { return Profile{}, err }
	id := strings.TrimSpace(string(idBytes))
	file, err := s.read()
	if err != nil { return Profile{}, err }
	for _, profile := range file.Profiles {
		if profile.ID == id { return profile, nil }
	}
	return Profile{}, fmt.Errorf("active profile %q not found", id)
}

func (s Store) SetActive(id string) error {
	file, err := s.read()
	if err != nil { return err }
	for _, profile := range file.Profiles {
		if profile.ID == id {
			if err := os.MkdirAll(s.stateDir, 0755); err != nil { return err }
			return os.WriteFile(s.activePath(), []byte(id+"\n"), 0600)
		}
	}
	return fmt.Errorf("profile %q not found", id)
}

func (s Store) Remove(id string) error {
	activeID := ""
	if payload, err := os.ReadFile(s.activePath()); err == nil { activeID = strings.TrimSpace(string(payload)) }
	if id == activeID { return fmt.Errorf("cannot remove active profile") }
	file, err := s.read()
	if err != nil { return err }
	next := file.Profiles[:0]
	removed := false
	for _, profile := range file.Profiles {
		if profile.ID == id { removed = true; continue }
		next = append(next, profile)
	}
	if !removed { return fmt.Errorf("profile %q not found", id) }
	file.Profiles = next
	return s.write(file)
}

func (s Store) SetCamouflageOverride(id string, site string) error {
	file, err := s.read()
	if err != nil { return err }
	for i := range file.Profiles {
		if file.Profiles[i].ID == id {
			file.Profiles[i].CamouflageSiteOverride = strings.TrimSpace(site)
			return s.write(file)
		}
	}
	return fmt.Errorf("profile %q not found", id)
}

func (p Profile) Bundle() (bundle.Bundle, error) { return bundle.Decode(p.Link) }

func (p Profile) CamouflageSite() string {
	if p.CamouflageSiteOverride != "" { return p.CamouflageSiteOverride }
	b, err := p.Bundle()
	if err != nil { return "" }
	return b.CamouflageSite()
}

func (s Store) read() (File, error) {
	payload, err := os.ReadFile(s.path())
	if os.IsNotExist(err) { return File{Version: 1}, nil }
	if err != nil { return File{}, err }
	var file File
	if err := json.Unmarshal(payload, &file); err != nil { return File{}, err }
	if file.Version == 0 { file.Version = 1 }
	return file, nil
}

func (s Store) write(file File) error {
	if err := os.MkdirAll(s.stateDir, 0755); err != nil { return err }
	payload, err := json.MarshalIndent(file, "", "  ")
	if err != nil { return err }
	return os.WriteFile(s.path(), append(payload, '\n'), 0600)
}

func (s Store) path() string { return filepath.Join(s.stateDir, "profiles.json") }
func (s Store) activePath() string { return filepath.Join(s.stateDir, "active_profile_id.txt") }
```

- [ ] **Step 4: Verify profile tests pass**

Run: `go -C mirage test ./internal/profiles`

Expected: PASS.

- [ ] **Step 5: Commit**

```powershell
git add -- mirage/internal/profiles/store.go mirage/internal/profiles/store_test.go
git commit -m @'
Add multi-server profile store

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
'@
```

---

## Task 6: Wire profiles into client CLI and config generation

**Files:**
- Modify: `internal/app/client.go`
- Modify: `internal/app/client_test.go`

- [ ] **Step 1: Write failing CLI/profile tests**

Add tests to `internal/app/client_test.go` using temp `platform` base helper already present in the file if available; otherwise test profile store integration via exported `ImportProfileGUI` and `ReadProfileLink`.

```go
func TestReadProfileLinkUsesActiveProfile(t *testing.T) {
	base := t.TempDir()
	state := filepath.Join(base, "state")
	store := profiles.NewStore(state)
	first := mustProfileLink(t, "main", "203.0.113.10")
	second := mustProfileLink(t, "backup", "198.51.100.2")
	if err := store.ImportLink(first); err != nil { t.Fatal(err) }
	if err := store.ImportLink(second); err != nil { t.Fatal(err) }
	if err := store.SetActive("backup"); err != nil { t.Fatal(err) }
	link, err := ReadProfileLink(base)
	if err != nil { t.Fatal(err) }
	b, err := bundle.Decode(link)
	if err != nil { t.Fatal(err) }
	if b.ProfileID != "backup" { t.Fatalf("ProfileID = %q", b.ProfileID) }
}
```

- [ ] **Step 2: Run failing app test**

Run: `go -C mirage test ./internal/app -run TestReadProfileLinkUsesActiveProfile -v`

Expected: FAIL because `ReadProfileLink` still reads `profile.link`.

- [ ] **Step 3: Update ImportProfileGUI**

In `internal/app/client.go`, replace `profile.link` write with profile store import:

```go
store := profiles.NewStore(filepath.Join(base, "state"))
if err := store.ImportLink(link); err != nil {
	return err
}
```

Keep mode/transport/ru_direct initialization only if files do not exist.

- [ ] **Step 4: Update ReadProfileLink**

Replace `ReadProfileLink` body:

```go
func ReadProfileLink(base string) (string, error) {
	store := profiles.NewStore(filepath.Join(base, "state"))
	profile, err := store.Active()
	if err == nil {
		return profile.Link, nil
	}
	payload, readErr := os.ReadFile(filepath.Join(base, "state", "profile.link"))
	if readErr != nil {
		return "", err
	}
	return strings.TrimSpace(strings.TrimPrefix(string(payload), "﻿")), nil
}
```

- [ ] **Step 5: Add CLI commands**

In `RunClient`, implement:

```go
case "profiles":
	store := profiles.NewStore(filepath.Join(platform.ClientBaseDir(), "state"))
	items, err := store.List()
	if err != nil { return err }
	for _, item := range items {
		fmt.Fprintf(stdout, "%s\t%s\t%s\n", item.ID, item.DisplayName, item.LastStatus)
	}
	return nil
case "use-profile":
	if len(args) < 2 { return fmt.Errorf("usage: mirage-client use-profile PROFILE_ID") }
	store := profiles.NewStore(filepath.Join(platform.ClientBaseDir(), "state"))
	return store.SetActive(args[1])
case "remove-profile":
	if len(args) < 2 { return fmt.Errorf("usage: mirage-client remove-profile PROFILE_ID") }
	store := profiles.NewStore(filepath.Join(platform.ClientBaseDir(), "state"))
	return store.Remove(args[1])
```

Remove `profiles`, `use-profile` from not-implemented list.

- [ ] **Step 6: Verify app tests**

Run: `go -C mirage test ./internal/app`

Expected: PASS.

- [ ] **Step 7: Commit**

```powershell
git add -- mirage/internal/app/client.go mirage/internal/app/client_test.go
git commit -m @'
Use active profile for client commands

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
'@
```

---

## Task 7: Apply camouflage override to generated client configs

**Files:**
- Modify: `internal/app/client.go`
- Modify: `internal/app/client_test.go`

- [ ] **Step 1: Write failing camouflage override test**

Add test in `internal/app/client_test.go`:

```go
func TestCamouflageOverrideChangesXrayServerName(t *testing.T) {
	b := bundle.Bundle{
		Version: 1,
		ProfileID: "main",
		Server: map[string]any{"host": "203.0.113.10", "camouflage_site": "www.microsoft.com"},
		Transports: map[string]any{
			"xray_reality_xhttp": map[string]any{"server": "203.0.113.10", "port": float64(443), "uuid": "u", "server_name": "www.microsoft.com", "public_key": "pk", "short_id": "sid", "path": "/api/session"},
		},
	}
	applyCamouflageOverride(&b, "www.cloudflare.com")
	xray := b.Transports["xray_reality_xhttp"].(map[string]any)
	if xray["server_name"] != "www.cloudflare.com" {
		t.Fatalf("server_name = %v", xray["server_name"])
	}
}
```

- [ ] **Step 2: Run failing test**

Run: `go -C mirage test ./internal/app -run TestCamouflageOverrideChangesXrayServerName -v`

Expected: FAIL because function missing.

- [ ] **Step 3: Implement override helper**

Add to `internal/app/client.go`:

```go
func applyCamouflageOverride(b *bundle.Bundle, site string) {
	site = strings.TrimSpace(site)
	if site == "" { return }
	if b.Server == nil { b.Server = map[string]any{} }
	b.Server["camouflage_site"] = site
	if xray, ok := b.Transports["xray_reality_xhttp"].(map[string]any); ok {
		xray["server_name"] = site
	}
}
```

Before `regenerateClientConfig` and `generateXrayConfig` use active profile:

```go
store := profiles.NewStore(filepath.Join(base, "state"))
if active, err := store.Active(); err == nil {
	applyCamouflageOverride(&b, active.CamouflageSite())
}
```

- [ ] **Step 4: Verify app tests**

Run: `go -C mirage test ./internal/app`

Expected: PASS.

- [ ] **Step 5: Commit**

```powershell
git add -- mirage/internal/app/client.go mirage/internal/app/client_test.go
git commit -m @'
Apply camouflage overrides to client configs

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
'@
```

---

## Task 8: Probe latency/status package

**Files:**
- Create: `internal/probe/probe.go`
- Create: `internal/probe/probe_test.go`

- [ ] **Step 1: Write failing probe tests**

Create `internal/probe/probe_test.go`:

```go
package probe

import (
	"net"
	"testing"
	"time"
)

func TestClassify(t *testing.T) {
	if got := Classify(50*time.Millisecond, nil); got.Status != Online { t.Fatalf("status = %s", got.Status) }
	if got := Classify(900*time.Millisecond, nil); got.Status != Slow { t.Fatalf("status = %s", got.Status) }
	if got := Classify(0, errTest{}); got.Status != Offline { t.Fatalf("status = %s", got.Status) }
}

func TestTCPProbe(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil { t.Fatal(err) }
	defer ln.Close()
	go func() { conn, _ := ln.Accept(); if conn != nil { conn.Close() } }()
	result := TCP(ln.Addr().String(), time.Second)
	if result.Status == Offline { t.Fatalf("result = %+v", result) }
	if result.LatencyMS <= 0 { t.Fatalf("latency = %d", result.LatencyMS) }
}

type errTest struct{}
func (errTest) Error() string { return "boom" }
```

- [ ] **Step 2: Run failing test**

Run: `go -C mirage test ./internal/probe`

Expected: FAIL because package does not exist.

- [ ] **Step 3: Implement probe package**

Create `internal/probe/probe.go`:

```go
package probe

import (
	"net"
	"time"
)

type Status string

const (
	Unknown Status = "unknown"
	Online  Status = "online"
	Slow    Status = "slow"
	Offline Status = "offline"
)

type Result struct {
	Status    Status
	LatencyMS int
	Error     string
}

func TCP(address string, timeout time.Duration) Result {
	start := time.Now()
	conn, err := net.DialTimeout("tcp", address, timeout)
	latency := time.Since(start)
	if conn != nil { conn.Close() }
	return Classify(latency, err)
}

func Classify(latency time.Duration, err error) Result {
	if err != nil { return Result{Status: Offline, Error: err.Error()} }
	ms := int(latency.Milliseconds())
	if ms < 1 { ms = 1 }
	if latency > 750*time.Millisecond { return Result{Status: Slow, LatencyMS: ms} }
	return Result{Status: Online, LatencyMS: ms}
}
```

- [ ] **Step 4: Verify probe tests**

Run: `go -C mirage test ./internal/probe`

Expected: PASS.

- [ ] **Step 5: Commit**

```powershell
git add -- mirage/internal/probe/probe.go mirage/internal/probe/probe_test.go
git commit -m @'
Add server latency probes

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
'@
```

---

## Task 9: GUI state/controller profile actions

**Files:**
- Modify: `internal/gui/state.go`
- Modify: `internal/gui/controller.go`
- Modify: `internal/gui/controller_test.go`
- Modify: `internal/gui/supervisor.go`

- [ ] **Step 1: Write failing controller tests**

Add to `internal/gui/controller_test.go`:

```go
func TestControllerSetsCamouflageSite(t *testing.T) {
	deps := &fakeDeps{}
	c := NewControllerWithDeps(deps)
	if err := c.SetCamouflageSite("www.cloudflare.com"); err != nil { t.Fatal(err) }
	if deps.camouflageSite != "www.cloudflare.com" { t.Fatalf("site = %q", deps.camouflageSite) }
	state := c.State()
	if !state.ReconnectRequired { t.Fatal("ReconnectRequired = false") }
}
```

- [ ] **Step 2: Run failing GUI tests**

Run: `go -C mirage test ./internal/gui -run TestControllerSetsCamouflageSite -v`

Expected: FAIL because controller method/state fields missing.

- [ ] **Step 3: Extend GUI state**

In `internal/gui/state.go`, add fields to `State`:

```go
type ProfileState struct {
	ID        string
	Name      string
	Host      string
	Status    string
	LatencyMS int
	Active    bool
}

Profiles          []ProfileState
ActiveProfileID   string
ActiveProfileName string
CamouflageSite    string
ReconnectRequired bool
```

- [ ] **Step 4: Extend controller deps interface**

In `internal/gui/controller.go`, add methods to `ControllerDeps`:

```go
Profiles() ([]ProfileState, error)
SetActiveProfile(id string) error
RemoveProfile(id string) error
SetCamouflageSite(site string) error
```

Add controller methods:

```go
func (c *Controller) SetCamouflageSite(site string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := c.deps.SetCamouflageSite(site); err != nil { return c.fail(err) }
	c.state.CamouflageSite = site
	c.state.ReconnectRequired = true
	c.updateState(c.state)
	return nil
}
```

- [ ] **Step 5: Implement runtime deps methods**

In `internal/gui/controller.go` runtime deps, use `profiles.NewStore(filepath.Join(platform.ClientBaseDir(), "state"))` for list/active/remove/camouflage. Convert store profiles into `ProfileState`.

- [ ] **Step 6: Update fake deps**

Add fields/methods in `controller_test.go` fake deps:

```go
camouflageSite string
func (f *fakeDeps) SetCamouflageSite(site string) error { f.camouflageSite = site; return nil }
func (f *fakeDeps) Profiles() ([]ProfileState, error) { return nil, nil }
func (f *fakeDeps) SetActiveProfile(id string) error { return nil }
func (f *fakeDeps) RemoveProfile(id string) error { return nil }
```

- [ ] **Step 7: Verify GUI tests**

Run: `go -C mirage test ./internal/gui`

Expected: PASS.

- [ ] **Step 8: Commit**

```powershell
git add -- mirage/internal/gui/state.go mirage/internal/gui/controller.go mirage/internal/gui/controller_test.go mirage/internal/gui/supervisor.go
git commit -m @'
Add GUI profile controller state

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
'@
```

---

## Task 10: Dark sidebar dashboard GUI

**Files:**
- Modify: `internal/gui/window.go`
- Modify: `internal/gui/window_nocgo.go` only if compile signatures require it

- [ ] **Step 1: Build GUI to expose compile errors before edits**

Run with CGO toolchain:

```powershell
$mingw = "C:\Users\redmi\AppData\Local\Microsoft\WinGet\Packages\BrechtSanders.WinLibs.POSIX.UCRT_Microsoft.Winget.Source_8wekyb3d8bbwe\mingw64\bin"
$env:Path = $mingw + ";" + $env:Path
$env:CGO_ENABLED = "1"
go -C mirage build -ldflags "-H=windowsgui" -o .\bin\client\mirage-client.exe .\cmd\mirage-client
```

Expected: PASS before visual refactor or fail only from earlier intentional interface changes.

- [ ] **Step 2: Replace window layout**

In `internal/gui/window.go`, update `DesktopGUI` fields:

```go
profileList      *fyne.Container
camouflageEntry  *widget.Entry
reconnectLabel   *widget.Label
serverStatusCard *widget.Card
```

Replace `build()` content with:

```go
sidebar := container.NewVBox(
	widget.NewLabelWithStyle("Mirage", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
	widget.NewButton("Dashboard", func() {}),
	widget.NewButton("Servers", func() {}),
	widget.NewButton("Routing", func() {}),
	widget.NewButton("Diagnostics", func() {}),
)

activeCard := widget.NewCard("Active server", "", container.NewVBox(
	g.statusLabel,
	g.transportLabel,
	g.actionButton,
))

cards := container.NewGridWithColumns(3,
	widget.NewCard("Servers", "", g.profileList),
	g.transportCards(),
	widget.NewCard("Security", "", container.NewVBox(g.ruDirectCheck, g.reconnectLabel)),
)

g.camouflageEntry = widget.NewEntry()
g.camouflageEntry.SetPlaceHolder("www.microsoft.com")
g.camouflageEntry.OnSubmitted = func(value string) { go func() { _ = g.controller.SetCamouflageSite(value) }() }

main := container.NewVBox(
	activeCard,
	cards,
	widget.NewCard("Routing", "", container.NewVBox(g.modeSelect, g.protocolSelect, g.camouflageEntry)),
	widget.NewCard("Diagnostics", "", container.NewVBox(container.NewHBox(importProfile, runDoctor, openLogs), g.doctorOutput)),
)

g.window.SetContent(container.NewBorder(nil, nil, sidebar, nil, container.NewVScroll(main)))
```

Preserve existing button callbacks from old `build()`.

- [ ] **Step 3: Update applyState**

In `applyState`, update profile list and camouflage field:

```go
g.camouflageEntry.SetText(state.CamouflageSite)
if state.ReconnectRequired {
	g.reconnectLabel.SetText("Reconnect required")
} else {
	g.reconnectLabel.SetText("DNS OK • IPv6 blocked")
}
```

- [ ] **Step 4: Build GUI**

Run same CGO build command.

Expected: PASS and `mirage/bin/client/mirage-client.exe` updated.

- [ ] **Step 5: Manual GUI smoke test**

Run: `.\mirage\bin\client\mirage-client.exe`

Expected: window opens with dark sidebar dashboard, Connect/Disconnect button visible, profiles/server card visible, camouflage entry visible, Doctor button usable.

- [ ] **Step 6: Commit**

```powershell
git add -- mirage/internal/gui/window.go mirage/internal/gui/window_nocgo.go mirage/bin/client/mirage-client.exe
git commit -m @'
Redesign GUI as dark sidebar dashboard

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
'@
```

---

## Task 11: Release secret scanner

**Files:**
- Create: `internal/releasecheck/scan.go`
- Create: `internal/releasecheck/scan_test.go`
- Modify: `cmd/mirage-server/main.go` is not needed; scanner used from build script through `go test` and optional `go test` only.

- [ ] **Step 1: Write failing scanner tests**

Create `internal/releasecheck/scan_test.go`:

```go
package releasecheck

import "testing"

func TestScanCatchesSecrets(t *testing.T) {
	findings := ScanText("state/default.link", "mirage://abc\nclient_private_key: secret\n")
	if len(findings) < 2 { t.Fatalf("findings = %#v", findings) }
}

func TestScanAllowsFakeDocumentationValues(t *testing.T) {
	findings := ScanText("docs/example.md", "host 203.0.113.10\nmirage://example\n")
	if len(findings) != 0 { t.Fatalf("findings = %#v", findings) }
}
```

- [ ] **Step 2: Run failing scanner tests**

Run: `go -C mirage test ./internal/releasecheck`

Expected: FAIL because package does not exist.

- [ ] **Step 3: Implement scanner**

Create `internal/releasecheck/scan.go`:

```go
package releasecheck

import "strings"

type Finding struct {
	Path   string
	Reason string
}

func ScanText(path string, text string) []Finding {
	if strings.Contains(text, "mirage://example") || strings.Contains(text, "203.0.113.") || strings.Contains(text, "198.51.100.") {
		text = strings.ReplaceAll(text, "mirage://example", "")
		text = strings.ReplaceAll(text, "203.0.113.", "")
		text = strings.ReplaceAll(text, "198.51.100.", "")
	}
	checks := []struct{ needle, reason string }{
		{"mirage://", "full mirage link"},
		{"client_private_key", "client private key"},
		{"private_key:", "private key"},
		{"PRIVATE KEY", "private key block"},
		{"ssh-key-", "ssh key path"},
		{"default.link", "generated link file"},
		{"profile.link", "generated profile link"},
	}
	var findings []Finding
	for _, check := range checks {
		if strings.Contains(text, check.needle) {
			findings = append(findings, Finding{Path: path, Reason: check.reason})
		}
	}
	return findings
}
```

- [ ] **Step 4: Verify scanner tests**

Run: `go -C mirage test ./internal/releasecheck`

Expected: PASS.

- [ ] **Step 5: Commit**

```powershell
git add -- mirage/internal/releasecheck/scan.go mirage/internal/releasecheck/scan_test.go
git commit -m @'
Add release secret scanner

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
'@
```

---

## Task 12: Build release script and gitignore cleanup

**Files:**
- Create: `scripts/build-release.ps1`
- Modify: `.gitignore`

- [ ] **Step 1: Read `.gitignore`**

Run is not needed; use file read/edit tools. Ensure `.superpowers/` and generated runtime state patterns are ignored.

- [ ] **Step 2: Update `.gitignore`**

Add entries:

```gitignore
.superpowers/
**/state/default.link
**/state/profile.link
**/state/profiles.json
**/state/active_profile_id.txt
mirage/release/
```

Keep `mirage/bin/client` and `mirage/bin/server` artifact policy intact.

- [ ] **Step 3: Create build-release.ps1**

Create `scripts/build-release.ps1`:

```powershell
$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $PSScriptRoot
$mirage = Join-Path $root 'mirage'
$release = Join-Path $mirage 'release'
New-Item -ItemType Directory -Force $release | Out-Null

$mingw = 'C:\Users\redmi\AppData\Local\Microsoft\WinGet\Packages\BrechtSanders.WinLibs.POSIX.UCRT_Microsoft.Winget.Source_8wekyb3d8bbwe\mingw64\bin'
if (Test-Path $mingw) { $env:Path = $mingw + ';' + $env:Path }
$env:CGO_ENABLED = '1'
go -C $mirage test ./...
go -C $mirage build -ldflags '-H=windowsgui' -o .\bin\client\mirage-client.exe .\cmd\mirage-client
$env:GOOS = 'linux'
$env:GOARCH = 'amd64'
$env:CGO_ENABLED = '0'
go -C $mirage build -o .\bin\server\mirage-server-linux-amd64 .\cmd\mirage-server
Remove-Item Env:\GOOS -ErrorAction SilentlyContinue
Remove-Item Env:\GOARCH -ErrorAction SilentlyContinue

Copy-Item (Join-Path $mirage 'bin\client\mirage-client.exe') (Join-Path $release 'mirage-client.exe') -Force
Copy-Item (Join-Path $mirage 'bin\server\mirage-server-linux-amd64') (Join-Path $release 'mirage-server-linux-amd64') -Force
Copy-Item (Join-Path $mirage 'scripts\install-server.sh') (Join-Path $release 'install-server.sh') -Force

Push-Location $release
Get-FileHash -Algorithm SHA256 .\mirage-client.exe, .\mirage-server-linux-amd64, .\install-server.sh |
  ForEach-Object { "$($_.Hash.ToLower())  $([System.IO.Path]::GetFileName($_.Path))" } |
  Set-Content -Encoding ascii .\SHA256SUMS
Pop-Location

$bad = Select-String -Path (Join-Path $release '*') -Pattern 'mirage://','client_private_key','private_key:','PRIVATE KEY','ssh-key-' -SimpleMatch -ErrorAction SilentlyContinue
if ($bad) {
  $bad | ForEach-Object { Write-Error "Secret-like release content: $($_.Path):$($_.LineNumber)" }
}

Write-Host "Release artifacts ready: $release"
```

- [ ] **Step 4: Run release build script**

Run: `powershell -ExecutionPolicy Bypass -File .\scripts\build-release.ps1`

Expected: tests pass, artifacts under `mirage/release`, `SHA256SUMS` created, no secret findings.

- [ ] **Step 5: Commit**

```powershell
git add -- .gitignore scripts/build-release.ps1
git commit -m @'
Add release build script

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
'@
```

---

## Task 13: Full verification and bug fixing pass

**Files:**
- Modify any files with failing tests or build errors.

- [ ] **Step 1: Format Go files**

Run: `go -C mirage fmt ./...`

Expected: exit 0.

- [ ] **Step 2: Run full Go tests**

Run: `go -C mirage test ./...`

Expected: PASS. If any test fails, fix code and rerun same command until PASS.

- [ ] **Step 3: Build Windows GUI client**

Run:

```powershell
$mingw = "C:\Users\redmi\AppData\Local\Microsoft\WinGet\Packages\BrechtSanders.WinLibs.POSIX.UCRT_Microsoft.Winget.Source_8wekyb3d8bbwe\mingw64\bin"
$env:Path = $mingw + ";" + $env:Path
$env:CGO_ENABLED = "1"
go -C mirage build -ldflags "-H=windowsgui" -o .\bin\client\mirage-client.exe .\cmd\mirage-client
```

Expected: PASS and artifact exists.

- [ ] **Step 4: Build Linux server**

Run:

```powershell
$env:GOOS='linux'
$env:GOARCH='amd64'
$env:CGO_ENABLED='0'
go -C mirage build -o .\bin\server\mirage-server-linux-amd64 .\cmd\mirage-server
Remove-Item Env:\GOOS
Remove-Item Env:\GOARCH
```

Expected: PASS and artifact exists.

- [ ] **Step 5: Run installer syntax checks**

Run: `bash -n mirage/scripts/install-server.sh && bash -n mirage/bin/server/install-server.sh`

Expected: PASS.

- [ ] **Step 6: Run release script**

Run: `powershell -ExecutionPolicy Bypass -File .\scripts\build-release.ps1`

Expected: PASS and release artifacts exist.

- [ ] **Step 7: Run repo secret scans**

Run:

```powershell
git grep -n "mirage://" -- . ':!mirage/docs/superpowers/specs/*' ':!mirage/docs/superpowers/plans/*'
git grep -n "client_private_key\|private_key:\|PRIVATE KEY\|ssh-key-" -- . ':!mirage/docs/superpowers/specs/*' ':!mirage/docs/superpowers/plans/*'
```

Expected: no real secret findings. Test fixtures with fake values are allowed only in `_test.go`.

- [ ] **Step 8: Manual GUI smoke test**

Run: `.\mirage\bin\client\mirage-client.exe`

Expected: GUI opens, dark sidebar visible, active server/profile area visible, camouflage site input visible, Doctor button does not crash, window can close to tray.

- [ ] **Step 9: Commit fixes and final artifacts**

```powershell
git add -- mirage scripts .gitignore
git commit -m @'
Prepare Mirage release MVP

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
'@
```

---

## Task 14: Private GitHub repo and release publication

**Files:**
- No source modifications required after verification.

- [ ] **Step 1: Ask user for explicit confirmation**

Creating private repo and GitHub Release changes remote state. Ask:

```text
Создать private GitHub repo `<owner>/<repo>` и загрузить release artifacts? Это внешнее действие в GitHub.
```

Expected: user says yes.

- [ ] **Step 2: Create private repo**

Run after confirmation:

```powershell
gh repo create <owner>/<repo> --private --source . --remote mirage-origin --push
```

Expected: repo created and current branch pushed.

- [ ] **Step 3: Create release**

Run after confirmation:

```powershell
gh release create v0.1.0 .\mirage\release\mirage-server-linux-amd64 .\mirage\release\install-server.sh .\mirage\release\mirage-client.exe .\mirage\release\SHA256SUMS --repo <owner>/<repo> --title "Mirage v0.1.0" --notes "Release-first MVP: Ubuntu server installer, Windows portable client, multi-server GUI, and secret-safe release gate."
```

Expected: release URL printed.

- [ ] **Step 4: Report install command**

Return final command using actual release asset URL from GitHub output.

Expected format:

```bash
curl -fsSL "$PRIVATE_RELEASE_INSTALL_URL" | sudo bash -s -- --auto
```

Do not invent URL; use `gh release view` or output from GitHub.

---

## Self-review

Spec coverage:

- Private GitHub Release delivery: Tasks 4, 12, 14.
- One-command Ubuntu install: Tasks 2, 3, 4, 13.
- No hardcoded previous deployment address: Tasks 2, 3, 11, 13.
- Bundle host/camouflage: Tasks 1, 3.
- Multi-server profiles and manual active server: Tasks 5, 6, 9.
- Ping/status: Tasks 8, 9, 10.
- Camouflage site in GUI: Tasks 7, 9, 10.
- Dark sidebar GUI: Task 10.
- Secret-safe release gate: Tasks 11, 12, 13.
- Full verification and bug fixes: Task 13.

Placeholder scan:

- No unresolved plan markers or unspecified edge-case steps.
- Commands include expected outcomes.
- Remote GitHub creation requires explicit user confirmation in Task 14 because it changes shared remote state.

Type consistency:

- `bundle.Bundle.DisplayName`, `Bundle.ServerHost`, `Bundle.CamouflageSite`, `profiles.Store`, `probe.Result`, and GUI `ProfileState` names are consistent across tasks.
