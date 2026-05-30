package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danilbrenner/sshelob/internal/config"
)

func TestListTunnels(t *testing.T) {
	t.Run("writes indexed tunnels", func(t *testing.T) {
		cfg := &config.Config{
			Tunnels: []config.TunnelDef{
				{Name: "local-main", Type: config.TunnelTypeLocal},
				{Name: "dynamic-proxy", Type: config.TunnelTypeDynamic},
			},
		}

		var out bytes.Buffer
		if err := listTunnels(&out, cfg); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		got := out.String()
		want := "(1)local: local-main\n(2)dynamic: dynamic-proxy\n"
		if got != want {
			t.Fatalf("list output mismatch:\n got: %q\nwant: %q", got, want)
		}
	})

	t.Run("returns write failure", func(t *testing.T) {
		cfg := &config.Config{Tunnels: []config.TunnelDef{{Name: "db", Type: config.TunnelTypeLocal}}}

		err := listTunnels(errWriter{}, cfg)
		if err == nil {
			t.Fatal("expected write error")
		}
		if !strings.Contains(err.Error(), "failed to write tunnel list") {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestListCmd(t *testing.T) {
	t.Run("rejects arguments", func(t *testing.T) {
		cmd := listCmd(cliDeps{}, &cliOptions{})

		err := cmd.Args(cmd, []string{"extra"})
		if err == nil {
			t.Fatal("expected argument validation error")
		}
	})

	t.Run("lists tunnels from config", func(t *testing.T) {
		configPath := writeListConfig(t)

		var out bytes.Buffer
		cmd := listCmd(cliDeps{stdout: &out}, &cliOptions{configPath: configPath})

		if err := cmd.RunE(nil, nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		got := out.String()
		want := "(1)local: app-db\n(2)dynamic: socks\n"
		if got != want {
			t.Fatalf("list output mismatch:\n got: %q\nwant: %q", got, want)
		}
	})

	t.Run("returns config load error", func(t *testing.T) {
		missingPath := filepath.Join(t.TempDir(), "missing.yml")

		var out bytes.Buffer
		cmd := listCmd(cliDeps{stdout: &out}, &cliOptions{configPath: missingPath})

		err := cmd.RunE(nil, nil)
		if err == nil {
			t.Fatal("expected load error")
		}
		if !strings.Contains(err.Error(), "cannot read file") {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func writeListConfig(t *testing.T) string {
	t.Helper()

	content := `tunnels:
  - name: app-db
    type: local
    connection: shared
    bind_addr: 127.0.0.1:5433
    dest_addr: db.internal:5432
  - name: socks
    type: dynamic
    connection: shared
    bind_addr: 127.0.0.1:1080
connections:
  - name: shared
    host: bastion.internal
    user: alice
    port: 22
    key_path: ~/.ssh/id_ed25519
`

	configPath := filepath.Join(t.TempDir(), "config.yml")
	if err := os.WriteFile(configPath, []byte(content), 0o600); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	return configPath
}

type errWriter struct{}

func (errWriter) Write(_ []byte) (int, error) {
	return 0, errors.New("boom")
}
