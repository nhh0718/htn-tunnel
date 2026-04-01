package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/htn-sys/htn-tunnel/internal/client"
	"github.com/htn-sys/htn-tunnel/internal/config"
	"github.com/spf13/cobra"
)

// version is injected at build time via ldflags: -X main.version=<tag>
var version = "dev"

// Global flags shared across subcommands.
var (
	flagServer string
	flagToken  string
)

func main() {
	root := &cobra.Command{
		Use:     "htn-tunnel",
		Short:   "Self-hosted tunneling client",
		Version: version,
	}
	root.PersistentFlags().StringVar(&flagServer, "server", "", "server address (host:port)")
	root.PersistentFlags().StringVar(&flagToken, "token", "", "override auth token")

	root.AddCommand(httpCmd(), tcpCmd(), authCmd())

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

// httpCmd implements: htn-tunnel http <port> [--subdomain name]
func httpCmd() *cobra.Command {
	var subdomain string
	cmd := &cobra.Command{
		Use:   "http <port>",
		Short: "Create an HTTP tunnel to localhost:<port>",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			localPort, err := parsePort(args[0])
			if err != nil {
				return err
			}

			cfg, err := loadClientCfg()
			if err != nil {
				return err
			}

			ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer cancel()

			printBanner(version, "connecting...")
			fmt.Printf("  Server:    %s\n", cfg.ServerAddr)
			fmt.Printf("  Ctrl+C to disconnect\n\n")

			r := client.NewReconnector(cfg, "http", localPort, subdomain)
			return r.Run(ctx)
		},
	}
	cmd.Flags().StringVar(&subdomain, "subdomain", "", "request a specific subdomain")
	return cmd
}

// tcpCmd implements: htn-tunnel tcp <port>
func tcpCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tcp <port>",
		Short: "Create a TCP tunnel to localhost:<port>",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			localPort, err := parsePort(args[0])
			if err != nil {
				return err
			}

			cfg, err := loadClientCfg()
			if err != nil {
				return err
			}

			ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer cancel()

			printBanner(version, "connecting...")
			fmt.Printf("  Server:    %s\n", cfg.ServerAddr)
			fmt.Printf("  Ctrl+C to disconnect\n\n")

			r := client.NewReconnector(cfg, "tcp", localPort, "")
			return r.Run(ctx)
		},
	}
	return cmd
}

// authCmd implements: htn-tunnel auth <token> [--server addr]
func authCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "auth <token>",
		Short: "Save auth token to config file",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			token := args[0]
			cfg := &config.ClientConfig{Token: token}
			if flagServer != "" {
				cfg.ServerAddr = flagServer
			}
			if err := config.SaveClientConfig(cfg); err != nil {
				return fmt.Errorf("save config: %w", err)
			}
			// Mask token in output for security.
			fmt.Printf("Token saved. (stored in ~/.htn-tunnel/config.yaml)\n")
			fmt.Printf("Warning: token is stored in plaintext — protect the config file.\n")
			return nil
		},
	}
}

// loadClientCfg loads the client config file and applies flag overrides.
func loadClientCfg() (*config.ClientConfig, error) {
	cfg, err := config.LoadClientConfig()
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}
	if flagServer != "" {
		cfg.ServerAddr = flagServer
	}
	if flagToken != "" {
		cfg.Token = flagToken
	}
	if cfg.ServerAddr == "" {
		cfg.ServerAddr = "localhost:4443"
	}
	if cfg.Token == "" {
		return nil, fmt.Errorf("no auth token set; run: htn-tunnel auth <token>")
	}
	return cfg, nil
}

func parsePort(s string) (int, error) {
	p, err := strconv.Atoi(s)
	if err != nil || p < 1 || p > 65535 {
		return 0, fmt.Errorf("invalid port %q (must be 1-65535)", s)
	}
	return p, nil
}

func printBanner(ver, status string) {
	fmt.Printf("\nhtn-tunnel v%s\n\n", ver)
	fmt.Printf("  Status:    %s\n", status)
}

// splitAddr extracts host from "host:port".
func splitAddr(addr string) (host, port string, err error) {
	return splitHostPort(addr)
}

func splitHostPort(addr string) (string, string, error) {
	for i := len(addr) - 1; i >= 0; i-- {
		if addr[i] == ':' {
			return addr[:i], addr[i+1:], nil
		}
	}
	return addr, "", nil
}
