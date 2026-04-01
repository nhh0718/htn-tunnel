# htn-tunnel — Deployment & Usage Guide

## Table of Contents

1. [Tổng quan kiến trúc](#1-tổng-quan-kiến-trúc)
2. [Chạy local (DevMode)](#2-chạy-local-devmode)
3. [Triển khai lên VPS](#3-triển-khai-lên-vps)
4. [Setup DNS & Cloudflare](#4-setup-dns--cloudflare)
5. [Setup nginx (chạy chung VPS)](#5-setup-nginx-chạy-chung-vps)
6. [Cài đặt & dùng client](#6-cài-đặt--dùng-client)
7. [Dashboard quản trị](#7-dashboard-quản-trị)
8. [Cấu hình nâng cao](#8-cấu-hình-nâng-cao)
9. [Troubleshooting](#9-troubleshooting)

---

## 1. Tổng quan kiến trúc

### Kiến trúc khi có nginx trên cùng VPS

```
Internet → Port 443 (nginx stream module, SNI routing)
├── *.33.id.vn      → TLS passthrough → htn-tunnel:8443
└── * (mọi domain)  → nginx HTTP:4430 (certbot TLS termination)

htn-tunnel ports:
├── :4443  — Control plane (yamux, client connections)
├── :8443  — HTTP tunnel proxy (certmagic wildcard TLS)
├── :8444  — HTTP redirect → HTTPS
└── :1807  — Dashboard

nginx ports:
├── :443   — Stream module (SNI routing only, no TLS termination)
├── :4430  — HTTPS cho tất cả site khác (certbot)
└── :80    — HTTP redirect
```

### Luồng request

```
Browser → https://abc123.33.id.vn
       → DNS → VPS IP:443
       → nginx stream (SNI = *.33.id.vn) → passthrough → htn-tunnel:8443
       → htn-tunnel TLS handshake (certmagic wildcard cert)
       → HTTP request → extract subdomain "abc123"
       → lookup yamux session → open stream → forward to client
       → client dials localhost:3000 → proxy response back
```

---

## 2. Chạy local (DevMode)

### Prerequisites

- Go 1.22+

### Build

```bash
git clone <repo> && cd htn-tunnel
go build -o bin/htn-server ./cmd/server
go build -o bin/htn-tunnel ./cmd/client
```

### Config server

```yaml
# server.yaml
listen_addr: ":4443"
domain: "localhost"
tokens:
  - "dev-token-12345"
dev_mode: true
dashboard_enabled: true
dashboard_addr: ":8080"
```

### Chạy

```bash
# Terminal 1: server
./bin/htn-server --config server.yaml

# Terminal 2: client
./bin/htn-tunnel auth dev-token-12345 --server localhost:4443
./bin/htn-tunnel http 3000
```

---

## 3. Triển khai lên VPS

### 3.1 Yêu cầu

| Tài nguyên | Tối thiểu |
|------------|-----------|
| RAM        | 512 MB    |
| CPU        | 1 vCPU    |
| OS         | Ubuntu 22.04 / Debian 12 |
| Ports mở   | 443, 4443, 10000-65535 |

### 3.2 Build binary

```bash
# Trên máy dev (cross-compile cho Linux)
GOOS=linux GOARCH=amd64 go build -o bin/htn-server ./cmd/server
```

### 3.3 Upload lên VPS

```bash
scp bin/htn-server root@<VPS_IP>:/usr/local/bin/htn-server
```

### 3.4 Cấp quyền bind port < 1024

```bash
chmod +x /usr/local/bin/htn-server
setcap 'cap_net_bind_service=+ep' /usr/local/bin/htn-server
```

### 3.5 Tạo thư mục cert

```bash
mkdir -p /var/lib/htn-tunnel/certs
chown nobody:nogroup /var/lib/htn-tunnel/certs
```

### 3.6 Tạo config

```bash
mkdir -p /etc/htn-tunnel
cat > /etc/htn-tunnel/server.yaml << 'EOF'
listen_addr: ":4443"
domain: "33.id.vn"
email: "your@email.com"

tokens:
  - "your-secure-token"

max_tunnels_per_token: 10
rate_limit: 100
global_rate_limit: 1000
tcp_port_range: [10000, 65535]
cert_storage: "/var/lib/htn-tunnel/certs"

dns_provider: "cloudflare"
dns_api_token: "your_cloudflare_api_token"

dev_mode: false
dashboard_enabled: true
dashboard_addr: ":1807"
admin_token: "your_admin_token"

# Khi chạy chung nginx trên port 443 → dùng port khác cho HTTP proxy
http_proxy_addr: ":8443"
http_redirect_addr: ":8444"
EOF
```

### 3.7 Tạo systemd service

```bash
cat > /etc/systemd/system/htn-tunnel.service << 'EOF'
[Unit]
Description=htn-tunnel server
After=network.target

[Service]
Type=simple
User=nobody
ExecStart=/usr/local/bin/htn-server --config /etc/htn-tunnel/server.yaml
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload
systemctl enable --now htn-tunnel
```

> **Lưu ý:** KHÔNG đặt `Environment=HTN_DNS_API_TOKEN=...` trong systemd nếu đã có trong server.yaml. Env var override yaml → dễ bị conflict.

### 3.8 Verify

```bash
systemctl status htn-tunnel
journalctl -u htn-tunnel -n 20 --no-pager
ss -tlnp | grep htn
```

---

## 4. Setup DNS & Cloudflare

### 4.1 Tạo DNS records

Trên Cloudflare, thêm 2 A records:

| Type | Name   | Value      | Proxy           |
|------|--------|------------|-----------------|
| A    | `33.id.vn`   | `<VPS IP>` | **DNS only** (grey cloud) |
| A    | `*.33.id.vn` | `<VPS IP>` | **DNS only** (grey cloud) |

> **QUAN TRỌNG:** Phải dùng **DNS only** (grey cloud). Cloudflare proxy (orange cloud) không tương thích với TLS passthrough.

### 4.2 Tạo Cloudflare API Token

1. Cloudflare → **My Profile** → **API Tokens**
2. **Create Token** → **Edit zone DNS** template
3. Permissions: `Zone / DNS / Edit`
4. Zone Resources: `Include / Specific zone / <your-zone>`
5. Copy token → dùng làm `dns_api_token` trong server.yaml

### 4.3 Verify cert

Lần đầu chạy, certmagic tự request wildcard cert `*.33.id.vn` qua DNS-01 challenge (30-60 giây):

```bash
journalctl -u htn-tunnel --no-pager | grep -i cert
# Tìm: "loaded wildcard cert from disk" hoặc certmagic issued cert
```

---

## 5. Setup nginx (chạy chung VPS)

Khi VPS đã có nginx phục vụ các site khác, cần cấu hình **nginx stream module** để phân luồng SNI trên port 443.

### 5.1 Chuyển tất cả nginx site từ port 443 sang 4430

```bash
# Sửa tất cả listen 443 → 4430 trong các config file nginx
sed -i 's/listen 443 ssl/listen 4430 ssl/g' /etc/nginx/conf.d/*.conf
sed -i 's/listen \[::\]:443 ssl/listen [::]:4430 ssl/g' /etc/nginx/conf.d/*.conf

# Kiểm tra không còn listen 443
grep "listen.*443" /etc/nginx/conf.d/*.conf
```

### 5.2 Thêm stream block vào nginx.conf

Sửa `/etc/nginx/nginx.conf`, thêm **ngoài** block `http { }`:

```nginx
stream {
    map $ssl_preread_server_name $backend {
        dashboard.33.id.vn   127.0.0.1:1807;    # Dashboard (HTTP, via proxy)
        ~^.*\.33\.id\.vn$    127.0.0.1:8443;    # htn-tunnel (TLS passthrough)
        default              127.0.0.1:4430;     # nginx HTTP (TLS termination)
    }
    server {
        listen 443;
        ssl_preread on;
        proxy_pass $backend;
    }
}
```

**Chú ý:** Dashboard chạy trên HTTP (:1807), nhưng clients kết nối qua HTTPS (443). nginx stream module route SNI `dashboard.33.id.vn` tới :1807, bỏ qua TLS handshake (vì backend là HTTP). Clients sẽ thấy certificate warning — điều này bình thường.

### 5.3 Reload nginx

```bash
nginx -t && systemctl reload nginx
```

### 5.4 Verify

```bash
# nginx stream đang listen 443
ss -tlnp | grep :443

# Test tunnel TLS
curl -vk https://127.0.0.1:8443/ -H "Host: test.33.id.vn" 2>&1 | tail -5
# Expect: "tunnel not found for subdomain: test" (= TLS OK, tunnel chỉ chưa có)
```

---

## 6. Cài đặt & dùng client

### 6.1 Build client

```bash
# Linux/Mac
go build -o htn-tunnel ./cmd/client

# Windows
go build -o htn-tunnel.exe ./cmd/client
```

### 6.2 Lưu config (một lần)

```bash
htn-tunnel auth <token> --server 33.id.vn:4443
# → Lưu vào ~/.htn-tunnel/config.yaml
```

### 6.3 HTTP tunnel

```bash
# Random subdomain
htn-tunnel http 3000

# Output:
#   Tunnel:    https://abc123.33.id.vn → localhost:3000
#   Status:    connected

# Subdomain cố định
htn-tunnel http 3000 --subdomain myapp
# → https://myapp.33.id.vn
```

### 6.4 TCP tunnel

```bash
htn-tunnel tcp 5432
# Output:
#   Tunnel:    tcp://remote:12345 → localhost:5432
```

### 6.5 Override (không cần config file)

```bash
htn-tunnel http 3000 --server 33.id.vn:4443 --token your-token
```

### 6.6 Auto-reconnect

Client tự động reconnect khi mất kết nối:
- Backoff: 1s → 2s → 4s → ... → 60s (cap)
- Mỗi lần reconnect có thể tạo subdomain mới
- Subdomain hiện tại được in ra console

---

## 7. Dashboard quản trị

Dashboard chạy trên port 1807 (HTTP).

### Truy cập từ bên ngoài

Nếu port 1807 bị firewall:
```bash
# SSH tunnel
ssh -L 1807:localhost:1807 root@<VPS_IP>
# → http://localhost:1807/_dashboard/
```

Hoặc mở firewall:
```bash
ufw allow 1807/tcp
# → http://<VPS_IP>:1807/_dashboard/
```

### API endpoints

```bash
# Stats (public)
curl http://localhost:1807/_dashboard/api/stats

# Tunnel list (public)
curl http://localhost:1807/_dashboard/api/tunnels

# Kill tunnel (cần admin token)
curl -X POST http://localhost:1807/_dashboard/api/tunnels/<id>/kill \
  -H "Authorization: Bearer <admin_token>"
```

---

## 8. Cấu hình nâng cao

### Tất cả tùy chọn server.yaml

| Field | Default | Mô tả |
|-------|---------|-------|
| `listen_addr` | `:4443` | Control plane TLS port |
| `domain` | — | Base domain (vd: `33.id.vn`) |
| `email` | — | Email cho Let's Encrypt |
| `tokens` | — | Danh sách auth token |
| `max_tunnels_per_token` | `10` | Giới hạn tunnel / token |
| `rate_limit` | `100` | Requests/min per token |
| `global_rate_limit` | `1000` | Requests/min toàn server |
| `tcp_port_range` | `[10000, 65535]` | Range port cho TCP tunnels |
| `cert_storage` | `/var/lib/htn-tunnel/certs` | Nơi lưu cert |
| `dns_provider` | — | Provider DNS-01 (`cloudflare`) |
| `dns_api_token` | — | API token cho DNS provider |
| `dev_mode` | `false` | Self-signed cert (local only) |
| `dashboard_enabled` | `true` | Bật dashboard |
| `dashboard_addr` | `:8080` | Dashboard port |
| `admin_token` | — | Token cho admin API |
| `http_proxy_addr` | `:443` | HTTPS proxy port (đổi khi có nginx) |
| `http_redirect_addr` | `:80` | HTTP redirect port |

### Environment variables (override yaml)

| Env var | Yaml field |
|---------|-----------|
| `HTN_LISTEN_ADDR` | `listen_addr` |
| `HTN_DOMAIN` | `domain` |
| `HTN_EMAIL` | `email` |
| `HTN_TOKENS` | `tokens` (comma-separated) |
| `HTN_DNS_PROVIDER` | `dns_provider` |
| `HTN_DNS_API_TOKEN` | `dns_api_token` |
| `HTN_ADMIN_TOKEN` | `admin_token` |
| `HTN_DEV_MODE` | `dev_mode` |
| `HTN_CERT_STORAGE` | `cert_storage` |
| `HTN_DASHBOARD_ADDR` | `dashboard_addr` |

### Generate token an toàn

```bash
openssl rand -hex 32
```

---

## 9. Troubleshooting

### Deploy lại binary mới

```bash
# Build
GOOS=linux GOARCH=amd64 go build -o bin/htn-server ./cmd/server

# Upload
scp bin/htn-server root@<VPS_IP>:/usr/local/bin/htn-server

# Restart trên VPS
setcap 'cap_net_bind_service=+ep' /usr/local/bin/htn-server
systemctl restart htn-tunnel
```

### Xem logs

```bash
journalctl -u htn-tunnel -n 50 --no-pager       # Last 50 lines
journalctl -u htn-tunnel -f                       # Follow live
journalctl -u htn-tunnel -f | grep -i "error"     # Errors only
```

### Client cứ disconnect

```bash
# Check server logs
journalctl -u htn-tunnel -n 30 --no-pager | grep "registered\|ended"
```

Nếu thấy pattern `registered → ended (EOF)` lặp lại: client cũ. Build lại client mới.

### nginx không start (port 443 conflict)

```bash
# Tìm process chiếm port 443
ss -tlnp | grep :443
ps aux | grep nginx | grep -v grep

# Kill rồi start lại
pkill -9 nginx
systemctl start nginx
```

### Cert không load được

```bash
# Check cert files
ls -la /var/lib/htn-tunnel/certs/certificates/acme-v02.api.letsencrypt.org-directory/

# Check permissions (phải owned by nobody)
ls -la /var/lib/htn-tunnel/certs/

# Fix permissions
chown -R nobody:nogroup /var/lib/htn-tunnel/certs
```

### WebSocket fail qua tunnel

Server rewrite Origin header cho WebSocket requests. Nếu vẫn lỗi, thêm domain vào Next.js config:

```js
// next.config.js
module.exports = {
  allowedDevOrigins: ['*.33.id.vn'],
}
```
