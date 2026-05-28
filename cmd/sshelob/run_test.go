package main

import (
	"reflect"
	"strings"
	"testing"

	"github.com/danilbrenner/sshelob/internal/config"
)

func TestParseIndexes(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		want    []int
		wantErr string
	}{
		{
			name: "single argument with commas",
			args: []string{"1,2,3"},
			want: []int{1, 2, 3},
		},
		{
			name: "multiple args with spaces",
			args: []string{"1, 2", "3"},
			want: []int{1, 2, 3},
		},
		{
			name:    "missing indexes",
			args:    nil,
			wantErr: "requires at least one index",
		},
		{
			name:    "empty token",
			args:    []string{"1,,2"},
			wantErr: "comma-separated positive integers",
		},
		{
			name:    "invalid token",
			args:    []string{"1,a"},
			wantErr: "invalid index",
		},
		{
			name:    "zero index",
			args:    []string{"0"},
			wantErr: "1-based",
		},
		{
			name:    "duplicate index",
			args:    []string{"1,1"},
			wantErr: "duplicate index 1",
		},
	}

	for _, testCase := range tests {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			got, err := parseIndexes(testCase.args)
			if testCase.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", testCase.wantErr)
				}
				if !strings.Contains(err.Error(), testCase.wantErr) {
					t.Fatalf("error mismatch: got %q, want substring %q", err.Error(), testCase.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if !reflect.DeepEqual(got, testCase.want) {
				t.Fatalf("indexes mismatch: got %v, want %v", got, testCase.want)
			}
		})
	}
}

func TestSelectTunnels(t *testing.T) {
	cfg := &config.Config{
		Tunnels: []config.TunnelDef{
			{Name: "first"},
			{Name: "second"},
			{Name: "third"},
		},
	}

	t.Run("selects in requested order", func(t *testing.T) {
		selected, err := selectTunnels(cfg, []int{3, 1})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		got := []string{selected[0].Name, selected[1].Name}
		want := []string{"third", "first"}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("selection mismatch: got %v, want %v", got, want)
		}
	})

	t.Run("errors on out of range index", func(t *testing.T) {
		_, err := selectTunnels(cfg, []int{4})
		if err == nil {
			t.Fatal("expected error for out of range index")
		}
		if !strings.Contains(err.Error(), "out of range") {
			t.Fatalf("error mismatch: got %q", err.Error())
		}
	})
}
