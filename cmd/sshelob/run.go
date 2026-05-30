package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"

	"github.com/danilbrenner/sshelob/internal/config"
	"github.com/danilbrenner/sshelob/internal/tunnel"
	"github.com/spf13/cobra"
	"golang.org/x/term"
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

			tunnels, err := buildTunnelsWithConnections(cfg, selected, os.Stdin, os.Stdout)
			if err != nil {
				return err
			}

			return runTunnels(ctx, tunnels)
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

type namedTunnel struct {
	name   string
	tunnel *tunnel.Tunnel
}

func buildTunnelsWithConnections(cfg *config.Config, defs []config.TunnelDef, in io.Reader, out io.Writer) ([]namedTunnel, error) {
	orderedConnections, groupedTunnelDefs, err := groupTunnelsByConnection(cfg, defs)
	if err != nil {
		return nil, err
	}

	result := make([]namedTunnel, 0, len(defs))
	inputReader := bufio.NewReader(in)

	for _, connectionDef := range orderedConnections {
		passphrase := ""
		if connectionDef.UsePassphrase {
			passphrase, err = promptPassphrase(in, inputReader, out, connectionDef.Name)
			if err != nil {
				return nil, err
			}
		}

		tunnelDefs := groupedTunnelDefs[connectionDef.Name]
		created, factoryErr := tunnel.TunnelFactory(connectionDef, passphrase, tunnelDefs, tunnel.WithEventWriter(os.Stdout))
		passphrase = ""
		if factoryErr != nil {
			return nil, fmt.Errorf("connection %q: %w", connectionDef.Name, factoryErr)
		}

		for i, createdTunnel := range created {
			result = append(result, namedTunnel{name: tunnelDefs[i].Name, tunnel: createdTunnel})
		}
	}

	return result, nil
}

func groupTunnelsByConnection(cfg *config.Config, defs []config.TunnelDef) ([]config.ConnectionDef, map[string][]config.TunnelDef, error) {
	groupedTunnelDefs := make(map[string][]config.TunnelDef)
	for _, tunnelDef := range defs {
		groupedTunnelDefs[tunnelDef.Connection] = append(groupedTunnelDefs[tunnelDef.Connection], tunnelDef)
	}

	orderedConnections := make([]config.ConnectionDef, 0, len(groupedTunnelDefs))
	for _, connectionDef := range cfg.Connections {
		if _, used := groupedTunnelDefs[connectionDef.Name]; used {
			orderedConnections = append(orderedConnections, connectionDef)
		}
	}

	if len(orderedConnections) != len(groupedTunnelDefs) {
		return nil, nil, fmt.Errorf("failed to resolve selected tunnel connections")
	}

	return orderedConnections, groupedTunnelDefs, nil
}

func promptPassphrase(in io.Reader, inputReader *bufio.Reader, out io.Writer, connectionName string) (string, error) {
	if _, err := fmt.Fprintf(out, "passphrase for connection %q: ", connectionName); err != nil {
		return "", fmt.Errorf("write passphrase prompt: %w", err)
	}

	if stdinFile, ok := in.(*os.File); ok && term.IsTerminal(int(stdinFile.Fd())) {
		raw, err := term.ReadPassword(int(stdinFile.Fd()))
		if _, newlineErr := fmt.Fprintln(out); newlineErr != nil {
			return "", fmt.Errorf("write passphrase prompt newline: %w", newlineErr)
		}
		if err != nil {
			return "", fmt.Errorf("read passphrase: %w", err)
		}
		return string(raw), nil
	}

	passphrase, err := inputReader.ReadString('\n')
	if err != nil && err != io.EOF {
		return "", fmt.Errorf("read passphrase: %w", err)
	}

	return strings.TrimRight(passphrase, "\r\n"), nil
}

func runTunnels(ctx context.Context, tunnels []namedTunnel) error {
	runCtx, stopSignal := signal.NotifyContext(ctx, os.Interrupt)
	defer stopSignal()

	errCh := make(chan error, len(tunnels))
	var wg sync.WaitGroup

	for _, named := range tunnels {
		tnl := named.tunnel
		name := named.name
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

	for _, named := range tunnels {
		if err := named.tunnel.Stop(); err != nil {
			errCh <- fmt.Errorf("tunnel %q stop failed: %w", named.name, err)
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
