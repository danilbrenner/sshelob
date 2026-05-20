# sshelob MVP — Implementation Checklist

## [ ] Phase 1 — Project Scaffold
- [ ] `go mod init` with module name, add `golang.org/x/crypto`, `gopkg.in/yaml.v3`, `github.com/charmbracelet/bubbletea` deps
- [ ] Create directory layout: `cmd/sshelob/`, `internal/config/`, `internal/tunnel/`, `internal/health/`, `internal/tui/`
- [ ] `cmd/sshelob/main.go` — entry point: load config, init TUI, run

## [ ] Phase 2 — Config
- [ ] Define `Config` and `TunnelDef` structs (type, host, user, port, bind/dest, key path, health block)
- [ ] `config.Load(path string) (*Config, error)` — reads YAML, validates required fields, returns error with field path on failure
- [ ] Unit tests: valid config parses correctly, missing required field returns descriptive error, unknown tunnel type returns error

## [ ] Phase 3 — Tunnel Engine
- [ ] `TunnelState` type with states: `Stopped`, `Connecting`, `Connected`, `Reconnecting`, `Error`
- [ ] `Tunnel` struct wrapping a config entry + state + log ring buffer (fixed cap, e.g. 200 lines)
- [ ] `Tunnel.Start()` — dials SSH with `golang.org/x/crypto/ssh`, opens port forward (local/remote/dynamic), blocks until disconnect, publishes state transitions
- [ ] `Tunnel.Stop()` — cancels context, transitions to `Stopped`
- [ ] Auto-reconnect loop: 1s initial delay, 2× backoff, max 30s, unlimited retries, stops only on `Stop()`
- [ ] Unit tests: state transitions on start/stop, backoff timing, ring buffer wraps correctly

## [ ] Phase 4 — Health Check
- [ ] `health.Checker` — optional TCP connect probe with configurable interval and timeout
- [ ] Integrates with `Tunnel`: health status exposed via tunnel state/log surface
- [ ] Unit tests: probe succeeds on open port, probe fails on closed port, interval respected

## [ ] Phase 5 — TUI
- [ ] `tui.Model` implements Bubble Tea `Model` interface
- [ ] Tunnel list view: one row per tunnel showing name, type, state badge, health status
- [ ] Log panel: selected tunnel's ring buffer rendered below list, auto-scrolls
- [ ] Keyboard bindings: `↑/↓` select tunnel, `s` start, `x` stop, `r` restart, `S` start-all, `X` stop-all, `q` quit
- [ ] State badge colours: Stopped=grey, Connecting=yellow, Connected=green, Reconnecting=yellow, Error=red
- [ ] TUI receives state/log updates from tunnel engine via channel and calls `Update`

## [ ] Phase 6 — Integration & Build
- [ ] Wire config → tunnel engine → TUI in `main.go`; fail-fast on config error before TUI starts
- [ ] `Makefile` targets: `build`, `test`, `lint` (golangci-lint)
- [ ] Cross-compile check: `GOOS=linux`, `GOOS=darwin`, `GOOS=windows` all produce binaries without CGO
- [ ] End-to-end smoke test: start tunnel to localhost SSH, verify `Connected` state appears in TUI within 5s
