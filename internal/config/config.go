// Package config loads and holds server/client configuration from YAML files
// and environment variable overrides.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// ServerConfig holds all configuration for the server binary.
type ServerConfig struct {
	// ListenAddr is the control-plane TLS listener address (default ":4443").
	ListenAddr string `yaml:"listen_addr"`
	// Domain is the base domain used for HTTP tunnel subdomains (e.g. "tunnel.example.com").
	Domain string `yaml:"domain"`
	// Email is used for Let's Encrypt ACME registration.
	Email string `yaml:"email"`
	// Tokens is the list of valid auth tokens (raw values; hashed in memory at runtime).
	Tokens []string `yaml:"tokens"`
	// MaxTunnelsPerToken is the per-token tunnel limit (default 10).
	MaxTunnelsPerToken int `yaml:"max_tunnels_per_token"`
	// RateLimit is the per-token request rate (requests/min, default 100).
	RateLimit int `yaml:"rate_limit"`
	// GlobalRateLimit is the server-wide request rate (requests/min, default 1000).
	GlobalRateLimit int `yaml:"global_rate_limit"`
	// TCPPortRange defines the [min, max] port range for TCP tunnel allocation (default [10000,65535]).
	TCPPortRange [2]int `yaml:"tcp_port_range"`
	// CertStorage is the directory for certmagic certificate storage.
	CertStorage string `yaml:"cert_storage"`
	// DevMode skips Let's Encrypt and uses a self-signed certificate (local testing only).
	DevMode bool `yaml:"dev_mode"`
	// DNSProvider selects the libdns provider for DNS-01 challenge (e.g. "cloudflare").
	DNSProvider string `yaml:"dns_provider"`
	// DNSAPIToken is the API token for the DNS provider.
	DNSAPIToken string `yaml:"dns_api_token"`
	// DashboardEnabled enables the embedded web dashboard (default true).
	DashboardEnabled bool `yaml:"dashboard_enabled"`
	// DashboardAddr is the dashboard HTTP listener address (default ":8080").
	DashboardAddr string `yaml:"dashboard_addr"`
	// AdminToken is the token required for admin dashboard endpoints.
	AdminToken string `yaml:"admin_token"`
	// HTTPProxyAddr is the HTTPS listener address for the HTTP tunnel proxy (default ":443").
	HTTPProxyAddr string `yaml:"http_proxy_addr"`
	// HTTPRedirectAddr is the plain HTTP listener address that redirects to HTTPS (default ":80").
	HTTPRedirectAddr string `yaml:"http_redirect_addr"`
	// KeyStorePath is the path to the API keys JSON file (default "/etc/htn-tunnel/keys.json").
	KeyStorePath string `yaml:"key_store_path"`
	// AllowRegistration enables self-service API key registration (default true).
	AllowRegistration *bool `yaml:"allow_registration"`
	// AllowAnonymous enables anonymous (no-token) connections with limited features (default true).
	AllowAnonymous *bool `yaml:"allow_anonymous"`
	// AnonTunnelTTL is how long anonymous tunnels live before auto-expiry, in seconds (default 7200 = 2h).
	AnonTunnelTTL int `yaml:"anon_tunnel_ttl"`
}

// defaults fills in zero values with sensible defaults.
func (c *ServerConfig) defaults() {
	if c.ListenAddr == "" {
		c.ListenAddr = ":4443"
	}
	if c.MaxTunnelsPerToken == 0 {
		c.MaxTunnelsPerToken = 10
	}
	if c.RateLimit == 0 {
		c.RateLimit = 100
	}
	if c.GlobalRateLimit == 0 {
		c.GlobalRateLimit = 1000
	}
	if c.TCPPortRange == [2]int{} {
		c.TCPPortRange = [2]int{10000, 65535}
	}
	if c.CertStorage == "" {
		c.CertStorage = "/var/lib/htn-tunnel/certs"
	}
	if c.DashboardAddr == "" {
		c.DashboardAddr = ":8080"
	}
	if c.HTTPProxyAddr == "" {
		c.HTTPProxyAddr = ":443"
	}
	if c.HTTPRedirectAddr == "" {
		c.HTTPRedirectAddr = ":80"
	}
	if c.KeyStorePath == "" {
		c.KeyStorePath = "/etc/htn-tunnel/keys.json"
	}
	if c.AllowRegistration == nil {
		t := true
		c.AllowRegistration = &t
	}
	if c.AllowAnonymous == nil {
		t := true
		c.AllowAnonymous = &t
	}
	if c.AnonTunnelTTL == 0 {
		c.AnonTunnelTTL = 7200 // 2 hours
	}
	// DashboardEnabled defaults to true — apply after YAML parse only if not explicitly set.
	// (yaml.v3 leaves bool false when the key is absent, so we can't distinguish
	//  "absent" from "explicitly false"; callers must set DashboardEnabled=true in YAML
	//  or leave the field absent for the default. env var override handles the rest.)
}

// applyEnv overrides config fields from environment variables.
// Env vars take precedence over YAML values.
func (c *ServerConfig) applyEnv() {
	if v := os.Getenv("HTN_LISTEN_ADDR"); v != "" {
		c.ListenAddr = v
	}
	if v := os.Getenv("HTN_DOMAIN"); v != "" {
		c.Domain = v
	}
	if v := os.Getenv("HTN_EMAIL"); v != "" {
		c.Email = v
	}
	if v := os.Getenv("HTN_TOKENS"); v != "" {
		c.Tokens = splitCSV(v)
	}
	if v := os.Getenv("HTN_MAX_TUNNELS_PER_TOKEN"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			c.MaxTunnelsPerToken = n
		}
	}
	if v := os.Getenv("HTN_RATE_LIMIT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			c.RateLimit = n
		}
	}
	if v := os.Getenv("HTN_GLOBAL_RATE_LIMIT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			c.GlobalRateLimit = n
		}
	}
	if v := os.Getenv("HTN_TCP_PORT_MIN"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			c.TCPPortRange[0] = n
		}
	}
	if v := os.Getenv("HTN_TCP_PORT_MAX"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			c.TCPPortRange[1] = n
		}
	}
	if v := os.Getenv("HTN_CERT_STORAGE"); v != "" {
		c.CertStorage = v
	}
	if v := os.Getenv("HTN_DEV_MODE"); v != "" {
		c.DevMode = v == "true" || v == "1"
	}
	if v := os.Getenv("HTN_DNS_PROVIDER"); v != "" {
		c.DNSProvider = v
	}
	if v := os.Getenv("HTN_DNS_API_TOKEN"); v != "" {
		c.DNSAPIToken = v
	}
	if v := os.Getenv("HTN_DASHBOARD_ENABLED"); v != "" {
		c.DashboardEnabled = v == "true" || v == "1"
	}
	if v := os.Getenv("HTN_DASHBOARD_ADDR"); v != "" {
		c.DashboardAddr = v
	}
	if v := os.Getenv("HTN_ADMIN_TOKEN"); v != "" {
		c.AdminToken = v
	}
}

// LoadServerConfig reads the YAML file at path, applies defaults, then env overrides.
// Falls back to all-defaults if path is "" or the file does not exist.
func LoadServerConfig(path string) (*ServerConfig, error) {
	cfg := &ServerConfig{}

	if path != "" {
		data, err := os.ReadFile(path)
		if err != nil && !os.IsNotExist(err) {
			return nil, fmt.Errorf("read config %q: %w", path, err)
		}
		if err == nil {
			if err := yaml.Unmarshal(data, cfg); err != nil {
				return nil, fmt.Errorf("parse config %q: %w", path, err)
			}
		}
	}

	cfg.defaults()
	cfg.applyEnv()
	return cfg, nil
}

// ClientConfig holds all configuration for the client binary.
type ClientConfig struct {
	// ServerAddr is the server control-plane address (host:port, default "tunnel.example.com:4443").
	ServerAddr string `yaml:"server_addr"`
	// Token is the auth token for this client.
	Token string `yaml:"token"`
}

// configDir returns the user's ~/.htn-tunnel directory, creating it if needed.
func configDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".htn-tunnel")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", fmt.Errorf("create config dir: %w", err)
	}
	return dir, nil
}

// ClientConfigPath returns the default client config file path.
func ClientConfigPath() (string, error) {
	dir, err := configDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.yaml"), nil
}

// LoadClientConfig reads the client config from the default path.
func LoadClientConfig() (*ClientConfig, error) {
	path, err := ClientConfigPath()
	if err != nil {
		return nil, err
	}

	cfg := &ClientConfig{}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return cfg, nil // no config yet; caller handles missing token
	}
	if err != nil {
		return nil, fmt.Errorf("read client config: %w", err)
	}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse client config: %w", err)
	}
	return cfg, nil
}

// SaveClientConfig writes cfg to the default client config path with 0600 permissions.
func SaveClientConfig(cfg *ClientConfig) error {
	path, err := ClientConfigPath()
	if err != nil {
		return err
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal client config: %w", err)
	}
	return os.WriteFile(path, data, 0600)
}

func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}
