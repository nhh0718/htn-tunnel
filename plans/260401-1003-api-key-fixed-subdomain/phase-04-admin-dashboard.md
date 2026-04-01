# Phase 4: Admin Dashboard UI

## Priority: MEDIUM | Status: Not started | Effort: 4-5h

## Overview
Admin dashboard for server management: view all users/tunnels, revoke keys, edit server config with hot-reload.

## Pages

### Login (`/_admin/`)
```
┌──────────────────────────────────┐
│     htn-tunnel Admin             │
│                                  │
│  Admin Key: [________________]   │
│  [Login]                         │
└──────────────────────────────────┘
```

### Panel (`/_admin/#panel`)
```
┌──────────────────────────────────┐
│  htn-tunnel Admin       [Logout] │
├──────────────────────────────────┤
│  Stats                           │
│  ┌───────┐ ┌───────┐ ┌────────┐ │
│  │  12   │ │   8   │ │  4     │ │
│  │ Users │ │ HTTP  │ │ TCP    │ │
│  └───────┘ └───────┘ └────────┘ │
│                                  │
│  [Users] [Tunnels] [Config]      │
├──────────────────────────────────┤
│  Users tab:                      │
│  ┌─────┬────────┬──────┬──────┐  │
│  │Key  │Name    │Subs  │Action│  │
│  ├─────┼────────┼──────┼──────┤  │
│  │htk..│Hoang   │hoang │Revoke│  │
│  │htk..│TeamDev │dev   │Revoke│  │
│  └─────┴────────┴──────┴──────┘  │
│                                  │
│  Config tab:                     │
│  Domain:    [33.id.vn        ]   │
│  Rate limit:[100  ] req/min      │
│  Max tunnels/key: [10  ]         │
│  Allow registration: [✓]        │
│  [Save & Apply]                  │
└──────────────────────────────────┘
```

## API Endpoints (Admin)

All require `Authorization: Bearer <admin_token>`.

### `GET /_admin/api/stats`
```json
{"active_http": 8, "active_tcp": 4, "total_keys": 12, "bytes_in": 50000000, "bytes_out": 12000000}
```

### `GET /_admin/api/keys`
```json
[
  {"key_preview": "htk_a1b2...d4e5", "name": "Hoang", "subdomains": ["hoang","myapp"], "max_tunnels": 5, "active": true, "created_at": "..."},
  {"key_preview": "htk_b2c3...e5f6", "name": "TeamDev", "subdomains": ["dev"], "max_tunnels": 10, "active": true, "created_at": "..."}
]
```

### `DELETE /_admin/api/keys/{key_preview}`
Find key by preview match, revoke it.
```json
{"status": "revoked"}
```

### `GET /_admin/api/tunnels`
All active tunnels across all users.
```json
[
  {"id": "...", "type": "http", "subdomain": "hoang", "local_port": 3000, "token_prefix": "htk_a1b2", "uptime": "2h", "bytes_in": 1200000}
]
```

### `GET /_admin/api/config`
Editable server config (secrets masked).
```json
{
  "domain": "33.id.vn",
  "rate_limit": 100,
  "global_rate_limit": 1000,
  "max_tunnels_per_token": 10,
  "allow_registration": true,
  "tcp_port_range": [10000, 65535]
}
```

### `PUT /_admin/api/config`
Update config + hot-reload.
```json
{"domain": "33.id.vn", "rate_limit": 200, "allow_registration": false}
```

Server writes to yaml, sends SIGHUP to self for hot-reload of non-TLS settings.

## Implementation Steps

### 1. Create admin static files
- `internal/dashboard/static/admin/index.html` — SPA with tabs
- `internal/dashboard/static/admin/style.css` — admin theme
- `internal/dashboard/static/admin/app.js` — tab switching, CRUD, config form

### 2. Add admin API routes
```go
// In handler.go registerRoutes():
h.mux.HandleFunc("GET /_admin/api/stats", h.adminOnly(h.handleAdminStats))
h.mux.HandleFunc("GET /_admin/api/keys", h.adminOnly(h.handleAdminListKeys))
h.mux.HandleFunc("DELETE /_admin/api/keys/{id}", h.adminOnly(h.handleAdminRevokeKey))
h.mux.HandleFunc("GET /_admin/api/tunnels", h.adminOnly(h.handleAdminTunnels))
h.mux.HandleFunc("GET /_admin/api/config", h.adminOnly(h.handleGetConfig))
h.mux.HandleFunc("PUT /_admin/api/config", h.adminOnly(h.handleUpdateConfig))
```

### 3. Config update handler
```go
func (h *Handler) handleUpdateConfig(w http.ResponseWriter, r *http.Request) {
    var updates map[string]any
    json.NewDecoder(r.Body).Decode(&updates)

    // Whitelist editable fields only
    allowed := []string{"domain", "rate_limit", "global_rate_limit",
        "max_tunnels_per_token", "allow_registration", "tcp_port_range"}

    // Apply to runtime config
    h.configProvider.UpdateConfig(updates, allowed)

    // Persist to yaml
    h.configProvider.SaveConfig()

    writeJSON(w, map[string]string{"status": "updated"})
}
```

### 4. ConfigProvider interface
```go
type ConfigProvider interface {
    GetEditableConfig() map[string]any
    UpdateConfig(updates map[string]any, allowed []string) error
    SaveConfig() error
}
```

### 5. Serve admin static files
```go
adminFS, _ := fs.Sub(staticFiles, "static/admin")
h.mux.Handle("GET /_admin/", http.StripPrefix("/_admin", http.FileServer(http.FS(adminFS))))
```

## Files to Create
- `internal/dashboard/static/admin/index.html`
- `internal/dashboard/static/admin/style.css`
- `internal/dashboard/static/admin/app.js`

## Files to Modify
- `internal/dashboard/handler.go` — admin routes, ConfigProvider interface
- `internal/server/server.go` — implement ConfigProvider, pass to handler

## Security
- Admin key never exposed in config API response
- DNS API token never exposed
- Only whitelisted fields editable
- Config changes logged

## Success Criteria
- [ ] Admin login with admin_token works
- [ ] Users tab shows all keys, revoke works
- [ ] Tunnels tab shows all active tunnels
- [ ] Config tab shows editable settings
- [ ] Config save persists to yaml and applies at runtime
- [ ] Non-admin cannot access /_admin/ API
