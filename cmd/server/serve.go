package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/nhh0718/htn-tunnel/internal/config"
	"github.com/nhh0718/htn-tunnel/internal/server"
	"github.com/spf13/cobra"
)

func serveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "serve",
		Short: "Start the tunnel server",
		RunE: func(cmd *cobra.Command, args []string) error {
			path := flagConfig
			if path == "" {
				path = "server.yaml"
			}

			cfg, err := config.LoadServerConfig(path)
			if err != nil {
				slog.Error("load config", "err", err)
				os.Exit(1)
			}

			slog.Info("starting htn-tunnel-server", "version", version)

			srv, err := server.NewServer(cfg, path, version)
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
			return nil
		},
	}
}
