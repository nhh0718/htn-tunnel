# API Key + User/Admin Dashboard Plan

## Problem
1. Auth tokens hardcoded — không scale
2. Subdomain random mỗi lần connect
3. Không có UI quản lý cho user lẫn admin

## Solution
2 dashboard tách biệt:
- **User Dashboard** (`/_dashboard/`) — tự register, login bằng API key, quản lý subdomain/tunnel
- **Admin Dashboard** (`/_admin/`) — login bằng admin key, quản lý users, config server

## Architecture

```
Port 1807 (HTTP):
├── /_dashboard/              # User Dashboard
│   ├── /                     # Landing: register hoặc login
│   ├── /register             # Form tạo key + claim subdomain
│   ├── /login                # Form nhập API key
│   └── /panel                # Sau login: subdomains, tunnels, key info
│
├── /_admin/                  # Admin Dashboard
│   ├── /                     # Login bằng admin key
│   └── /panel                # Users, tunnels, server config
│
├── /_dashboard/api/          # User API (auth: API key header)
│   ├── POST /register        # Tạo key + subdomain
│   ├── POST /login           # Validate key, trả user info
│   ├── GET  /me              # Key info + subdomains
│   ├── POST /subdomains      # Claim subdomain mới
│   ├── DELETE /subdomains/:name  # Release subdomain
│   └── GET  /tunnels         # Active tunnels của user
│
└── /_admin/api/              # Admin API (auth: admin key header)
    ├── GET  /keys            # List all keys
    ├── DELETE /keys/:id      # Revoke key
    ├── GET  /tunnels         # All active tunnels
    ├── GET  /config          # Server config (masked secrets)
    ├── PUT  /config          # Update config (domain, limits...)
    └── GET  /stats           # Server stats
```

## User Flow
```
1. User mở https://33.id.vn:1807/_dashboard/
2. Click "Register" → nhập name + subdomain → nhận API key
3. Copy key → dùng trong CLI:
   htn-tunnel auth htk_xxx --server 33.id.vn:4443
   htn-tunnel http 3000 --subdomain hoang
4. Quay lại dashboard login bằng key → quản lý subdomains, xem tunnels
```

## Admin Flow
```
1. Admin mở https://33.id.vn:1807/_admin/
2. Login bằng admin_token từ server.yaml
3. Xem tất cả users, tunnels
4. Revoke key xấu
5. Đổi config (domain, rate limits...) → server hot-reload
```

## Phases

| # | Description | Effort |
|---|-------------|--------|
| 1 | [Key store backend](./phase-01-api-key-store.md) | 3-4h |
| 2 | [Registration + subdomain API](./phase-02-registration-api.md) | 3-4h |
| 3 | [User Dashboard UI](./phase-03-user-dashboard.md) | 4-5h |
| 4 | [Admin Dashboard UI](./phase-04-admin-dashboard.md) | 4-5h |
| 5 | [Client CLI update](./phase-05-client-cli.md) | 1-2h |

Total: ~15-20h

## Key Decisions
- **Single HTTP server** on port 1807 serves both dashboards
- **No session/cookie** — API key in `Authorization` header (stateless)
- **User dashboard is SPA** — vanilla JS, no framework (embedded in binary)
- **Admin config edit** → write to yaml + signal server to hot-reload
- **Backward compatible** — old `tokens` config still works
