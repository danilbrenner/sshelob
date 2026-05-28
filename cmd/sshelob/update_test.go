package main

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestUpdateCmd(t *testing.T) {
	t.Run("rejects arguments", func(t *testing.T) {
		cmd := updateCmd(context.Background(), cliDeps{})

		err := cmd.Args(cmd, []string{"extra"})
		if err == nil {
			t.Fatal("expected argument validation error")
		}
	})

	t.Run("returns update error", func(t *testing.T) {
		var out bytes.Buffer
		cmd := updateCmd(context.Background(), cliDeps{
			stdout:     &out,
			apiBaseURL: "://invalid-base-url",
			repo:       "danilbrenner/sshelob",
		})

		err := cmd.RunE(nil, nil)
		if err == nil {
			t.Fatal("expected update error")
		}
		if !strings.Contains(err.Error(), "fetch latest release metadata") {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.Len() != 0 {
			t.Fatalf("expected no output, got %q", out.String())
		}
	})
}
