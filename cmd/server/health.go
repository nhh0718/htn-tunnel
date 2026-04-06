package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/nhh0718/htn-tunnel/internal/config"
	"github.com/spf13/cobra"
)

func healthCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "health",
		Short: "Check server health",
		RunE: func(cmd *cobra.Command, args []string) error {
			path := flagConfig
			if path == "" {
				path = "server.yaml"
			}

			cfg, err := config.LoadServerConfig(path)
			if err != nil {
				// Fall back to default dashboard addr when config is missing.
				cfg = &config.ServerConfig{DashboardAddr: ":8080"}
			}

			addr := cfg.DashboardAddr
			if addr == "" {
				addr = ":8080"
			}
			// Ensure addr has a host for http.Get.
			if addr[0] == ':' {
				addr = "localhost" + addr
			}

			url := fmt.Sprintf("http://%s/_healthz", addr)
			resp, err := http.Get(url) //nolint:noctx
			if err != nil {
				fmt.Fprintf(os.Stderr, "  health check failed: %v\n", err)
				fmt.Fprintf(os.Stderr, "  Is the server running? (dashboard addr: %s)\n", cfg.DashboardAddr)
				os.Exit(1)
			}
			defer resp.Body.Close()

			var result map[string]any
			if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
				fmt.Fprintf(os.Stderr, "  invalid response from server: %v\n", err)
				os.Exit(1)
			}

			if resp.StatusCode != http.StatusOK {
				fmt.Printf("  status: DEGRADED (HTTP %d)\n", resp.StatusCode)
			} else {
				fmt.Printf("  status:    %v\n", result["status"])
			}
			fmt.Printf("  version:   %v\n", result["version"])
			fmt.Printf("  tunnels:   %v\n", result["tunnels"])
			fmt.Printf("  users:     %v\n", result["users"])
			fmt.Printf("  bytes_in:  %v\n", result["bytes_in"])
			fmt.Printf("  bytes_out: %v\n", result["bytes_out"])
			return nil
		},
	}
}
