package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/nhh0718/htn-tunnel/internal/config"
	"github.com/spf13/cobra"
)

func statusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show server status overview",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runStatus()
		},
	}
}

func runStatus() error {
	cfgPath := resolveConfigPath()
	cfg, err := config.LoadServerConfig(cfgPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	fmt.Printf("\n  htn-tunnel Server Status\n\n")
	fmt.Printf("  Version:     %s\n", version)
	fmt.Printf("  Config:      %s\n", cfgPath)
	fmt.Printf("  Domain:      %s\n", cfg.Domain)
	fmt.Printf("  Listen:      %s\n", cfg.ListenAddr)
	fmt.Printf("  Dashboard:   %s\n", cfg.DashboardAddr)
	fmt.Printf("  HTTP Proxy:  %s\n", cfg.HTTPProxyAddr)
	fmt.Printf("  Keys file:   %s\n", cfg.KeyStorePath)
	fmt.Printf("  Anonymous:   %v\n", cfg.AllowAnonymous != nil && *cfg.AllowAnonymous)

	// Probe live server via healthz endpoint.
	addr := cfg.DashboardAddr
	if addr == "" {
		addr = ":8080"
	}
	if addr[0] == ':' {
		addr = "localhost" + addr
	}
	url := fmt.Sprintf("http://%s/_healthz", addr)
	resp, err := http.Get(url) //nolint:noctx
	if err != nil {
		fmt.Printf("  Server:      offline\n\n")
		return nil
	}
	defer resp.Body.Close()

	var health map[string]any
	json.NewDecoder(resp.Body).Decode(&health) //nolint:errcheck
	fmt.Printf("  Server:      online\n")
	if t, ok := health["tunnels"]; ok {
		fmt.Printf("  Tunnels:     %.0f\n", t)
	}
	if u, ok := health["users"]; ok {
		fmt.Printf("  Users:       %.0f\n", u)
	}
	fmt.Println()
	return nil
}

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
