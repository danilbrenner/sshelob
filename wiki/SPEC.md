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
- Single config file (multi-tunnel definitions).

### 5.2 Validation
- Startup must fail-fast on invalid config.
- Errors must identify exact field/path causing the failure.
- No silent skipping of invalid tunnel entries.

### 5.3 Required Tunnel Definition Capabilities
Each tunnel definition must support:
- Tunnel type: local (`-L`), remote (`-R`), dynamic (`-D`)
- SSH target host/user/port
- Bind/listen and destination fields required by type
- Optional health check block (see section 8)
- Optional authentication key path/options (see section 9)

## 6. Runtime / Tunnel Engine
- MVP engine: **Pure Go SSH first** (no autossh dependency for MVP behavior).
- Auto-reconnect is mandatory for all running tunnels.

### 6.1 Reconnect Policy (Default)
- Initial delay: 1s
- Backoff multiplier: 2x
- Maximum delay: 30s
- Retry count: unlimited until user manually stops the tunnel

## 7. TUI Functional Requirements
### 7.1 Required Actions
- Per tunnel: `start`, `stop`, `restart`, `status`
- Bulk: `start all`, `stop all`

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
- MVP auth method: SSH key auth only.
- Password authentication is out of MVP scope.
- Key auth support includes private key path and expected key-loading behavior needed by runtime.

## 10. Stop/Shutdown Behavior
- Tunnel stop in MVP is immediate (no graceful drain requirement).

## 11. Reliability Acceptance Criteria
For stable network conditions:
- Tunnel reaches `Connected` within 5 seconds after start.
- After disconnect, tunnel auto-recovers within 35 seconds.

## 12. Explicit Non-Goals (Post-MVP)
- Team/multi-user shared management features
- Password authentication
- Shell/Fig completion support
- Advanced metrics/observability beyond MVP log+status surface
- Architecture-level extensibility documentation (tracked separately)

## 13. Deliverable Boundary for This Spec
This is an MVP technical requirements document only.  
Architecture design docs are intentionally deferred to a later phase.
