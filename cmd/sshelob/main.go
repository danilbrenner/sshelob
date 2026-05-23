package main

import (
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

	slog.Info("config loaded", "path", *configPath, "configCnt", len(cfg.Tunnels))
}
