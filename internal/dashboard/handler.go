package dashboard

import (
	"context"
	"encoding/json"
	"io/fs"
	"net/http"
	"strings"
	"time"
)

// TunnelStats is the JSON shape returned by tunnel list endpoints.
type TunnelStats struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"`
	Subdomain string    `json:"subdomain,omitempty"`
	Port      int       `json:"port,omitempty"`
	LocalPort int       `json:"local_port"`
	Token     string    `json:"token_prefix"` // masked
	Uptime    string    `json:"uptime"`
	BytesIn   int64     `json:"bytes_in"`
	BytesOut  int64     `json:"bytes_out"`
	CreatedAt time.Time `json:"created_at"`
}

// AggregateStats is the JSON shape returned by stats endpoints.
type AggregateStats struct {
	ActiveHTTP   int   `json:"active_http"`
	ActiveTCP    int   `json:"active_tcp"`
	TotalTunnels int   `json:"total_tunnels"`
	BytesIn      int64 `json:"bytes_in"`
	BytesOut     int64 `json:"bytes_out"`
}

// TunnelProvider is the interface the server's TunnelManager satisfies.
type TunnelProvider interface {
	ListTunnelsForDashboard() []TunnelStats
	StatsForDashboard() AggregateStats
	KillTunnelByID(id string) bool
	TunnelsForToken(token string) []TunnelStats
}

// APIKeyInfo mirrors server.APIKey without importing server package.
type APIKeyInfo struct {
	Name       string   `json:"name"`
	Subdomains []string `json:"subdomains"`
	MaxTunnels int      `json:"max_tunnels"`
	Active     bool     `json:"active"`
	CreatedAt  time.Time `json:"created_at"`
}

// KeyProvider is the interface the server's KeyStore satisfies.
type KeyProvider interface {
	Validate(key string) bool
	GetKey(key string) *APIKeyInfo
	CreateKey(name string, subdomains []string, maxTunnels int) (string, error)
	RevokeKey(keyID string) error
	ListKeys() map[string]*APIKeyInfo
	AddSubdomain(keyID, subdomain string) error
	RemoveSubdomain(keyID, subdomain string) error
	OwnedSubdomains(key string) []string
}

// ConfigProvider allows reading/writing server config from the admin dashboard.
type ConfigProvider interface {
	GetEditableConfig() map[string]any
	UpdateConfig(updates map[string]any) error
}

type contextKey string

const ctxKeyID contextKey = "keyID"

// Handler serves the user dashboard, admin dashboard, and their APIs.
type Handler struct {
	tunnels    TunnelProvider
	keys       KeyProvider
	config     ConfigProvider
	adminToken string
	domain     string
	mux        *http.ServeMux
}

// NewHandler creates a Handler and registers all routes.
func NewHandler(tunnels TunnelProvider, keys KeyProvider, config ConfigProvider, adminToken, domain string) *Handler {
	h := &Handler{
		tunnels:    tunnels,
		keys:       keys,
		config:     config,
		adminToken: adminToken,
		domain:     domain,
		mux:        http.NewServeMux(),
	}
	h.registerRoutes()
	return h
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'")
	h.mux.ServeHTTP(w, r)
}

func (h *Handler) registerRoutes() {
	// --- User Dashboard API ---
	h.mux.HandleFunc("GET /_dashboard/api/info", h.handlePublicInfo)
	h.mux.HandleFunc("POST /_dashboard/api/register", h.handleRegister)
	h.mux.HandleFunc("POST /_dashboard/api/login", h.handleLogin)
	h.mux.HandleFunc("GET /_dashboard/api/me", h.userAuth(h.handleMe))
	h.mux.HandleFunc("POST /_dashboard/api/subdomains", h.userAuth(h.handleAddSubdomain))
	h.mux.HandleFunc("DELETE /_dashboard/api/subdomains/{name}", h.userAuth(h.handleRemoveSubdomain))
	h.mux.HandleFunc("GET /_dashboard/api/tunnels", h.userAuth(h.handleUserTunnels))

	// --- Admin Dashboard API ---
	h.mux.HandleFunc("GET /_admin/api/stats", h.adminOnly(h.handleAdminStats))
	h.mux.HandleFunc("GET /_admin/api/keys", h.adminOnly(h.handleAdminListKeys))
	h.mux.HandleFunc("DELETE /_admin/api/keys/{id}", h.adminOnly(h.handleAdminRevokeKey))
	h.mux.HandleFunc("GET /_admin/api/tunnels", h.adminOnly(h.handleAdminTunnels))
	h.mux.HandleFunc("POST /_admin/api/tunnels/{id}/kill", h.adminOnly(h.handleKill))
	h.mux.HandleFunc("GET /_admin/api/config", h.adminOnly(h.handleGetConfig))
	h.mux.HandleFunc("PUT /_admin/api/config", h.adminOnly(h.handleUpdateConfig))

	// --- Static files ---
	userFS, _ := fs.Sub(staticFiles, "static/user")
	adminFS, _ := fs.Sub(staticFiles, "static/admin")
	h.mux.Handle("GET /_dashboard/", http.StripPrefix("/_dashboard", http.FileServer(http.FS(userFS))))
	h.mux.Handle("GET /_admin/", http.StripPrefix("/_admin", http.FileServer(http.FS(adminFS))))
}

// --- User API Handlers ---

func (h *Handler) handlePublicInfo(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]any{
		"domain": h.domain,
	})
}

func (h *Handler) handleRegister(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name      string `json:"name"`
		Subdomain string `json:"subdomain"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.Name == "" {
		writeError(w, "name is required", http.StatusBadRequest)
		return
	}

	var subs []string
	if req.Subdomain != "" {
		subs = []string{req.Subdomain}
	}
	key, err := h.keys.CreateKey(req.Name, subs, 10)
	if err != nil {
		writeError(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusCreated)
	writeJSON(w, map[string]any{
		"key":        key,
		"name":       req.Name,
		"subdomains": subs,
		"domain":     h.domain,
	})
}

func (h *Handler) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Key string `json:"key"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "invalid request", http.StatusBadRequest)
		return
	}
	if !h.keys.Validate(req.Key) {
		writeError(w, "invalid or revoked key", http.StatusUnauthorized)
		return
	}
	info := h.keys.GetKey(req.Key)
	writeJSON(w, info)
}

func (h *Handler) handleMe(w http.ResponseWriter, r *http.Request) {
	keyID := r.Context().Value(ctxKeyID).(string)
	info := h.keys.GetKey(keyID)
	if info == nil {
		writeError(w, "key not found", http.StatusNotFound)
		return
	}
	writeJSON(w, map[string]any{
		"name":        info.Name,
		"subdomains":  info.Subdomains,
		"max_tunnels": info.MaxTunnels,
		"created_at":  info.CreatedAt,
		"domain":      h.domain,
	})
}

func (h *Handler) handleAddSubdomain(w http.ResponseWriter, r *http.Request) {
	keyID := r.Context().Value(ctxKeyID).(string)
	var req struct {
		Subdomain string `json:"subdomain"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Subdomain == "" {
		writeError(w, "subdomain is required", http.StatusBadRequest)
		return
	}
	if err := h.keys.AddSubdomain(keyID, req.Subdomain); err != nil {
		writeError(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusCreated)
	writeJSON(w, map[string]any{
		"subdomains": h.keys.OwnedSubdomains(keyID),
	})
}

func (h *Handler) handleRemoveSubdomain(w http.ResponseWriter, r *http.Request) {
	keyID := r.Context().Value(ctxKeyID).(string)
	name := r.PathValue("name")
	if err := h.keys.RemoveSubdomain(keyID, name); err != nil {
		writeError(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, map[string]any{
		"subdomains": h.keys.OwnedSubdomains(keyID),
	})
}

func (h *Handler) handleUserTunnels(w http.ResponseWriter, r *http.Request) {
	keyID := r.Context().Value(ctxKeyID).(string)
	writeJSON(w, h.tunnels.TunnelsForToken(keyID))
}

// --- Admin API Handlers ---

func (h *Handler) handleAdminStats(w http.ResponseWriter, r *http.Request) {
	stats := h.tunnels.StatsForDashboard()
	keys := h.keys.ListKeys()
	writeJSON(w, map[string]any{
		"active_http":   stats.ActiveHTTP,
		"active_tcp":    stats.ActiveTCP,
		"total_tunnels": stats.TotalTunnels,
		"total_keys":    len(keys),
		"bytes_in":      stats.BytesIn,
		"bytes_out":     stats.BytesOut,
	})
}

func (h *Handler) handleAdminListKeys(w http.ResponseWriter, r *http.Request) {
	keys := h.keys.ListKeys()
	result := make([]map[string]any, 0, len(keys))
	for id, k := range keys {
		result = append(result, map[string]any{
			"key_preview": maskKey(id),
			"name":        k.Name,
			"subdomains":  k.Subdomains,
			"max_tunnels": k.MaxTunnels,
			"active":      k.Active,
			"created_at":  k.CreatedAt,
		})
	}
	writeJSON(w, result)
}

func (h *Handler) handleAdminRevokeKey(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.keys.RevokeKey(id); err != nil {
		writeError(w, err.Error(), http.StatusNotFound)
		return
	}
	writeJSON(w, map[string]string{"status": "revoked"})
}

func (h *Handler) handleAdminTunnels(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, h.tunnels.ListTunnelsForDashboard())
}

func (h *Handler) handleGetConfig(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, h.config.GetEditableConfig())
}

func (h *Handler) handleUpdateConfig(w http.ResponseWriter, r *http.Request) {
	var updates map[string]any
	if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
		writeError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if err := h.config.UpdateConfig(updates); err != nil {
		writeError(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, map[string]string{"status": "updated"})
}

func (h *Handler) handleKill(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !h.tunnels.KillTunnelByID(id) {
		writeError(w, "tunnel not found", http.StatusNotFound)
		return
	}
	writeJSON(w, map[string]string{"status": "killed"})
}

// --- Middleware ---

func (h *Handler) userAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		key := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		if key == "" {
			writeError(w, "missing API key", http.StatusUnauthorized)
			return
		}
		if !h.keys.Validate(key) {
			writeError(w, "invalid or revoked key", http.StatusUnauthorized)
			return
		}
		ctx := context.WithValue(r.Context(), ctxKeyID, key)
		next(w, r.WithContext(ctx))
	}
}

func (h *Handler) adminOnly(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if h.adminToken == "" {
			writeError(w, "admin token not configured", http.StatusForbidden)
			return
		}
		auth := r.Header.Get("Authorization")
		token := strings.TrimPrefix(auth, "Bearer ")
		if token != h.adminToken {
			writeError(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}

// --- Helpers ---

func maskKey(key string) string {
	if len(key) < 12 {
		return key
	}
	return key[:8] + "..." + key[len(key)-4:]
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v) //nolint:errcheck
}

func writeError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg}) //nolint:errcheck
}
