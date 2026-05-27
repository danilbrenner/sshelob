# sshelob MVP Technical Requirements

## 1. Scope
This document defines the MVP technical requirements for **sshelob**, a terminal UI SSH tunnel manager for individual developers and SREs managing local tunnels.

## 2. MVP Goals
1. Manage multiple SSH tunnels from a single TUI.
2. Support tunnel lifecycle actions per tunnel and in bulk.
3. Provide reliable auto-reconnect in pure Go SSH mode.
4. Surface operational status and logs in the TUI.

## 3. Target User
- Individual developer/SRE operating tunnels on their own machine.

## 4. Platform Support
- macOS, Linux, Windows (MVP requirement).

## 5. Configuration Requirements
### 5.1 Format and Location
- YAML only in MVP.
- Single config file (multi-tunnel + connection definitions).
- Default location: `-config` flag (explicit), then fallback to `~/.config/sshelob/config.yml`.
- Clear error message showing paths checked if config not found.

### 5.2 Structure: Connections + Tunnels
Config is split into two sections:
- **`connections`**: Named SSH connection definitions (host, user, port, key_path, use_passphrase).
- **`tunnels`**: Tunnel definitions that reference a connection by name.

Each tunnel definition must specify:
- `connection`: reference to a named connection
- `name`: unique tunnel name
- Tunnel type: local (`-L`), remote (`-R`), dynamic (`-D`)
- Bind/listen and destination fields required by type
- Optional health check block (see section 8)

### 5.3 Validation
- Startup must fail-fast on invalid config.
- Errors must identify exact field/path causing the failure.
- No silent skipping of invalid tunnel or connection entries.
- All connection references must resolve to defined connections.

### 5.4 Passphrase Handling
- `use_passphrase: true` in a connection triggers prompt at startup.
- Passphrase prompted once per connection, never stored to disk or held in memory after tunnel creation.
- Passphrase passed by value into tunnel factory, goes out of scope immediately.

## 6. Runtime / Tunnel Engine
- MVP engine: **Pure Go SSH first** (no autossh dependency for MVP behavior).
- Auto-reconnect is mandatory for all running tunnels.
- Tunnel factory pattern: one factory call per connection, grouped by passphrase needs.

### 6.1 Reconnect Policy (Default)
- Initial delay: 1s
- Backoff multiplier: 2x
- Maximum delay: 30s
- Retry count: unlimited until user manually stops the tunnel

### 6.2 CLI Mode (Pre-TUI)
- MVP must provide non-TUI CLI modes for basic tunnel control.
- `sshelob list` — prints formatted table of all configured tunnels: `name | type | host | bind_addr`.
- `sshelob run <indexes>` — starts tunnels by comma-separated 1-based indexes (example: `sshelob run 1,2,3`).
- `sshelob run all` — starts all configured tunnels.
- `run` mode keeps tunnels running until user stops the process (SIGINT).
- At startup, if any connection has `use_passphrase: true`, prompt user for passphrase(s) before tunnels start.

## 7. TUI Functional Requirements

### 7.0 Entry Points
- `sshelob list` — formatted table view (like `docker ps`), exits after display.
- `sshelob run <indexes>` or `sshelob run all` — launches live TUI for selected running tunnels.

### 7.1 Required Actions
- Per tunnel (in `run` mode): `start`, `stop`, `restart`, `status` (post dry-run).
- Bulk (in `run` mode): `start all`, `stop all` scoped to the tunnels passed to `run`.

### 7.2 Status States
At minimum, TUI must represent:
- Stopped
- Connecting
- Connected
- Reconnecting
- Error

### 7.3 Log Surface
Per tunnel, the TUI must show:
- Connection lifecycle events
- Errors
- Reconnect attempts

Implementation requirement:
- In-memory ring buffer per tunnel (bounded).

## 8. Health Check Requirements
- Health checks are optional and configured per tunnel.
- Probe type in MVP: TCP connect probe only.
- Configurable interval and timeout.
- Health status must be visible in TUI status area.

## 9. Authentication Requirements
- MVP auth method: SSH key auth only (via `connections` section).
- Password authentication is out of MVP scope.
- Key passphrase support via `use_passphrase: true` in connection config.
- Passphrase prompted at startup before tunnels start, never stored.
- Optional SSH agent support (future enhancement).

## 10. Stop/Shutdown Behavior
- Tunnel stop in MVP is immediate (no graceful drain requirement).

## 11. Reliability Acceptance Criteria
For stable network conditions:
- Tunnel reaches `Connected` within 5 seconds after start.
- After disconnect, tunnel auto-recovers within 35 seconds.

## 12. GitHub Release Artifacts

### 12.1 Trigger and Tag Rules
- Release workflow is separate from CI (`release.yml`) and runs on tag push only.
- Accepted stable tags: exact `vX.Y.Z`.
- Accepted prerelease tags: exact `vX.Y.Z-beta.N`.
- No other tag shapes trigger release builds.

### 12.2 Branch Policy
- Stable tag commit must be reachable from `main`.
- Beta tag commit must be reachable from at least one `beta/*` branch.
- Beta tags are allowed even if the commit is also reachable from `main`.

### 12.3 Validation and Quality Gates
- Jobs are split as: `validate` -> `quality` -> `build` -> `publish`.
- `validate` enforces tag format and branch ancestry rules.
- `quality` runs both lint and tests before any publishing.
- Release publish must not run unless prior jobs pass.

### 12.4 Build Matrix and Packaging
- Build matrix targets:
  - `linux/amd64`, `linux/arm64`
  - `darwin/amd64`, `darwin/arm64`
  - `windows/amd64`, `windows/arm64`
- Matrix uses explicit `go build` with `GOOS/GOARCH` (not Makefile cross-compile target).
- Cross-compilation runs on `ubuntu-latest`.
- Build uses `CGO_ENABLED=0`.
- Matrix strategy uses `fail-fast: false`.
- Package format:
  - Linux/macOS: `.tar.gz`
  - Windows: `.zip`
- Archive contents are binary-only at archive root:
  - `sshelob` on Linux/macOS
  - `sshelob.exe` on Windows

### 12.5 Artifact Naming and Checksums
- Artifact naming is deterministic: `sshelob_<tag>_<os>_<arch>.<ext>`.
- Generate a single `checksums.txt` with SHA256 for every archive.
- `checksums.txt` contains one line per archive and is sorted by filename.

### 12.6 Release Publishing Policy
- Publish GitHub Release assets directly (no duplicate Actions run artifacts).
- Release `name` equals tag.
- Stable releases are published with `prerelease: false`.
- Beta releases are published with `prerelease: true`.
- Release notes are disabled (`generate_release_notes: false`) and body is empty.
- Releases publish immediately (`draft: false`).

### 12.7 Immutability and Failure Handling
- Tags and releases are immutable.
- If a release run fails at any stage, the tag is treated as burned and must not be reused.
- Recovery is done by fixing forward with a new tag:
  - Beta: increment beta number (`-beta.N+1`)
  - Stable: next patch version
- If a release for a tag already exists, workflow fails instead of overwriting assets.

### 12.8 Security and Workflow Controls
- Use least-privilege GitHub Actions permissions:
  - workflow default read-only
  - publish job `contents: write` only
- Use per-ref concurrency and do not cancel in-progress release runs.

### 12.9 Build Metadata
- Release builds inject ldflags for `Version`, `Commit`, and `BuildDate`.
- Version value includes the leading `v` (example: `v0.1.0`).
- Archive/checksum signing is out of scope for this phase.

## 13. CLI & Version
- `sshelob version` — prints version and build info (e.g. `v0.1.0 (commit abc1234, built 2026-05-27)`).
- `sshelob update` — fetches and installs latest stable release from GitHub Releases.

## 14. Logging
- Tunnel lifecycle events (connect, reconnect, error) printed as plain text to stdout.
- Internal/fatal errors logged via slog to stderr.
- TUI and non-TUI modes handled cleanly (no slog pollution into TUI).

## 15. Deliverable Boundary for This Spec
This is an MVP technical requirements document only.  
Architecture design docs are intentionally deferred to a later phase.
