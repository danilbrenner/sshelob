# sshelob MVP — Implementation Checklist

## [x] Phase 1 — Project Scaffold
- [x] `go mod init` with module name, add `golang.org/x/crypto`, `gopkg.in/yaml.v3`, `github.com/charmbracelet/bubbletea` deps
- [x] Create directory layout: `cmd/sshelob/`, `internal/config/`, `internal/tunnel/`, `internal/health/`, `internal/tui/`
- [x] `cmd/sshelob/main.go` — entry point: load config, init tunnel engine, run

## [x] Phase 2 — Config
- [x] Define `Config` and `TunnelDef` structs (type, host, user, port, bind/dest, key path, health block)
- [x] `config.Load(path string) (*Config, error)` — reads YAML, validates required fields, returns error with field path on failure
- [x] Unit tests: valid config parses correctly, missing required field returns descriptive error, unknown tunnel type returns error

## [x] Phase 3 — Tunnel Engine
- [x] `TunnelState` type with states: `Stopped`, `Connecting`, `Connected`, `Reconnecting`, `Error`
- [x] `Tunnel` struct wrapping a config entry + state + log ring buffer (fixed cap, e.g. 200 lines)
- [x] `Tunnel.Start()` — dials SSH with `golang.org/x/crypto/ssh`, opens port forward (local/remote/dynamic), blocks until disconnect, publishes state transitions
- [x] `Tunnel.Stop()` — cancels context, transitions to `Stopped`
- [x] Auto-reconnect loop: 1s initial delay, 2× backoff, max 30s, unlimited retries, stops only on `Stop()`
- [x] Unit tests: state transitions on start/stop, backoff timing, ring buffer wraps correctly

## [x] Phase 3.x — CLI (Pre-TUI)
- [x] `sshelob list` — lists configured tunnels in format `(index)type: name`
- [x] `sshelob run <indexes>` — starts one or multiple tunnels by 1-based index and keeps running until stopped (example: `sshelob run 1,2,3`)

## [x] Phase 4 — Build & Test Pipeline
- [x] `Makefile` targets: `build`, `test`, `lint`, `cross-compile`
- [x] GitHub Actions workflow: lint + test on push/PR
- [x] `golangci-lint` config (`.golangci.yml`)
- [x] Cross-compile check: `GOOS=linux`, `GOOS=darwin`, `GOOS=windows` all produce binaries without CGO

## [x] Phase 4.1 — GitHub Release Artifacts
- [x] Add `.github/workflows/release.yml` triggered by tag push only (no `workflow_dispatch`)
- [x] Accept only tag formats: stable `vX.Y.Z` and prerelease `vX.Y.Z-beta.N`
- [x] Validate branch ancestry rules:
  - stable tag commit reachable from `main`
  - beta tag commit reachable from at least one `beta/*` branch
- [x] Split workflow jobs: `validate` -> `quality` -> `build` -> `publish`
- [x] Run `lint` and `test` in release workflow before build/publish
- [x] Build matrix on `ubuntu-latest` with explicit `go build` (`CGO_ENABLED=0`, `GOOS`, `GOARCH`), `fail-fast: false`:
  - `linux/amd64`, `linux/arm64`
  - `darwin/amd64`, `darwin/arm64`
  - `windows/amd64`, `windows/arm64`
- [x] Package binaries only at archive root:
  - Linux/macOS -> `.tar.gz` with `sshelob`
  - Windows -> `.zip` with `sshelob.exe`
- [x] Use deterministic artifact names: `sshelob_<tag>_<os>_<arch>.<ext>`
- [x] Generate sorted `checksums.txt` (SHA256), one line per archive
- [x] Publish GitHub Release immediately (`draft: false`) with:
  - `name = tag`
  - stable `prerelease: false`
  - beta `prerelease: true`
  - no release notes (`generate_release_notes: false`, empty body)
- [x] Upload release assets only (no duplicate Actions run artifacts)
- [x] Enforce immutability policy:
  - fail if release/tag already exists
  - treat failed tag as burned; recover with a new tag
- [x] Set least-privilege permissions (workflow read-only; publish job `contents: write`)
- [x] Add per-ref concurrency with no cancel-in-progress
- [x] Inject ldflags metadata in release builds: `Version` (with leading `v`), `Commit`, `BuildDate`
- [x] Explicitly defer artifact/checksum signing to a later phase

## [x] Phase 5 — Version & Update
- [x] `sshelob version` — prints `sshelob v0.x.x (commit abc1234, built YYYY-MM-DD)`, version baked in via ldflags at build time
- [x] `sshelob update` — fetches latest stable release from GitHub Releases, replaces binary in-place
- [x] Unit tests: version string format, update fetch/replace logic

## [ ] Phase 6 — Bug Fixes & Enhancements
- [x] **Logging**: plain-text for tunnel lifecycle events (connect, reconnect, error);
- [x] **`run all` keyword**: `sshelob run all` starts all configured tunnels
- [x] `**Config location**: `-config` flag first, fallback to `~/.config/sshelob/config.yml`; clear error showing paths checked
- [x] Update README with new features, config format and install/update instructions

## [x] Phase 6.1 — Configuration improvement
- [x] **Connections section** (breaking change): new top-level `connections:` in config with `name`, `host`, `user`, `port`, `key_path`, `use_passphrase`; tunnels reference connection by name; inline host/user/port/key removed from `TunnelDef`
- [x] **Passphrase prompt**: at startup, group tunnels by connection, prompt once per connection with `use_passphrase: true`; passphrase passed by value into `TunnelFactory(conn, passphrase) ([]*Tunnel, error)`; never stored after factory returns
- [x] Update `Config` and `TunnelDef` structs to support connections
- [x] `config.Load()` validation for connection references

## [ ] Phase 7 — Health Check
- [ ] `health.Checker` — optional TCP connect probe with configurable interval and timeout
- [ ] Integrates with `Tunnel`: health status exposed via tunnel state/log surface
- [ ] Unit tests: probe succeeds on open port, probe fails on closed port, interval respected

## [ ] Phase 8 — TUI: List & Run Views (A+B)
- [ ] `sshelob list` — renders formatted table (name, type, host, bind_addr), exits immediately (like `docker ps`)
- [ ] `sshelob run` — launches live TUI for selected tunnels
- [ ] Tunnel list view: one row per tunnel showing name, type, state badge, health status
- [ ] State badge colours: Stopped=grey, Connecting=yellow, Connected=green, Reconnecting=yellow, Error=red
- [ ] Keyboard: `↑/↓` select tunnel, `q` quit
- [ ] TUI receives state updates from tunnel engine via channel and calls `Update`

## [ ] Phase 9 — Integration & Smoke Test
- [ ] Wire config → connections → tunnel factory → tunnel engine → TUI in `main.go`; fail-fast on config error before TUI starts
- [ ] End-to-end smoke test: start tunnel to localhost SSH, verify `Connected` state appears within 5s

## [ ] Phase 10 — TUI: Log Panel (C)
- [ ] Log panel: selected tunnel's ring buffer rendered below list, auto-scrolls
- [ ] Per tunnel log shows: connection lifecycle events, errors, reconnect attempts

## [ ] Phase 11 — TUI: Actions (D)
- [ ] Keyboard actions (detail to be defined post dry-run of Phase 8–9)
- [ ] Bulk actions scoped to tunnels passed to `run`
