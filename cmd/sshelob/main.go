package main

import (
	"context"
	"flag"
	"log/slog"
	"os"

	"github.com/danilbrenner/sshelob/internal/config"
	"github.com/danilbrenner/sshelob/internal/logging"
)

func main() {
	stdoutHandler, stderrHandler := logging.Config()

	logger := slog.New(logging.NewSplitHandler(stdoutHandler, stderrHandler))

	slog.SetDefault(logger)

	configPath := flag.String("config", "config.yml", "path to YAML config file")
	flag.Parse()
	args := flag.Args()

	if len(args) == 0 {
		slog.Error("missing command", "usage", "sshelob [-config path] <list|run> [indexes]")
		os.Exit(1)
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		slog.Error("failed to load config", "path", *configPath, "error", err)
		os.Exit(1)
	}

	switch args[0] {
	case "list":
		if len(args) > 1 {
			slog.Error("list does not accept extra arguments", "usage", "sshelob [-config path] list")
			os.Exit(1)
		}
		listTunnels(os.Stdout, cfg)
	case "run":
		indexes, parseErr := parseIndexes(args[1:])
		if parseErr != nil {
			slog.Error("invalid run indexes", "error", parseErr, "usage", "sshelob [-config path] run 1,2,3")
			os.Exit(1)
		}

		selected, selectErr := selectTunnels(cfg, indexes)
		if selectErr != nil {
			slog.Error("failed to select tunnels", "error", selectErr)
			os.Exit(1)
		}

		if runErr := runTunnels(context.Background(), selected); runErr != nil {
			slog.Error("run failed", "error", runErr)
			os.Exit(1)
		}
	default:
		slog.Error("unknown command", "command", args[0], "usage", "sshelob [-config path] <list|run> [indexes]")
		os.Exit(1)
	}
}
