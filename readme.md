# sshelob

CLI-first SSH tunnel manager for local, remote, and dynamic forwards.

![Status](https://img.shields.io/badge/Status-Early%20Development-blue)
![Go](https://img.shields.io/badge/Go-1.25+-00ADD8)

## Features

- Manage multiple SSH tunnels from one config file
- Reuse named SSH connections across multiple tunnels
- Tunnel types: `local`, `remote`, `dynamic`
- `sshelob run all` or `sshelob run 1,2,3` to start selected tunnels
- Optional passphrase prompts for encrypted SSH private keys (once per connection)
- Plain-text lifecycle logging to stdout (connect, reconnect, error)
- Exponential auto-reconnect (1s initial, 2x backoff, max 30s, retries until stopped)
- Commands: `list`, `run`, `version`, `update`

## Installation

### Option 1: Install with Go

```bash
go install github.com/danilbrenner/sshelob/cmd/sshelob@latest
```

### Option 2: Build from source

```bash
git clone https://github.com/danilbrenner/sshelob.git
cd sshelob
make build
./build/sshelob version
```

## Configuration

By default, sshelob reads:

- `~/.config/sshelob/config.yml`

You can override this with:

```bash
sshelob --config /path/to/config.yml list
```

### Config format

```yaml
connections:
  - name: bastion-main
    host: bastion.example.com
    user: alice
    port: 22
    key_path: ~/.ssh/id_ed25519
    use_passphrase: false

tunnels:
  - name: postgres-local
    type: local
    connection: bastion-main
    bind_addr: "127.0.0.1:5433"
    dest_addr: "db.internal:5432"
    health_check:
      interval: 10s
      timeout: 3s

  - name: socks-proxy
    type: dynamic
    connection: bastion-main
    bind_addr: "127.0.0.1:1080"
```

Field notes:

- `connections` must include at least one named SSH connection
- connection fields: `name`, `host`, `user`, `port`, `key_path`, and optional `use_passphrase`
- each tunnel must set `connection` to a valid connection name
- `type` must be one of `local`, `remote`, or `dynamic`
- `dest_addr` is required for `local` and `remote`; it must be omitted for `dynamic`
- `bind_addr` is the local bind for `local`/`dynamic` and the remote bind for `remote`

If `use_passphrase: true` is set on a connection, `sshelob run ...` prompts for that connection passphrase before starting its tunnels.

## Usage

```bash
# Show configured tunnels
sshelob list

# Run specific tunnels by 1-based index
sshelob run 1,2

# Run all configured tunnels
sshelob run all

# Print build/version metadata
sshelob version
```

## Update

Upgrade to the latest stable GitHub release:

```bash
sshelob update
```

On success, sshelob prints the installed tag (for example: `updated sshelob to v0.1.0`).
