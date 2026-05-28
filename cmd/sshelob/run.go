package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"

	"log/slog"

	"github.com/danilbrenner/sshelob/internal/config"
	"github.com/danilbrenner/sshelob/internal/tunnel"
	"github.com/spf13/cobra"
)

func runCmd(ctx context.Context, opts *cliOptions) *cobra.Command {
	return &cobra.Command{
		Use:     "run <indexes>",
		Short:   "Run selected tunnels by 1-based indexes",
		Args:    cobra.MinimumNArgs(1),
		Example: "sshelob run 1,2,3",
		RunE: func(_ *cobra.Command, args []string) error {
			cfg, err := config.Load(opts.configPath)
			if err != nil {
				return err
			}

			indexes, err := parseIndexes(args)
			if err != nil {
				return err
			}

			selected, err := selectTunnels(cfg, indexes)
			if err != nil {
				return err
			}

			return runTunnels(ctx, selected)
		},
	}
}

func parseIndexes(args []string) ([]int, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("run command requires at least one index (example: sshelob run 1,2,3)")
	}
	joined := strings.Join(args, ",")
	parts := strings.Split(joined, ",")
	indexes := make([]int, 0, len(parts))
	seen := make(map[int]struct{}, len(parts))

	for _, part := range parts {
		value := strings.TrimSpace(part)
		if value == "" {
			return nil, fmt.Errorf("indexes must be comma-separated positive integers")
		}

		index, err := strconv.Atoi(value)
		if err != nil {
			return nil, fmt.Errorf("invalid index %q: %w", value, err)
		}
		if index < 1 {
			return nil, fmt.Errorf("invalid index %d: indexes are 1-based", index)
		}
		if _, exists := seen[index]; exists {
			return nil, fmt.Errorf("duplicate index %d", index)
		}

		seen[index] = struct{}{}
		indexes = append(indexes, index)
	}

	return indexes, nil
}

func selectTunnels(cfg *config.Config, indexes []int) ([]config.TunnelDef, error) {
	selected := make([]config.TunnelDef, 0, len(indexes))
	for _, index := range indexes {
		if index > len(cfg.Tunnels) {
			return nil, fmt.Errorf("index %d is out of range (configured tunnels: %d)", index, len(cfg.Tunnels))
		}
		selected = append(selected, cfg.Tunnels[index-1])
	}
	return selected, nil
}

func runTunnels(ctx context.Context, defs []config.TunnelDef) error {
	runCtx, stopSignal := signal.NotifyContext(ctx, os.Interrupt)
	defer stopSignal()

	tunnels := make([]*tunnel.Tunnel, 0, len(defs))
	for _, tunnelDef := range defs {
		tunnels = append(tunnels, tunnel.NewTunnel(tunnelDef))
	}

	errCh := make(chan error, len(tunnels))
	var wg sync.WaitGroup

	for i, tnl := range tunnels {
		tnl := tnl
		name := defs[i].Name
		wg.Add(1)
		go func(name string, tnl *tunnel.Tunnel) {
			defer wg.Done()
			if err := tnl.Start(); err != nil {
				errCh <- fmt.Errorf("tunnel %q failed: %w", name, err)
				stopSignal()
			}
		}(name, tnl)
	}

	slog.Info("tunnels started", "count", len(tunnels))
	<-runCtx.Done()
	slog.Info("stopping tunnels")

	for i, tnl := range tunnels {
		if err := tnl.Stop(); err != nil {
			errCh <- fmt.Errorf("tunnel %q stop failed: %w", defs[i].Name, err)
		}
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		if err != nil {
			return err
		}
	}
	return nil
}
