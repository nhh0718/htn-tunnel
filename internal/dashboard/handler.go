package dashboard

import (
	"encoding/json"
	"io/fs"
	"net/http"
	"strings"
	"time"
)

// TunnelStats is the JSON shape returned by GET /_dashboard/api/tunnels.
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

// AggregateStats is the JSON shape returned by GET /_dashboard/api/stats.
type AggregateStats struct {
	ActiveHTTP   int   `json:"active_http"`
	ActiveTCP    int   `json:"active_tcp"`
	TotalTunnels int   `json:"total_tunnels"`
	BytesIn      int64 `json:"bytes_in"`
	BytesOut     int64 `json:"bytes_out"`
}

// TunnelProvider is the interface the server's TunnelManager satisfies.
// Using an interface keeps the dashboard package free of server-package imports.
type TunnelProvider interface {
	ListTunnelsForDashboard() []TunnelStats
	StatsForDashboard() AggregateStats
	KillTunnelByID(id string) bool
}

// Handler serves the dashboard HTTP API and embedded static files.
type Handler struct {
	provider   TunnelProvider
	adminToken string
	mux        *http.ServeMux
}

// NewHandler creates a Handler and registers routes on its own ServeMux.
func NewHandler(provider TunnelProvider, adminToken string) *Handler {
	h := &Handler{
		provider:   provider,
		adminToken: adminToken,
		mux:        http.NewServeMux(),
	}
	h.registerRoutes()
	return h
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Security headers on all dashboard responses.
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'")
	h.mux.ServeHTTP(w, r)
}

func (h *Handler) registerRoutes() {
	// API routes
	h.mux.HandleFunc("GET /_dashboard/api/tunnels", h.handleTunnels)
	h.mux.HandleFunc("GET /_dashboard/api/stats", h.handleStats)
	h.mux.HandleFunc("POST /_dashboard/api/tunnels/{id}/kill", h.adminOnly(h.handleKill))

	// Static files served from embedded FS under /_dashboard/
	// Use fs.Sub to serve directly from "static/" subdirectory, avoiding redirect loops.
	sub, _ := fs.Sub(staticFiles, "static")
	h.mux.Handle("GET /_dashboard/", http.StripPrefix("/_dashboard", http.FileServer(http.FS(sub))))
}

func (h *Handler) handleTunnels(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, h.provider.ListTunnelsForDashboard())
}

func (h *Handler) handleStats(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, h.provider.StatsForDashboard())
}

func (h *Handler) handleKill(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		http.Error(w, "missing tunnel id", http.StatusBadRequest)
		return
	}
	if !h.provider.KillTunnelByID(id) {
		http.Error(w, "tunnel not found", http.StatusNotFound)
		return
	}
	writeJSON(w, map[string]string{"status": "killed"})
}

// adminOnly is middleware that requires a valid admin token in the
// Authorization: Bearer <token> header.
func (h *Handler) adminOnly(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if h.adminToken == "" {
			http.Error(w, "admin token not configured", http.StatusForbidden)
			return
		}
		auth := r.Header.Get("Authorization")
		token := strings.TrimPrefix(auth, "Bearer ")
		if token != h.adminToken {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v) //nolint:errcheck
}
