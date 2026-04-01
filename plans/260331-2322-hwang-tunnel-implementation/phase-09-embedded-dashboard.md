# Phase 9: Embedded Dashboard

## Context Links
- [Plan Overview](plan.md)
- [Phase 2: Server Core](phase-02-server-core-listener-auth.md)
- [Phase 8: Testing](phase-08-testing-hardening.md)

## Overview
- **Priority:** P2
- **Status:** completed
- **Effort:** 6h
- **Description:** Embedded web dashboard in the server binary using Go's `embed` package. Shows active tunnels, connection stats, bandwidth usage, and token management. No external frontend build tool — plain HTML/JS/CSS.

## Key Insights
- Go 1.16+ `embed` package bundles static files into the binary — zero runtime deps
- No React/Vue/build tooling — keep it simple with vanilla JS + Chart.js CDN
- Dashboard served on a separate port (default 8080) or under `/_dashboard/` path on the main domain
- Read-only view by default; admin actions (revoke token, kill tunnel) require admin token
- Server already tracks all needed data in TunnelManager — just expose via JSON API

## Requirements

### Functional
- Dashboard web UI served from embedded static files
- Real-time tunnel list: subdomain, type (HTTP/TCP), remote port, client IP, uptime, bandwidth
- Connection stats: total active tunnels, total connections, bandwidth/s
- Token overview: which tokens have active tunnels, tunnel count per token
- Auto-refresh every 5s (or WebSocket for real-time updates)
- Admin auth: dashboard protected by admin token (separate from tunnel tokens)
- Responsive design (works on mobile for quick checks)

### Non-Functional
- Dashboard adds < 500KB to binary size
- Page load < 200ms
- No external CDN dependencies (bundle Chart.js if used, or use lightweight alternative)

## Architecture

### Dashboard Endpoints
```
GET  /_dashboard/               → index.html (embedded)
GET  /_dashboard/static/*       → JS/CSS assets (embedded)
GET  /_dashboard/api/tunnels    → JSON: active tunnel list
GET  /_dashboard/api/stats      → JSON: aggregate stats
GET  /_dashboard/api/tokens     → JSON: token usage summary (admin only)
POST /_dashboard/api/tunnels/:id/kill  → Kill tunnel (admin only)
```

### Data Flow
```
TunnelManager (in-memory state)
     │
     ▼
Dashboard API handlers
     │ (JSON responses)
     ▼
Embedded HTML/JS UI
     │ (fetch every 5s)
     ▼
Browser
```

### UI Layout
```
┌─────────────────────────────────────────────┐
│  htn-tunnel Dashboard          [Admin Login] │
├─────────────────────────────────────────────┤
│  Active: 12 tunnels │ Bandwidth: 2.4 MB/s   │
│  HTTP: 8  │  TCP: 4  │ Connections: 47       │
├─────────────────────────────────────────────┤
│  Tunnel List                                 │
│  ┌───────────┬──────┬────────┬──────┬─────┐ │
│  │ Subdomain │ Type │ Port   │ Up   │ BW  │ │
│  ├───────────┼──────┼────────┼──────┼─────┤ │
│  │ myapp     │ HTTP │ —      │ 2h   │ 1MB │ │
│  │ —         │ TCP  │ 34567  │ 15m  │ 3MB │ │
│  └───────────┴──────┴────────┴──────┴─────┘ │
└─────────────────────────────────────────────┘
```

## Related Code Files

### Files to Create
- `internal/dashboard/handler.go` — HTTP handlers for dashboard API + static file serving
- `internal/dashboard/embed.go` — `//go:embed` directives for static files
- `internal/dashboard/static/index.html` — Dashboard HTML
- `internal/dashboard/static/app.js` — Dashboard JS (vanilla, fetch-based)
- `internal/dashboard/static/style.css` — Dashboard styles

### Files to Modify
- `internal/server/server.go` — mount dashboard routes
- `internal/server/tunnel-manager.go` — add methods to export stats
- `internal/config/config.go` — dashboard port, admin token config

## Implementation Steps

1. **Add dashboard config fields**
   - `DashboardEnabled bool` (default true)
   - `DashboardAddr string` (default `:8080`)
   - `AdminToken string` (separate from tunnel tokens)

2. **Add stats methods to TunnelManager**
   - `Stats() DashboardStats` — returns aggregate counts, bandwidth
   - `ListTunnels() []TunnelInfo` — returns active tunnel details
   - `TokenSummary() []TokenUsage` — tunnels per token
   - Track bytes transferred per tunnel (atomic counters on io.Copy wrappers)

3. **Create `internal/dashboard/embed.go`**
   ```go
   package dashboard

   import "embed"

   //go:embed static/*
   var StaticFiles embed.FS
   ```

4. **Create `internal/dashboard/handler.go`**
   - `Handler` struct with `tunnelManager`, `adminToken`
   - `NewHandler(tm *TunnelManager, adminToken string) *Handler`
   - Route registration on `http.ServeMux`
   - JSON API handlers: marshal TunnelManager data
   - Admin middleware: check `Authorization: Bearer <admin-token>` header
   - Static file handler: `http.FileServer(http.FS(StaticFiles))`

5. **Create `internal/dashboard/static/index.html`**
   - Clean, minimal HTML with embedded CSS
   - No framework — vanilla HTML + JS
   - Responsive layout (flexbox/grid)
   - Table for tunnel list, summary cards for stats

6. **Create `internal/dashboard/static/app.js`**
   - `fetchTunnels()` — GET `/api/tunnels`, render table
   - `fetchStats()` — GET `/api/stats`, update summary cards
   - Auto-refresh every 5s via `setInterval`
   - Admin mode: if admin token provided, show kill buttons + token stats
   - Admin token stored in `localStorage` after login prompt

7. **Create `internal/dashboard/static/style.css`**
   - Dark theme (developer-friendly)
   - Monospace font for data
   - Responsive breakpoints
   - Status indicators (green=active, red=dead)

8. **Integrate into server startup**
   - If `DashboardEnabled`: create dashboard handler, start HTTP server on `DashboardAddr`
   - Server.Shutdown() also stops dashboard server

9. **Add bandwidth tracking**
   - Wrap `io.Copy` in HTTP and TCP proxies with a counting wrapper
   - Atomic increment on `TunnelSession.BytesIn` / `TunnelSession.BytesOut`
   - Dashboard reads these counters for per-tunnel and aggregate stats

## Todo List
- [x] Add dashboard config fields
- [x] Add TunnelManager stats/export methods
- [x] Add bandwidth tracking (io.Copy wrapper with atomic counters)
- [x] Create embed.go with //go:embed
- [x] Create dashboard API handlers (tunnels, stats, token summary)
- [x] Create admin auth middleware
- [x] Create index.html with responsive layout
- [x] Create app.js with auto-refresh + admin mode
- [x] Create style.css (dark theme)
- [x] Integrate dashboard into server lifecycle
- [x] Test: dashboard loads, API returns correct data, admin auth works

## Success Criteria
- `http://server:8080/_dashboard/` shows tunnel list and stats
- Data refreshes every 5s
- Admin login enables kill tunnel + token stats
- Dashboard adds < 500KB to binary
- Works on mobile viewport
- Dashboard unavailable when `DashboardEnabled=false`

## Risk Assessment
- **Risk:** Dashboard port conflicts with other services
  - **Mitigation:** Configurable port. Can also serve under path on main domain.
- **Risk:** Dashboard exposes sensitive info (active tunnels, IPs)
  - **Mitigation:** Admin token required for sensitive endpoints. Basic tunnel list is non-sensitive.

## Security Considerations
- Admin token separate from tunnel tokens
- No CORS on dashboard API (same-origin only)
- CSP headers on HTML responses
- No inline scripts (all JS in separate file)
- Admin token in `Authorization` header, not URL params

## Next Steps
→ Dashboard is the final feature phase. After this: Phase 8 testing covers dashboard endpoints.
