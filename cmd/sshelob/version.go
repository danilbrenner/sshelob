package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	Version   = "v0.0.0-dev"
	Commit    = "unknown"
	BuildDate = "unknown"
)

func formatVersion(version string, commit string, buildDate string) string {
	return fmt.Sprintf("sshelob %s (commit %s, built %s)", version, commit, buildDate)
}

func versionCmd(deps cliDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			_, err := fmt.Fprintln(deps.stdout, formatVersion(Version, Commit, BuildDate))
			return err
		},
	}
}
