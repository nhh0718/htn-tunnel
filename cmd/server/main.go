package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/nhh0718/htn-tunnel/internal/config"
	"github.com/nhh0718/htn-tunnel/internal/server"
)

// version is injected at build time via ldflags: -X main.version=<tag>
var version = "dev"

func main() {
	cfgPath := flag.String("config", "", "path to server.yaml (default: ./server.yaml)")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	if *showVersion {
		slog.Info("htn-tunnel-server", "version", version)
		os.Exit(0)
	}

	path := *cfgPath
	if path == "" {
		path = "server.yaml"
	}

	cfg, err := config.LoadServerConfig(path)
	if err != nil {
		slog.Error("load config", "err", err)
		os.Exit(1)
	}

	slog.Info("starting htn-tunnel-server", "version", version)

	srv, err := server.NewServer(cfg, path)
	if err != nil {
		slog.Error("init server", "err", err)
		os.Exit(1)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if err := srv.Start(ctx); err != nil {
		slog.Error("server error", "err", err)
		os.Exit(1)
	}

	slog.Info("server stopped")
}
