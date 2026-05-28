package main

import (
	"context"
	"fmt"

	"github.com/danilbrenner/sshelob/internal/update"
	"github.com/spf13/cobra"
)

const (
	defaultGitHubAPIBaseURL = "https://api.github.com"
	defaultGitHubRepo       = "danilbrenner/sshelob"
)

func updateCmd(ctx context.Context, deps cliDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "update",
		Short: "Self-update to latest stable release",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			tag, err := update.RunUpdate(ctx, deps.client, deps.apiBaseURL, deps.repo)
			if err != nil {
				return err
			}

			if _, err := fmt.Fprintf(deps.stdout, "updated sshelob to %s\n", tag); err != nil {
				return err
			}

			return nil
		},
	}
}
