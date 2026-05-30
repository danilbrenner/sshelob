package config_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danilbrenner/sshelob/internal/config"
)

func TestLoad_ValidConfigWithConnections(t *testing.T) {
	yaml := `
connections:
  - name: bastion
    host: bastion.example.com
    user: alice
    port: 22
    key_path: ~/.ssh/id_ed25519
    use_passphrase: true
tunnels:
  - name: db
    type: local
    connection: bastion
    bind_addr: 127.0.0.1:5433
    dest_addr: db.internal:5432
  - name: socks
    type: dynamic
    connection: bastion
    bind_addr: 127.0.0.1:1080
`

	cfg, err := config.Load(writeTemp(t, yaml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(cfg.Connections) != 1 {
		t.Fatalf("expected 1 connection, got %d", len(cfg.Connections))
	}
	if len(cfg.Tunnels) != 2 {
		t.Fatalf("expected 2 tunnels, got %d", len(cfg.Tunnels))
	}

	if cfg.Connections[0].Name != "bastion" {
		t.Fatalf("unexpected connection name: %q", cfg.Connections[0].Name)
	}
	if !cfg.Connections[0].UsePassphrase {
		t.Fatal("expected use_passphrase to be true")
	}
	if cfg.Tunnels[0].Connection != "bastion" {
		t.Fatalf("unexpected tunnel connection: %q", cfg.Tunnels[0].Connection)
	}
}

func TestLoad_MissingConnections(t *testing.T) {
	yaml := `
tunnels:
  - name: db
    type: local
    connection: bastion
    bind_addr: :5433
    dest_addr: db:5432
`

	_, err := config.Load(writeTemp(t, yaml))
	if err == nil {
		t.Fatal("expected error")
	}
	assertErrContains(t, err, "connections")
}

func TestLoad_MissingTunnelConnection(t *testing.T) {
	yaml := `
connections:
  - name: bastion
    host: bastion.example.com
    user: alice
    port: 22
    key_path: ~/.ssh/id_ed25519
tunnels:
  - name: db
    type: local
    bind_addr: :5433
    dest_addr: db:5432
`

	_, err := config.Load(writeTemp(t, yaml))
	if err == nil {
		t.Fatal("expected error")
	}
	assertErrContains(t, err, "connection")
}

func TestLoad_UnknownConnectionReference(t *testing.T) {
	yaml := `
connections:
  - name: bastion
    host: bastion.example.com
    user: alice
    port: 22
    key_path: ~/.ssh/id_ed25519
tunnels:
  - name: db
    type: local
    connection: missing
    bind_addr: :5433
    dest_addr: db:5432
`

	_, err := config.Load(writeTemp(t, yaml))
	if err == nil {
		t.Fatal("expected error")
	}
	assertErrContains(t, err, "unknown connection")
	assertErrContains(t, err, "missing")
}

func TestLoad_DuplicateConnectionName(t *testing.T) {
	yaml := `
connections:
  - name: shared
    host: one.example.com
    user: alice
    port: 22
    key_path: ~/.ssh/id_ed25519
  - name: shared
    host: two.example.com
    user: bob
    port: 22
    key_path: ~/.ssh/id_rsa
tunnels:
  - name: db
    type: local
    connection: shared
    bind_addr: :5433
    dest_addr: db:5432
`

	_, err := config.Load(writeTemp(t, yaml))
	if err == nil {
		t.Fatal("expected error")
	}
	assertErrContains(t, err, "duplicate connection name")
}

func TestLoad_MissingConnectionKeyPath(t *testing.T) {
	yaml := `
connections:
  - name: bastion
    host: bastion.example.com
    user: alice
    port: 22
tunnels:
  - name: db
    type: local
    connection: bastion
    bind_addr: :5433
    dest_addr: db:5432
`

	_, err := config.Load(writeTemp(t, yaml))
	if err == nil {
		t.Fatal("expected error")
	}
	assertErrContains(t, err, "key_path")
}

func TestLoad_UnknownTunnelType(t *testing.T) {
	yaml := `
connections:
  - name: bastion
    host: bastion.example.com
    user: alice
    port: 22
    key_path: ~/.ssh/id_ed25519
tunnels:
  - name: bad-type
    type: socks5
    connection: bastion
    bind_addr: :1080
`

	_, err := config.Load(writeTemp(t, yaml))
	if err == nil {
		t.Fatal("expected error")
	}
	assertErrContains(t, err, "unknown tunnel type")
}

func TestLoad_LocalRequiresDestAddr(t *testing.T) {
	yaml := `
connections:
  - name: bastion
    host: bastion.example.com
    user: alice
    port: 22
    key_path: ~/.ssh/id_ed25519
tunnels:
  - name: no-dest
    type: local
    connection: bastion
    bind_addr: :5433
`

	_, err := config.Load(writeTemp(t, yaml))
	if err == nil {
		t.Fatal("expected error")
	}
	assertErrContains(t, err, "dest_addr")
}

func TestLoad_DynamicMustNotSetDestAddr(t *testing.T) {
	yaml := `
connections:
  - name: bastion
    host: bastion.example.com
    user: alice
    port: 22
    key_path: ~/.ssh/id_ed25519
tunnels:
  - name: bad-dynamic
    type: dynamic
    connection: bastion
    bind_addr: :1080
    dest_addr: should-not-be-here:80
`

	_, err := config.Load(writeTemp(t, yaml))
	if err == nil {
		t.Fatal("expected error")
	}
	assertErrContains(t, err, "dest_addr")
	assertErrContains(t, err, "dynamic")
}

func TestLoad_FileNotFound_DefaultOnly(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	_, err := config.Load("")
	if err == nil {
		t.Fatal("expected error")
	}

	fallbackPath := filepath.Join(homeDir, ".config", "sshelob", "config.yml")
	assertErrContains(t, err, "cannot read file")
	assertErrContains(t, err, fallbackPath)
}

func TestLoad_FallbackPathWhenEmpty(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	fallbackPath := filepath.Join(homeDir, ".config", "sshelob", "config.yml")
	writeConfigAtPath(t, fallbackPath, `
connections:
  - name: bastion
    host: bastion.example.com
    user: alice
    port: 22
    key_path: ~/.ssh/id_ed25519
tunnels:
  - name: fallback
    type: dynamic
    connection: bastion
    bind_addr: 127.0.0.1:1080
`)

	cfg, err := config.Load("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Tunnels) != 1 {
		t.Fatalf("expected 1 tunnel, got %d", len(cfg.Tunnels))
	}
}

func TestLoad_ErrorIncludesFieldIndex(t *testing.T) {
	yaml := `
connections:
  - name: bastion
    host: bastion.example.com
    user: alice
    port: 22
    key_path: ~/.ssh/id_ed25519
tunnels:
  - name: ok
    type: local
    connection: bastion
    bind_addr: :5001
    dest_addr: db:5432
  - name: bad
    type: unknown-type
    connection: bastion
    bind_addr: :5002
`

	_, err := config.Load(writeTemp(t, yaml))
	if err == nil {
		t.Fatal("expected error")
	}
	assertErrContains(t, err, "[1]")
}

func writeTemp(t *testing.T, content string) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "config-*.yml")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	if _, err := f.WriteString(content); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}
	_ = f.Close()
	return f.Name()
}

func assertErrContains(t *testing.T, err error, substr string) {
	t.Helper()
	if !strings.Contains(err.Error(), substr) {
		t.Errorf("expected error to contain %q, got: %v", substr, err)
	}
}

func writeConfigAtPath(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}
}
