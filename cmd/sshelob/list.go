package main

import (
	"fmt"
	"io"

	"github.com/danilbrenner/sshelob/internal/config"
	"github.com/spf13/cobra"
)

func listTunnels(w io.Writer, cfg *config.Config) error {
	for i, tunnelDef := range cfg.Tunnels {
		if _, err := fmt.Fprintf(w, "(%d)%s: %s\n", i+1, tunnelDef.Type, tunnelDef.Name); err != nil {
			return fmt.Errorf("failed to write tunnel list: %w", err)
		}
	}

	return nil
}

func listCmd(deps cliDeps, opts *cliOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List configured tunnels",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			cfg, err := config.Load(opts.configPath)
			if err != nil {
				return err
			}

			return listTunnels(deps.stdout, cfg)
		},
	}
}
