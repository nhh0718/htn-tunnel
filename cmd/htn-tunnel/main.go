package main

import (
	"bufio"
	"context"
	"fmt"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/nhh0718/htn-tunnel/internal/client"
	"github.com/nhh0718/htn-tunnel/internal/config"
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

	root.AddCommand(httpCmd(), tcpCmd(), authCmd(), loginCmd(), logoutCmd(), dashboardCmd(), statusCmd())

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

// httpCmd implements: htn-tunnel http <port>[:<subdomain>]
func httpCmd() *cobra.Command {
	var subdomain string
	cmd := &cobra.Command{
		Use:   "http <port>[:<subdomain>]",
		Short: "Create an HTTP tunnel to localhost:<port>",
		Long:  "Create an HTTP tunnel.\n\nExamples:\n  htn-tunnel http 3000          # interactive subdomain picker\n  htn-tunnel http 3000:myapp    # fixed subdomain\n  htn-tunnel http 8080:dev",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			localPort, sub, err := parsePortSubdomain(args[0])
			if err != nil {
				return err
			}

			// --subdomain flag as fallback (backward compat)
			if sub == "" && subdomain != "" {
				sub = subdomain
			}

			cfg, err := loadClientCfg()
			if err != nil {
				return err
			}

			// Custom subdomain requires auth — auto-trigger login if no token.
			if sub != "" && cfg.Token == "" {
				fmt.Println("\n  Cần đăng ký để dùng subdomain cố định.")
				if err := runLogin(cfg); err != nil {
					return err
				}
			}

			ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer cancel()

			// If authenticated and no subdomain specified, try interactive picker.
			if sub == "" && cfg.Token != "" {
				resolved, err := pickSubdomain(ctx, cfg)
				if err != nil {
					return err
				}
				sub = resolved
			} else if sub != "" && cfg.Token != "" {
				// Validate subdomain is owned by this key.
				if err := validateSubdomain(ctx, cfg, sub); err != nil {
					resolved, pickErr := handleSubdomainError(ctx, cfg, sub, err)
					if pickErr != nil {
						return pickErr
					}
					sub = resolved
				}
			}
			// Token empty + no subdomain → anonymous random subdomain (no picker).

			r := client.NewReconnector(cfg, "http", localPort, sub, version)
			return r.Run(ctx)
		},
	}
	cmd.Flags().StringVar(&subdomain, "subdomain", "", "(deprecated) use port:subdomain syntax instead")
	return cmd
}

// parsePortSubdomain parses "3000:hoang" → (3000, "hoang") or "3000" → (3000, "").
func parsePortSubdomain(arg string) (int, string, error) {
	parts := strings.SplitN(arg, ":", 2)
	p, err := strconv.Atoi(parts[0])
	if err != nil || p < 1 || p > 65535 {
		return 0, "", fmt.Errorf("invalid port %q (must be 1-65535)", parts[0])
	}
	sub := ""
	if len(parts) == 2 && parts[1] != "" {
		sub = strings.ToLower(parts[1])
	}
	return p, sub, nil
}

// pickSubdomain connects to server, gets owned subdomains, presents interactive picker.
func pickSubdomain(ctx context.Context, cfg *config.ClientConfig) (string, error) {
	c := client.NewClient(cfg)
	if err := c.Connect(ctx); err != nil {
		return "", nil // can't connect for picker, use random
	}
	info, err := c.GetAccountInfo()
	c.Close()
	if err != nil || len(info.Subdomains) == 0 {
		return "", nil // no subdomains, use random
	}

	domain := info.Domain
	if domain == "" {
		host, _, _ := strings.Cut(cfg.ServerAddr, ":")
		domain = host
	}

	fmt.Printf("\n  Chọn subdomain:\n\n")
	for i, sub := range info.Subdomains {
		fmt.Printf("  [%d] %s.%s\n", i+1, sub, domain)
	}
	fmt.Printf("  [%d] Subdomain ngẫu nhiên\n\n", len(info.Subdomains)+1)
	fmt.Printf("  Nhập số (1-%d): ", len(info.Subdomains)+1)

	var choice int
	fmt.Scan(&choice)
	fmt.Println()

	if choice >= 1 && choice <= len(info.Subdomains) {
		return info.Subdomains[choice-1], nil
	}
	return "", nil // random
}

// validateSubdomain checks if subdomain is owned by the current key.
func validateSubdomain(ctx context.Context, cfg *config.ClientConfig, sub string) error {
	c := client.NewClient(cfg)
	if err := c.Connect(ctx); err != nil {
		return nil // can't validate, let server handle it
	}
	info, err := c.GetAccountInfo()
	c.Close()
	if err != nil {
		return nil
	}
	for _, s := range info.Subdomains {
		if s == sub {
			return nil // owned
		}
	}
	return fmt.Errorf("subdomain \"%s\" không thuộc tài khoản của bạn", sub)
}

// handleSubdomainError shows error and presents fallback options.
func handleSubdomainError(ctx context.Context, cfg *config.ClientConfig, requested string, origErr error) (string, error) {
	c := client.NewClient(cfg)
	if err := c.Connect(ctx); err != nil {
		return "", origErr
	}
	info, err := c.GetAccountInfo()
	c.Close()
	if err != nil || len(info.Subdomains) == 0 {
		return "", origErr
	}

	domain := info.Domain
	if domain == "" {
		host, _, _ := strings.Cut(cfg.ServerAddr, ":")
		domain = host
	}

	fmt.Printf("\n  ✗ Subdomain \"%s\" không thuộc tài khoản của bạn.\n\n", requested)
	fmt.Println("  Subdomain của bạn:")
	for _, sub := range info.Subdomains {
		fmt.Printf("    %s.%s\n", sub, domain)
	}
	fmt.Println()
	for i, sub := range info.Subdomains {
		fmt.Printf("  [%d] Dùng %s\n", i+1, sub)
	}
	fmt.Printf("  [%d] Subdomain ngẫu nhiên\n", len(info.Subdomains)+1)
	fmt.Printf("  [%d] Hủy\n\n", len(info.Subdomains)+2)
	fmt.Printf("  Nhập số (1-%d): ", len(info.Subdomains)+2)

	var choice int
	fmt.Scan(&choice)
	fmt.Println()

	if choice == len(info.Subdomains)+2 {
		return "", fmt.Errorf("cancelled")
	}
	if choice >= 1 && choice <= len(info.Subdomains) {
		return info.Subdomains[choice-1], nil
	}
	return "", nil // random
}

// loginCmd implements: htn-tunnel login — opens browser for registration, receives key via callback.
func loginCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "login",
		Short: "Đăng ký hoặc đăng nhập qua browser",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _ := loadClientCfg()
			return runLogin(cfg)
		},
	}
}

// runLogin executes the browser-based login/register flow.
// Starts a localhost callback server, opens the dashboard in the browser,
// waits for the key callback or falls back to manual entry.
func runLogin(cfg *config.ClientConfig) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	callbackURL, resultCh, err := client.StartCallbackServer(ctx)
	if err != nil {
		return err
	}

	domain := extractHost(cfg.ServerAddr)
	dashboardURL := fmt.Sprintf("https://dashboard.%s/_dashboard/#register?callback=%s",
		domain, url.QueryEscape(callbackURL))

	fmt.Printf("\n  Đang mở trình duyệt...\n  %s\n\n", dashboardURL)

	if err := client.OpenBrowser(dashboardURL); err != nil {
		// Fallback: manual key entry for headless/SSH.
		fmt.Printf("  Không mở được browser. Mở link trên rồi nhập key:\n  Key: ")
		scanner := bufio.NewScanner(os.Stdin)
		if scanner.Scan() {
			key := strings.TrimSpace(scanner.Text())
			if key != "" {
				cfg.Token = key
				if err := config.SaveClientConfig(cfg); err != nil {
					return fmt.Errorf("save config: %w", err)
				}
				fmt.Printf("\n  ✓ Auth thành công! Key đã lưu.\n\n")
				return nil
			}
		}
		return fmt.Errorf("không nhận được key")
	}

	fmt.Printf("  (Đợi đăng ký trên browser...)\n\n")

	select {
	case result := <-resultCh:
		if result.Key == "" {
			return fmt.Errorf("không nhận được key từ browser")
		}
		cfg.Token = result.Key
		if err := config.SaveClientConfig(cfg); err != nil {
			return fmt.Errorf("save config: %w", err)
		}
		fmt.Printf("  ✓ Auth thành công! Key đã lưu.\n")
		if result.Name != "" {
			fmt.Printf("  Xin chào, %s!\n", result.Name)
		}
		fmt.Println()
	case <-ctx.Done():
		return fmt.Errorf("timeout — không nhận được phản hồi từ browser")
	}
	return nil
}

// extractHost extracts the hostname from "host:port" or returns addr as-is.
func extractHost(addr string) string {
	host, _, err := splitHostPort(addr)
	if err != nil || host == "" {
		return addr
	}
	return host
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

			r := client.NewReconnector(cfg, "tcp", localPort, "", version)
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

// statusCmd shows current key info and subdomains by querying the server.
func statusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show your account info and subdomains",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadClientCfg()
			if err != nil {
				return err
			}

			fmt.Printf("\nhtn-tunnel status\n\n")

			if cfg.Token == "" {
				fmt.Printf("  Status:    Chưa đăng nhập (anonymous)\n")
				fmt.Printf("  Tunnels:   Random subdomain, giới hạn 1/IP\n")
				fmt.Printf("  Server:    %s\n\n", cfg.ServerAddr)
				fmt.Printf("  Đăng ký:   htn-tunnel login\n\n")
				return nil
			}

			ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer cancel()

			c := client.NewClient(cfg)
			if err := c.Connect(ctx); err != nil {
				return fmt.Errorf("connect: %w", err)
			}
			defer c.Close()

			info, err := c.GetAccountInfo()
			if err != nil {
				return err
			}

			masked := cfg.Token
			if len(masked) > 12 {
				masked = masked[:8] + "..." + masked[len(masked)-4:]
			}
			fmt.Printf("  Key:         %s\n", masked)
			fmt.Printf("  Name:        %s\n", info.Name)
			fmt.Printf("  Subdomains:  %s\n", joinOrNone(info.Subdomains))
			fmt.Printf("  Max tunnels: %d\n", info.MaxTunnels)
			fmt.Printf("  Server:      %s\n\n", cfg.ServerAddr)
			return nil
		},
	}
}

// dashboardCmd opens the web dashboard in the user's browser.
func dashboardCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "dashboard",
		Short: "Mở dashboard trong trình duyệt",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _ := loadClientCfg()
			domain := extractHost(cfg.ServerAddr)
			u := fmt.Sprintf("https://dashboard.%s/_dashboard/", domain)
			fmt.Printf("  Đang mở: %s\n", u)
			return client.OpenBrowser(u)
		},
	}
}

// logoutCmd clears the saved auth key.
func logoutCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "logout",
		Short: "Xóa auth key khỏi máy",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := &config.ClientConfig{}
			// Preserve server address, clear token.
			if old, err := config.LoadClientConfig(); err == nil {
				cfg.ServerAddr = old.ServerAddr
			}
			if err := config.SaveClientConfig(cfg); err != nil {
				return fmt.Errorf("save config: %w", err)
			}
			fmt.Println("  ✓ Đã đăng xuất. Key đã xóa.")
			return nil
		},
	}
}

func joinOrNone(ss []string) string {
	if len(ss) == 0 {
		return "(none)"
	}
	result := ""
	for i, s := range ss {
		if i > 0 {
			result += ", "
		}
		result += s
	}
	return result
}

// loadClientCfg loads the client config file and applies flag overrides.
// Token may be empty (anonymous mode).
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
	// No longer require token — empty token = anonymous mode.
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

func splitHostPort(addr string) (string, string, error) {
	for i := len(addr) - 1; i >= 0; i-- {
		if addr[i] == ':' {
			return addr[:i], addr[i+1:], nil
		}
	}
	return addr, "", nil
}
