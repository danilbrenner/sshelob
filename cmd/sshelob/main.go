package main

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"os"

	"github.com/danilbrenner/sshelob/internal/logging"
	"github.com/spf13/cobra"
)

type cliDeps struct {
	stdout io.Writer
	client *http.Client

	apiBaseURL string
	repo       string
}

type cliOptions struct {
	configPath string
}

func newRootCommand(ctx context.Context, deps cliDeps) *cobra.Command {
	if deps.stdout == nil {
		deps.stdout = os.Stdout
	}
	if deps.client == nil {
		deps.client = http.DefaultClient
	}
	if deps.apiBaseURL == "" {
		deps.apiBaseURL = defaultGitHubAPIBaseURL
	}
	if deps.repo == "" {
		deps.repo = defaultGitHubRepo
	}

	opts := &cliOptions{}

	rootCmd := &cobra.Command{
		Use:           "sshelob",
		Short:         "Manage SSH tunnels",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}

	rootCmd.PersistentFlags().StringVar(&opts.configPath, "config", "config.yml", "path to YAML config file")

	rootCmd.AddCommand(versionCmd(deps))

	rootCmd.AddCommand(listCmd(deps, opts))

	rootCmd.AddCommand(updateCmd(ctx, deps))

	rootCmd.AddCommand(runCmd(ctx, opts))

	return rootCmd
}

func main() {
	stdoutHandler, stderrHandler := logging.Config()

	logger := slog.New(logging.NewSplitHandler(stdoutHandler, stderrHandler))

	slog.SetDefault(logger)

	rootCmd := newRootCommand(context.Background(), cliDeps{
		stdout: os.Stdout,
		client: http.DefaultClient,

		apiBaseURL: defaultGitHubAPIBaseURL,
		repo:       defaultGitHubRepo,
	})

	if err := rootCmd.Execute(); err != nil {
		slog.Error("command failed", "error", err)
		os.Exit(1)
	}
}
