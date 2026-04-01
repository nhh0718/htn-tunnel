package server

import (
	"fmt"
	"log/slog"
	"os"
	"sync"

	"github.com/nhh0718/htn-tunnel/internal/config"
	"gopkg.in/yaml.v3"
)

// ConfigProvider allows the admin dashboard to read/write server config.
type ConfigProvider struct {
	mu       sync.RWMutex
	cfg      *config.ServerConfig
	filePath string // path to server.yaml
}

// NewConfigProvider wraps a ServerConfig for dashboard access.
func NewConfigProvider(cfg *config.ServerConfig, filePath string) *ConfigProvider {
	return &ConfigProvider{cfg: cfg, filePath: filePath}
}

// GetEditableConfig returns whitelisted config fields (secrets masked).
func (p *ConfigProvider) GetEditableConfig() map[string]any {
	p.mu.RLock()
	defer p.mu.RUnlock()

	allowReg := true
	if p.cfg.AllowRegistration != nil {
		allowReg = *p.cfg.AllowRegistration
	}

	return map[string]any{
		"domain":                p.cfg.Domain,
		"listen_addr":          p.cfg.ListenAddr,
		"http_proxy_addr":      p.cfg.HTTPProxyAddr,
		"http_redirect_addr":   p.cfg.HTTPRedirectAddr,
		"dashboard_addr":       p.cfg.DashboardAddr,
		"max_tunnels_per_token": p.cfg.MaxTunnelsPerToken,
		"rate_limit":           p.cfg.RateLimit,
		"global_rate_limit":    p.cfg.GlobalRateLimit,
		"tcp_port_range":       p.cfg.TCPPortRange,
		"allow_registration":   allowReg,
		"dashboard_enabled":    p.cfg.DashboardEnabled,
		"dev_mode":             p.cfg.DevMode,
	}
}

// UpdateConfig applies whitelisted updates and persists to yaml.
func (p *ConfigProvider) UpdateConfig(updates map[string]any) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	for key, val := range updates {
		switch key {
		case "domain":
			if s, ok := val.(string); ok {
				p.cfg.Domain = s
			}
		case "max_tunnels_per_token":
			if n, ok := toInt(val); ok {
				p.cfg.MaxTunnelsPerToken = n
			}
		case "rate_limit":
			if n, ok := toInt(val); ok {
				p.cfg.RateLimit = n
			}
		case "global_rate_limit":
			if n, ok := toInt(val); ok {
				p.cfg.GlobalRateLimit = n
			}
		case "allow_registration":
			if b, ok := val.(bool); ok {
				p.cfg.AllowRegistration = &b
			}
		case "dashboard_enabled":
			if b, ok := val.(bool); ok {
				p.cfg.DashboardEnabled = b
			}
		default:
			// Ignore unknown/protected fields (listen_addr, tokens, secrets, etc.)
			slog.Debug("config update: ignoring protected field", "key", key)
		}
	}

	return p.save()
}

func (p *ConfigProvider) save() error {
	if p.filePath == "" {
		return nil
	}
	data, err := yaml.Marshal(p.cfg)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	if err := os.WriteFile(p.filePath, data, 0600); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	slog.Info("server config updated and saved", "path", p.filePath)
	return nil
}

func toInt(v any) (int, bool) {
	switch n := v.(type) {
	case float64:
		return int(n), true
	case int:
		return n, true
	case int64:
		return int(n), true
	}
	return 0, false
}
