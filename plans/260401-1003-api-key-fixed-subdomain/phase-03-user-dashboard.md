# Phase 3: User Dashboard UI

## Priority: HIGH | Status: Not started | Effort: 4-5h

## Overview
Web UI cho user tự register, login bằng API key, quản lý subdomains và xem tunnels. Vanilla JS SPA, embedded trong Go binary.

## Pages

### Landing (`/_dashboard/`)
```
┌──────────────────────────────────┐
│         htn-tunnel               │
│   Self-hosted tunnel service     │
│                                  │
│  ┌────────────┐ ┌─────────────┐  │
│  │  Register  │ │   Login     │  │
│  └────────────┘ └─────────────┘  │
└──────────────────────────────────┘
```

### Register (`/_dashboard/#register`)
```
┌──────────────────────────────────┐
│  Create your tunnel account      │
│                                  │
│  Name:      [_______________]    │
│  Subdomain: [______].33.id.vn   │
│                                  │
│  [Check availability]            │
│  ✓ hoang.33.id.vn is available   │
│                                  │
│  [Create Account]                │
│                                  │
│  ┌─────────────────────────────┐ │
│  │ Your API Key (save this!):  │ │
│  │ htk_a1b2c3d4e5f6...        │ │
│  │ [Copy to clipboard]        │ │
│  │                             │ │
│  │ Quick start:                │ │
│  │ npm i -g htn-tunnel         │ │
│  │ htn-tunnel auth htk_a1b2... │ │
│  │ htn-tunnel http 3000        │ │
│  │   --subdomain hoang         │ │
│  └─────────────────────────────┘ │
└──────────────────────────────────┘
```

### Panel (`/_dashboard/#panel`) — after login
```
┌──────────────────────────────────┐
│  htn-tunnel    Welcome, Hoang    │
│                         [Logout] │
├──────────────────────────────────┤
│  Your Subdomains                 │
│  ┌──────────────────────┬──────┐ │
│  │ hoang.33.id.vn       │  ●   │ │
│  │ myapp.33.id.vn       │  ○   │ │
│  └──────────────────────┴──────┘ │
│  ● = tunnel active  ○ = offline  │
│                                  │
│  Add: [________] [+ Add]         │
│                                  │
│  Active Tunnels                  │
│  ┌──────────────────────────┐    │
│  │ hoang → localhost:3000   │    │
│  │ Uptime: 2h 15m           │    │
│  │ In: 1.2MB  Out: 45KB     │    │
│  └──────────────────────────┘    │
│                                  │
│  Your API Key                    │
│  htk_a1b2...d4e5  [Show full]   │
│                                  │
│  Quick Start                     │
│  htn-tunnel http 3000            │
│    --subdomain hoang             │
└──────────────────────────────────┘
```

## File Structure

```
internal/dashboard/
├── static/
│   ├── index.html          # Existing → becomes admin redirect
│   ├── style.css           # Shared styles
│   ├── app.js              # Existing admin dashboard
│   ├── user/
│   │   ├── index.html      # User SPA (register/login/panel)
│   │   ├── style.css       # User-specific styles
│   │   └── app.js          # User dashboard logic
│   └── admin/
│       ├── index.html      # Admin SPA
│       ├── style.css       # Admin-specific styles
│       └── app.js          # Admin dashboard logic
├── embed.go                # //go:embed static/*
└── handler.go              # Routes for both dashboards
```

## API Endpoints (User)

### `POST /_dashboard/api/register`
No auth required.
```json
// Request
{"name": "Hoang", "subdomain": "hoang"}

// Response 201
{"key": "htk_a1b2...", "name": "Hoang", "subdomains": ["hoang"]}

// Error 400
{"error": "subdomain 'hoang' is already taken"}
```

### `POST /_dashboard/api/login`
```json
// Request
{"key": "htk_a1b2..."}

// Response 200
{"name": "Hoang", "subdomains": ["hoang", "myapp"], "max_tunnels": 10}

// Error 401
{"error": "invalid or revoked key"}
```

### `GET /_dashboard/api/me`
Header: `Authorization: Bearer htk_a1b2...`
```json
{"name": "Hoang", "subdomains": ["hoang", "myapp"], "max_tunnels": 10, "created_at": "..."}
```

### `POST /_dashboard/api/subdomains`
Header: `Authorization: Bearer htk_a1b2...`
```json
// Request
{"subdomain": "newone"}

// Response 201
{"subdomains": ["hoang", "myapp", "newone"]}
```

### `DELETE /_dashboard/api/subdomains/{name}`
Header: `Authorization: Bearer htk_a1b2...`
```json
// Response 200
{"subdomains": ["hoang", "myapp"]}
```

### `GET /_dashboard/api/tunnels`
Header: `Authorization: Bearer htk_a1b2...`

Returns only tunnels belonging to this key.
```json
[
  {"subdomain": "hoang", "local_port": 3000, "uptime": "2h15m", "bytes_in": 1200000, "bytes_out": 45000}
]
```

## Implementation Steps

### 1. Restructure static files
Move existing dashboard to `static/admin/`, create `static/user/`.

### 2. Update handler routing
```go
func (h *Handler) registerRoutes() {
    // User dashboard
    userFS, _ := fs.Sub(staticFiles, "static/user")
    h.mux.Handle("GET /_dashboard/", http.StripPrefix("/_dashboard", http.FileServer(http.FS(userFS))))

    // User API
    h.mux.HandleFunc("POST /_dashboard/api/register", h.handleRegister)
    h.mux.HandleFunc("POST /_dashboard/api/login", h.handleLogin)
    h.mux.HandleFunc("GET /_dashboard/api/me", h.userAuth(h.handleMe))
    h.mux.HandleFunc("POST /_dashboard/api/subdomains", h.userAuth(h.handleAddSubdomain))
    h.mux.HandleFunc("DELETE /_dashboard/api/subdomains/{name}", h.userAuth(h.handleRemoveSubdomain))
    h.mux.HandleFunc("GET /_dashboard/api/tunnels", h.userAuth(h.handleUserTunnels))

    // Admin dashboard (Phase 4)
    adminFS, _ := fs.Sub(staticFiles, "static/admin")
    h.mux.Handle("GET /_admin/", http.StripPrefix("/_admin", http.FileServer(http.FS(adminFS))))
    // ... admin API routes
}
```

### 3. User auth middleware
```go
func (h *Handler) userAuth(next http.HandlerFunc) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        key := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
        if key == "" {
            http.Error(w, `{"error":"missing API key"}`, http.StatusUnauthorized)
            return
        }
        apiKey := h.keyProvider.GetKey(key)
        if apiKey == nil || !apiKey.Active {
            http.Error(w, `{"error":"invalid or revoked key"}`, http.StatusUnauthorized)
            return
        }
        // Store key in context for handlers
        ctx := context.WithValue(r.Context(), ctxKeyID, key)
        ctx = context.WithValue(ctx, ctxAPIKey, apiKey)
        next(w, r.WithContext(ctx))
    }
}
```

### 4. Build user SPA
Vanilla JS with hash-based routing (`#register`, `#login`, `#panel`). Key stored in `localStorage`.

## Files to Create
- `internal/dashboard/static/user/index.html`
- `internal/dashboard/static/user/style.css`
- `internal/dashboard/static/user/app.js`

## Files to Modify
- `internal/dashboard/handler.go` — user API routes + handlers + userAuth middleware
- `internal/dashboard/embed.go` — already embeds `static/*` recursively

## Success Criteria
- [ ] User can register via web form → gets API key
- [ ] User can login with API key → sees panel
- [ ] Panel shows subdomains with online/offline status
- [ ] User can add/remove subdomains from panel
- [ ] User sees active tunnels with stats
- [ ] Quick start instructions shown with user's key
- [ ] Key stored in localStorage for session persistence
