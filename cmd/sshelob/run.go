package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"

	"github.com/danilbrenner/sshelob/internal/config"
	"github.com/danilbrenner/sshelob/internal/tunnel"
	"github.com/spf13/cobra"
)

func runCmd(ctx context.Context, opts *cliOptions) *cobra.Command {
	return &cobra.Command{
		Use:     "run <indexes|all>",
		Short:   "Run selected tunnels by 1-based indexes or all",
		Args:    cobra.MinimumNArgs(1),
		Example: "sshelob run 1,2,3\nsshelob run all",
		RunE: func(_ *cobra.Command, args []string) error {
			cfg, err := config.Load(opts.configPath)
			if err != nil {
				return err
			}

			selection, err := parseRunSelection(args)
			if err != nil {
				return err
			}

			selected, err := selectTunnels(cfg, selection)
			if err != nil {
				return err
			}

			return runTunnels(ctx, selected)
		},
	}
}

type runSelection struct {
	all     bool
	indexes []int
}

func parseRunSelection(args []string) (runSelection, error) {
	if len(args) == 0 {
		return runSelection{}, fmt.Errorf("run command requires indexes or 'all' (examples: sshelob run 1,2,3 or sshelob run all)")
	}

	if len(args) == 1 && strings.EqualFold(strings.TrimSpace(args[0]), "all") {
		return runSelection{all: true}, nil
	}

	joined := strings.Join(args, ",")
	parts := strings.Split(joined, ",")
	indexes := make([]int, 0, len(parts))
	seen := make(map[int]struct{}, len(parts))

	for _, part := range parts {
		value := strings.TrimSpace(part)
		if value == "" {
			return runSelection{}, fmt.Errorf("indexes must be comma-separated positive integers")
		}
		if strings.EqualFold(value, "all") {
			return runSelection{}, fmt.Errorf("'all' cannot be combined with indexes")
		}

		index, err := strconv.Atoi(value)
		if err != nil {
			return runSelection{}, fmt.Errorf("invalid index %q: %w", value, err)
		}
		if index < 1 {
			return runSelection{}, fmt.Errorf("invalid index %d: indexes are 1-based", index)
		}
		if _, exists := seen[index]; exists {
			return runSelection{}, fmt.Errorf("duplicate index %d", index)
		}

		seen[index] = struct{}{}
		indexes = append(indexes, index)
	}

	return runSelection{indexes: indexes}, nil
}

func selectTunnels(cfg *config.Config, selection runSelection) ([]config.TunnelDef, error) {
	if selection.all {
		selected := make([]config.TunnelDef, len(cfg.Tunnels))
		copy(selected, cfg.Tunnels)
		return selected, nil
	}

	indexes := selection.indexes
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
		tunnels = append(tunnels, tunnel.NewTunnel(tunnelDef, tunnel.WithEventWriter(os.Stdout)))
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

	if _, err := fmt.Fprintf(os.Stdout, "started %d tunnel(s)\n", len(tunnels)); err != nil {
		return fmt.Errorf("write start status: %w", err)
	}
	<-runCtx.Done()
	if _, err := fmt.Fprintln(os.Stdout, "stopping tunnels"); err != nil {
		return fmt.Errorf("write stop status: %w", err)
	}

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
