package main

import (
	"os"

	"github.com/spf13/cobra"
)

// version is injected at build time via ldflags: -X main.version=<tag>
var version = "dev"

// flagConfig is the --config / -c persistent flag shared across commands.
var flagConfig string

func main() {
	root := &cobra.Command{
		Use:     "htn-server",
		Short:   "htn-tunnel server",
		Version: version,
	}
	root.PersistentFlags().StringVarP(&flagConfig, "config", "c", "", "path to server.yaml")

	serve := serveCmd()
	root.AddCommand(serve, initCmd(), healthCmd())

	// Default to serve when no subcommand is provided.
	root.RunE = serve.RunE

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}
