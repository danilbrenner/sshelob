package config_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danilbrenner/sshelob/internal/config"
)

// writeTemp writes content to a temp YAML file and returns its path.
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

func TestLoad_ValidLocalTunnel(t *testing.T) {
	yaml := `
tunnels:
  - name: db-tunnel
    type: local
    host: bastion.example.com
    user: alice
    port: 22
    bind_addr: "127.0.0.1:5433"
    dest_addr: "db.internal:5432"
    key_path: ~/.ssh/id_ed25519
    health_check:
      interval: 10s
      timeout: 3s
`
	cfg, err := config.Load(writeTemp(t, yaml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Tunnels) != 1 {
		t.Fatalf("expected 1 tunnel, got %d", len(cfg.Tunnels))
	}
	tun := cfg.Tunnels[0]
	if tun.Name != "db-tunnel" {
		t.Errorf("name: got %q, want %q", tun.Name, "db-tunnel")
	}
	if tun.Type != config.TunnelTypeLocal {
		t.Errorf("type: got %q, want %q", tun.Type, config.TunnelTypeLocal)
	}
	if tun.HealthCheck == nil {
		t.Fatal("expected health_check to be set")
	}
	if tun.HealthCheck.Interval != "10s" {
		t.Errorf("health_check.interval: got %q, want %q", tun.HealthCheck.Interval, "10s")
	}
}

func TestLoad_ValidRemoteTunnel(t *testing.T) {
	yaml := `
tunnels:
  - name: remote-fwd
    type: remote
    host: jump.example.com
    user: bob
    port: 22
    bind_addr: "0.0.0.0:8080"
    dest_addr: "localhost:8080"
`
	cfg, err := config.Load(writeTemp(t, yaml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Tunnels[0].Type != config.TunnelTypeRemote {
		t.Errorf("expected remote type")
	}
}

func TestLoad_ValidDynamicTunnel(t *testing.T) {
	yaml := `
tunnels:
  - name: socks-proxy
    type: dynamic
    host: proxy.example.com
    user: carol
    port: 22
    bind_addr: "127.0.0.1:1080"
`
	cfg, err := config.Load(writeTemp(t, yaml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Tunnels[0].Type != config.TunnelTypeDynamic {
		t.Errorf("expected dynamic type")
	}
}

func TestLoad_MultipleTunnels(t *testing.T) {
	yaml := `
tunnels:
  - name: tunnel-1
    type: local
    host: host1.example.com
    user: user1
    port: 22
    bind_addr: ":5001"
    dest_addr: "svc1:80"
  - name: tunnel-2
    type: dynamic
    host: host2.example.com
    user: user2
    port: 22
    bind_addr: ":1080"
`
	cfg, err := config.Load(writeTemp(t, yaml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Tunnels) != 2 {
		t.Fatalf("expected 2 tunnels, got %d", len(cfg.Tunnels))
	}
}

func TestLoad_FileNotFound(t *testing.T) {
	missingPath := filepath.Join(t.TempDir(), "nonexistent.yml")
	_, err := config.Load(missingPath)
	if err == nil {
		t.Fatal("expected error for missing file")
	}
	assertErrContains(t, err, "cannot read file")
	assertErrContains(t, err, missingPath)
}

func TestLoad_FileNotFound_DefaultOnly(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	_, err := config.Load("")
	if err == nil {
		t.Fatal("expected error for missing default config")
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
tunnels:
  - name: fallback
    type: dynamic
    host: bastion.example.com
    user: alice
    port: 22
    bind_addr: "127.0.0.1:1080"
`)

	cfg, err := config.Load("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Tunnels) != 1 {
		t.Fatalf("expected 1 tunnel, got %d", len(cfg.Tunnels))
	}
}

func TestLoad_ExplicitMissingPathReturnsError(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	fallbackPath := filepath.Join(homeDir, ".config", "sshelob", "config.yml")
	writeConfigAtPath(t, fallbackPath, `
tunnels:
  - name: fallback
    type: local
    host: bastion.example.com
    user: alice
    port: 22
    bind_addr: ":5433"
    dest_addr: "db:5432"
`)

	missingPath := filepath.Join(t.TempDir(), "missing.yml")
	_, err := config.Load(missingPath)
	if err != nil {
		assertErrContains(t, err, "cannot read file")
		assertErrContains(t, err, missingPath)
		return
	}
	t.Fatal("expected error for missing explicit config path")
}

func TestLoad_EmptyTunnels(t *testing.T) {
	yaml := `tunnels: []`
	_, err := config.Load(writeTemp(t, yaml))
	if err == nil {
		t.Fatal("expected error for empty tunnels list")
	}
	assertErrContains(t, err, "tunnels")
}

func TestLoad_MissingName(t *testing.T) {
	yaml := `
tunnels:
  - type: local
    host: bastion.example.com
    user: alice
    port: 22
    bind_addr: ":5433"
    dest_addr: "db:5432"
`
	_, err := config.Load(writeTemp(t, yaml))
	if err == nil {
		t.Fatal("expected error for missing name")
	}
	assertErrContains(t, err, "name")
}

func TestLoad_MissingType(t *testing.T) {
	yaml := `
tunnels:
  - name: broken
    host: bastion.example.com
    user: alice
    port: 22
    bind_addr: ":5433"
    dest_addr: "db:5432"
`
	_, err := config.Load(writeTemp(t, yaml))
	if err == nil {
		t.Fatal("expected error for missing type")
	}
	assertErrContains(t, err, "type")
}

func TestLoad_UnknownTunnelType(t *testing.T) {
	yaml := `
tunnels:
  - name: bad-type
    type: socks5
    host: bastion.example.com
    user: alice
    port: 22
    bind_addr: ":1080"
`
	_, err := config.Load(writeTemp(t, yaml))
	if err == nil {
		t.Fatal("expected error for unknown tunnel type")
	}
	assertErrContains(t, err, "type")
	assertErrContains(t, err, "socks5")
}

func TestLoad_MissingHost(t *testing.T) {
	yaml := `
tunnels:
  - name: no-host
    type: local
    user: alice
    port: 22
    bind_addr: ":5433"
    dest_addr: "db:5432"
`
	_, err := config.Load(writeTemp(t, yaml))
	if err == nil {
		t.Fatal("expected error for missing host")
	}
	assertErrContains(t, err, "host")
}

func TestLoad_MissingUser(t *testing.T) {
	yaml := `
tunnels:
  - name: no-user
    type: local
    host: bastion.example.com
    port: 22
    bind_addr: ":5433"
    dest_addr: "db:5432"
`
	_, err := config.Load(writeTemp(t, yaml))
	if err == nil {
		t.Fatal("expected error for missing user")
	}
	assertErrContains(t, err, "user")
}

func TestLoad_MissingPort(t *testing.T) {
	yaml := `
tunnels:
  - name: no-port
    type: local
    host: bastion.example.com
    user: alice
    bind_addr: ":5433"
    dest_addr: "db:5432"
`
	_, err := config.Load(writeTemp(t, yaml))
	if err == nil {
		t.Fatal("expected error for missing port")
	}
	assertErrContains(t, err, "port")
}

func TestLoad_MissingBindAddr(t *testing.T) {
	yaml := `
tunnels:
  - name: no-bind
    type: local
    host: bastion.example.com
    user: alice
    port: 22
    dest_addr: "db:5432"
`
	_, err := config.Load(writeTemp(t, yaml))
	if err == nil {
		t.Fatal("expected error for missing bind_addr")
	}
	assertErrContains(t, err, "bind_addr")
}

func TestLoad_MissingDestAddrForLocal(t *testing.T) {
	yaml := `
tunnels:
  - name: no-dest
    type: local
    host: bastion.example.com
    user: alice
    port: 22
    bind_addr: ":5433"
`
	_, err := config.Load(writeTemp(t, yaml))
	if err == nil {
		t.Fatal("expected error for missing dest_addr in local tunnel")
	}
	assertErrContains(t, err, "dest_addr")
}

func TestLoad_MissingDestAddrForRemote(t *testing.T) {
	yaml := `
tunnels:
  - name: no-dest-remote
    type: remote
    host: bastion.example.com
    user: alice
    port: 22
    bind_addr: ":8080"
`
	_, err := config.Load(writeTemp(t, yaml))
	if err == nil {
		t.Fatal("expected error for missing dest_addr in remote tunnel")
	}
	assertErrContains(t, err, "dest_addr")
}

func TestLoad_DynamicWithDestAddrIsInvalid(t *testing.T) {
	yaml := `
tunnels:
  - name: dynamic-bad
    type: dynamic
    host: proxy.example.com
    user: carol
    port: 22
    bind_addr: "127.0.0.1:1080"
    dest_addr: "should-not-be-here:80"
`
	_, err := config.Load(writeTemp(t, yaml))
	if err == nil {
		t.Fatal("expected error for dynamic tunnel with dest_addr set")
	}
	assertErrContains(t, err, "dest_addr")
	assertErrContains(t, err, "dynamic")
}

func TestLoad_DynamicDoesNotRequireDestAddr(t *testing.T) {
	yaml := `
tunnels:
  - name: dynamic-ok
    type: dynamic
    host: proxy.example.com
    user: carol
    port: 22
    bind_addr: "127.0.0.1:1080"
`
	_, err := config.Load(writeTemp(t, yaml))
	if err != nil {
		t.Fatalf("unexpected error for dynamic tunnel without dest_addr: %v", err)
	}
}

func TestLoad_ErrorIncludesFieldIndex(t *testing.T) {
	yaml := `
tunnels:
  - name: ok
    type: local
    host: bastion.example.com
    user: alice
    port: 22
    bind_addr: ":5001"
    dest_addr: "db:5432"
  - name: bad
    type: unknown-type
    host: bastion.example.com
    user: alice
    port: 22
    bind_addr: ":5002"
`
	_, err := config.Load(writeTemp(t, yaml))
	if err == nil {
		t.Fatal("expected error")
	}
	// Error must reference the failing index (1)
	assertErrContains(t, err, "[1]")
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
