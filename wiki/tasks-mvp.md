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

## [ ] Phase 4 — Build & Test Pipeline
- [ ] `Makefile` targets: `build`, `test`, `lint`, `cross-compile`
- [ ] GitHub Actions workflow: lint + test on push/PR
- [ ] `golangci-lint` config (`.golangci.yml`)
- [ ] Cross-compile check: `GOOS=linux`, `GOOS=darwin`, `GOOS=windows` all produce binaries without CGO

## [ ] Phase 5 — Version & Update
- [ ] `sshelob version` — prints `sshelob v0.x.x (commit abc1234, built YYYY-MM-DD)`, version baked in via ldflags at build time
- [ ] `sshelob update` — fetches latest stable release from GitHub Releases, replaces binary in-place
- [ ] Unit tests: version string format, update fetch/replace logic

## [ ] Phase 6 — Bug Fixes & Enhancements
- [ ] **Logging**: plain-text for tunnel lifecycle events (connect, reconnect, error); slog reserved for internal/fatal errors only
- [ ] **`run all` keyword**: `sshelob run all` starts all configured tunnels
- [ ] **Config location**: `-config` flag first, fallback to `~/.config/sshelob/config.yml`; clear error showing paths checked
- [ ] **Connections section** (breaking change): new top-level `connections:` in config with `name`, `host`, `user`, `port`, `key_path`, `use_passphrase`; tunnels reference connection by name; inline host/user/port/key removed from `TunnelDef`
- [ ] **Passphrase prompt**: at startup, group tunnels by connection, prompt once per connection with `use_passphrase: true`; passphrase passed by value into `TunnelFactory(conn, passphrase) ([]*Tunnel, error)`; never stored after factory returns
- [ ] Update `Config` and `TunnelDef` structs to support connections
- [ ] `config.Load()` validation for connection references
- [ ] Unit tests: `run all`, config location fallback, connections validation, factory passphrase scoping

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
