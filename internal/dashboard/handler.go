package dashboard

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"strconv"
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

// RequestLogEntry mirrors server.LogEntry without importing the server package.
type RequestLogEntry struct {
	Timestamp  time.Time `json:"ts"`
	TunnelID   string    `json:"tid"`
	Subdomain  string    `json:"sub"`
	Token      string    `json:"tok"`
	Method     string    `json:"m"`
	Path       string    `json:"p"`
	Status     int       `json:"s"`
	DurationMs int       `json:"d"`
	Size       int64     `json:"z"`
}

// RequestLogBucket mirrors server.TrafficBucket for per-minute traffic stats.
type RequestLogBucket struct {
	Timestamp  time.Time `json:"ts"`
	Requests   int       `json:"reqs"`
	BytesIn    int64     `json:"bytes_in"`
	BytesOut   int64     `json:"bytes_out"`
	Status2xx  int       `json:"s2xx"`
	Status3xx  int       `json:"s3xx"`
	Status4xx  int       `json:"s4xx"`
	Status5xx  int       `json:"s5xx"`
	AvgLatency int       `json:"avg_ms"`
}

// RequestLogPathCount mirrors server.PathCount for top-paths analytics.
type RequestLogPathCount struct {
	Path  string `json:"path"`
	Count int    `json:"count"`
}

// RequestLogProvider abstracts access to the in-memory request log.
// Satisfied by server.RequestLogAdapter (in internal/server).
type RequestLogProvider interface {
	Recent(limit int, token string) []RequestLogEntry
	TrafficStats(minutes int, token string) []RequestLogBucket
	TopPaths(n int, token string) []RequestLogPathCount
	Subscribe() chan RequestLogEntry
	Unsubscribe(ch chan RequestLogEntry)
}

type contextKey string

const ctxKeyID contextKey = "keyID"

// Handler serves the user dashboard, admin dashboard, and their APIs.
type Handler struct {
	tunnels     TunnelProvider
	keys        KeyProvider
	config      ConfigProvider
	requestLog  RequestLogProvider
	adminToken  string
	domain      string
	version     string
	mux         *http.ServeMux
}

// NewHandler creates a Handler and registers all routes.
// requestLog may be nil; analytics endpoints will return empty results when nil.
func NewHandler(tunnels TunnelProvider, keys KeyProvider, config ConfigProvider,
	adminToken, domain, version string, requestLog RequestLogProvider) *Handler {
	h := &Handler{
		tunnels:    tunnels,
		keys:       keys,
		config:     config,
		requestLog: requestLog,
		adminToken: adminToken,
		domain:     domain,
		version:    version,
		mux:        http.NewServeMux(),
	}
	h.registerRoutes()
	return h
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; connect-src 'self'")
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

	// --- User analytics ---
	h.mux.HandleFunc("GET /_dashboard/api/logs", h.userAuth(h.handleUserLogs))
	h.mux.HandleFunc("GET /_dashboard/api/stats/traffic", h.userAuth(h.handleUserTraffic))
	h.mux.HandleFunc("GET /_dashboard/api/stats/top-paths", h.userAuth(h.handleUserTopPaths))
	// SSE: auth via ?key= query param (EventSource cannot send custom headers).
	h.mux.HandleFunc("GET /_dashboard/api/logs/stream", h.handleUserLogStream)

	// --- Admin Dashboard API ---
	h.mux.HandleFunc("GET /_admin/api/stats", h.adminOnly(h.handleAdminStats))
	h.mux.HandleFunc("GET /_admin/api/keys", h.adminOnly(h.handleAdminListKeys))
	h.mux.HandleFunc("DELETE /_admin/api/keys/{id}", h.adminOnly(h.handleAdminRevokeKey))
	h.mux.HandleFunc("GET /_admin/api/tunnels", h.adminOnly(h.handleAdminTunnels))
	h.mux.HandleFunc("POST /_admin/api/tunnels/{id}/kill", h.adminOnly(h.handleKill))
	h.mux.HandleFunc("GET /_admin/api/config", h.adminOnly(h.handleGetConfig))
	h.mux.HandleFunc("PUT /_admin/api/config", h.adminOnly(h.handleUpdateConfig))

	// --- Admin analytics ---
	h.mux.HandleFunc("GET /_admin/api/logs", h.adminOnly(h.handleAdminLogs))
	h.mux.HandleFunc("GET /_admin/api/stats/traffic", h.adminOnly(h.handleAdminTraffic))
	h.mux.HandleFunc("GET /_admin/api/stats/top-paths", h.adminOnly(h.handleAdminTopPaths))
	// SSE: auth via ?key= query param.
	h.mux.HandleFunc("GET /_admin/api/logs/stream", h.handleAdminLogStream)

	// --- Health endpoint (no auth — monitoring tools need unauthenticated access) ---
	h.mux.HandleFunc("GET /_healthz", h.handleHealthz)

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

// --- Health Handler ---

func (h *Handler) handleHealthz(w http.ResponseWriter, r *http.Request) {
	stats := h.tunnels.StatsForDashboard()
	keys := h.keys.ListKeys()
	writeJSON(w, map[string]any{
		"status":    "ok",
		"version":   h.version,
		"tunnels":   stats.TotalTunnels,
		"users":     len(keys),
		"bytes_in":  stats.BytesIn,
		"bytes_out": stats.BytesOut,
	})
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

// --- User analytics handlers ---

func (h *Handler) handleUserLogs(w http.ResponseWriter, r *http.Request) {
	keyID := r.Context().Value(ctxKeyID).(string)
	limit := 100
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 && n <= 500 {
			limit = n
		}
	}
	writeJSON(w, h.requestLog.Recent(limit, maskKey(keyID)))
}

func (h *Handler) handleUserTraffic(w http.ResponseWriter, r *http.Request) {
	keyID := r.Context().Value(ctxKeyID).(string)
	writeJSON(w, h.requestLog.TrafficStats(30, maskKey(keyID)))
}

func (h *Handler) handleUserTopPaths(w http.ResponseWriter, r *http.Request) {
	keyID := r.Context().Value(ctxKeyID).(string)
	writeJSON(w, h.requestLog.TopPaths(10, maskKey(keyID)))
}

// handleUserLogStream streams new request log entries to the authenticated user
// via Server-Sent Events. Auth is via ?key= query param (EventSource limitation).
func (h *Handler) handleUserLogStream(w http.ResponseWriter, r *http.Request) {
	key := r.URL.Query().Get("key")
	if key == "" || !h.keys.Validate(key) {
		writeError(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	h.serveSSE(w, r, maskKey(key))
}

// --- Admin analytics handlers ---

func (h *Handler) handleAdminLogs(w http.ResponseWriter, r *http.Request) {
	limit := 200
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 && n <= 1000 {
			limit = n
		}
	}
	writeJSON(w, h.requestLog.Recent(limit, ""))
}

func (h *Handler) handleAdminTraffic(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, h.requestLog.TrafficStats(60, ""))
}

func (h *Handler) handleAdminTopPaths(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, h.requestLog.TopPaths(20, ""))
}

// handleAdminLogStream streams all request log entries to the authenticated admin.
// Auth is via ?key= query param.
func (h *Handler) handleAdminLogStream(w http.ResponseWriter, r *http.Request) {
	key := r.URL.Query().Get("key")
	if key != h.adminToken {
		writeError(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	h.serveSSE(w, r, "")
}

// serveSSE writes a Server-Sent Events stream of RequestLogEntry JSON objects.
// filterToken filters by token when non-empty; empty means all entries.
func (h *Handler) serveSSE(w http.ResponseWriter, r *http.Request, filterToken string) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "SSE not supported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // disable nginx buffering

	// Send initial comment so EventSource transitions to "open" state immediately.
	fmt.Fprintf(w, ": connected\n\n")
	flusher.Flush()

	ch := h.requestLog.Subscribe()
	defer h.requestLog.Unsubscribe(ch)

	// Keepalive ticker prevents nginx/proxy from killing idle connections.
	keepalive := time.NewTicker(15 * time.Second)
	defer keepalive.Stop()

	for {
		select {
		case entry, ok := <-ch:
			if !ok {
				return
			}
			if filterToken != "" && entry.Token != filterToken {
				continue
			}
			data, _ := json.Marshal(entry)
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		case <-keepalive.C:
			fmt.Fprintf(w, ": ping\n\n")
			flusher.Flush()
		case <-r.Context().Done():
			return
		}
	}
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

// maskKey must produce the same format as server.maskToken
// so that dashboard log filtering matches stored log entries.
func maskKey(key string) string {
	if len(key) <= 8 {
		return "***"
	}
	return key[:4] + "..." + key[len(key)-4:]
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
