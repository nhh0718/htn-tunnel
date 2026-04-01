# htn-tunnel

[![Release](https://img.shields.io/github/v/release/nhh0718/htn-tunnel?style=flat-square)](https://github.com/nhh0718/htn-tunnel/releases)
[![npm](https://img.shields.io/npm/v/htn-tunnel?style=flat-square)](https://www.npmjs.com/package/htn-tunnel)
[![Go](https://img.shields.io/github/go-mod/go-version/nhh0718/htn-tunnel?style=flat-square)](https://go.dev/)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue?style=flat-square)](LICENSE)

Self-hosted tunneling tool — expose localhost to the internet via a public VPS.

[English](#english) | [Tieng Viet](#tieng-viet)

---

## English

### What is htn-tunnel?

A self-hosted alternative to ngrok/Cloudflare Tunnel. Run the server on your VPS, install the client on your machine, and expose any local port to the internet with a public HTTPS URL.

```
You (localhost:3000) --tunnel--> Your VPS --internet--> https://myapp.your-domain.com
```

### Features

- **HTTP tunnels** — HTTPS subdomain routing (random or permanent per user)
- **TCP tunnels** — raw port forwarding for any TCP service (databases, SSH, etc.)
- **Auto-TLS** — wildcard Let's Encrypt cert via DNS-01 (Cloudflare)
- **WebSocket support** — full proxy with Origin rewrite (Next.js HMR, dev servers)
- **API key management** — self-service registration via web dashboard
- **Fixed subdomains** — claim a subdomain, it's permanently yours
- **User dashboard** — register, login, manage subdomains
- **Admin dashboard** — manage users, tunnels, edit server config
- **Auto-reconnect** — heartbeat keepalive + exponential backoff
- **Single binary** — no runtime dependencies, cross-platform

---

### Getting Started (Step by Step)

#### Step 1: Install the client

Choose one method:

**Option A: Go install** (if you have Go installed)

```bash
go install github.com/nhh0718/htn-tunnel/cmd/htn-tunnel@latest
```

**Option B: npm** (if you have Node.js installed)

```bash
npm install -g htn-tunnel
```

**Option C: Download binary** from [GitHub Releases](https://github.com/nhh0718/htn-tunnel/releases)

```bash
# Linux (amd64)
curl -L https://github.com/nhh0718/htn-tunnel/releases/latest/download/htn-tunnel_linux_amd64.tar.gz | tar xz
sudo mv htn-tunnel /usr/local/bin/

# macOS (Apple Silicon)
curl -L https://github.com/nhh0718/htn-tunnel/releases/latest/download/htn-tunnel_darwin_arm64.tar.gz | tar xz
sudo mv htn-tunnel /usr/local/bin/

# Windows — download .zip from the Releases page and add to PATH
```

#### Step 2: Register an account

Open the dashboard in your browser:

```
https://<server-address>:1807/_dashboard/
```

1. Click **Register**
2. Enter your **name** and desired **subdomain** (e.g. `myapp`)
3. Click **Create Account**
4. **Copy your API key** (starts with `htk_...`) — save it somewhere safe!

> You only need to register once. Your subdomain is permanently reserved.

#### Step 3: Save your API key

```bash
htn-tunnel auth htk_your_key_here --server your-server:4443
```

This saves the key to `~/.htn-tunnel/config.yaml`. You only need to do this once.

#### Step 4: Start a tunnel

```bash
# Expose a local web server on port 3000
htn-tunnel http 3000 --subdomain myapp
```

Output:
```
htn-tunnel vdev

  Tunnel:    https://myapp.your-domain.com -> localhost:3000
  Status:    connected
```

Open `https://myapp.your-domain.com` in your browser — it shows your local app!

#### Step 5: (Optional) TCP tunnel

```bash
# Expose PostgreSQL on port 5432
htn-tunnel tcp 5432
```

Output:
```
  Tunnel:    tcp://remote:34567 -> localhost:5432
  Status:    connected
```

Connect from anywhere: `psql -h your-server -p 34567 -U postgres`

---

### Managing Your Account

#### Dashboard

Open `https://<server>:1807/_dashboard/` and login with your API key to:
- View your subdomains (online/offline status)
- Add or remove subdomains
- See active tunnel stats (uptime, bandwidth)
- Copy quick-start commands

#### CLI status

```bash
htn-tunnel status
```

Shows your account info, owned subdomains, and server connection.

---

### CLI Reference

| Command | Description |
|---------|-------------|
| `htn-tunnel http <port>` | Create HTTP tunnel to localhost:port |
| `htn-tunnel tcp <port>` | Create TCP tunnel to localhost:port |
| `htn-tunnel auth <token>` | Save API key to config |
| `htn-tunnel status` | Show account info |

**Flags:**

| Flag | Description |
|------|-------------|
| `--subdomain <name>` | Request a fixed subdomain (HTTP only) |
| `--server <host:port>` | Override server address |
| `--token <key>` | Override auth token |

---

### Server Setup (for VPS admins)

See the full [Deployment Guide](docs/deployment-guide.md) for:
- VPS requirements and setup
- DNS configuration (Cloudflare)
- nginx coexistence (stream SNI routing)
- systemd service
- TLS certificate management

Quick overview:
```bash
# Build
GOOS=linux GOARCH=amd64 go build -o htn-server ./cmd/server

# Upload + configure
scp htn-server root@your-vps:/usr/local/bin/
# Edit /etc/htn-tunnel/server.yaml — see deployment guide

# Start
systemctl enable --now htn-tunnel
```

**Admin dashboard:** `https://<server>:1807/_admin/` — login with admin key to manage users, tunnels, and server config.

### Architecture

```
Internet -> Port 443 (nginx stream, SNI routing)
|-- *.domain.com  -> htn-tunnel:8443 (TLS passthrough)
|-- * (default)   -> nginx:4430 (other sites)

htn-tunnel server:
  :4443  Control plane (client connections, yamux)
  :8443  HTTP tunnel proxy (certmagic wildcard TLS)
  :8444  HTTP redirect
  :1807  Dashboard (user /_dashboard/ + admin /_admin/)
```

### Documentation

- [Deployment Guide](docs/deployment-guide.md) — Full VPS setup
- [Publish Guide](docs/publish-guide.md) — npm + GitHub Releases publishing
- [Commands Reference](docs/commands.md) — All VPS management commands

### License

[MIT](LICENSE)

---

## Tiếng Việt

### htn-tunnel là gì?

Công cụ tunnel tự host — thay thế ngrok/Cloudflare Tunnel. Chạy server trên VPS, cài client trên máy bạn, expose bất kỳ port local nào ra internet với URL HTTPS.

```
Máy bạn (localhost:3000) --tunnel--> VPS của bạn --internet--> https://myapp.domain.com
```

### Tính năng

- **HTTP tunnels** — subdomain HTTPS (random hoặc cố định vĩnh viễn)
- **TCP tunnels** — forward port bất kỳ (database, SSH, ...)
- **Auto-TLS** — wildcard Let's Encrypt cert qua DNS-01 (Cloudflare)
- **WebSocket** — proxy đầy đủ, hỗ trợ HMR (Next.js, Vite, ...)
- **API key** — người dùng tự đăng ký qua dashboard
- **Subdomain cố định** — claim 1 lần, dùng mãi mãi
- **User dashboard** — đăng ký, đăng nhập, quản lý subdomain
- **Admin dashboard** — quản lý users, tunnels, sửa config server
- **Tự động kết nối lại** — heartbeat + exponential backoff
- **Single binary** — không cần cài thêm gì, chạy mọi nền tảng

---

### Hướng dẫn sử dụng (từng bước)

#### Bước 1: Cài client

Chọn 1 trong 3 cách:

**Cách A: Go install** (nếu đã cài Go)

```bash
go install github.com/nhh0718/htn-tunnel/cmd/htn-tunnel@latest
```

**Cách B: npm** (nếu đã cài Node.js)

```bash
npm install -g htn-tunnel
```

**Cách C: Tải binary** từ [GitHub Releases](https://github.com/nhh0718/htn-tunnel/releases)

```bash
# Linux
curl -L https://github.com/nhh0718/htn-tunnel/releases/latest/download/htn-tunnel_linux_amd64.tar.gz | tar xz
sudo mv htn-tunnel /usr/local/bin/

# macOS
curl -L https://github.com/nhh0718/htn-tunnel/releases/latest/download/htn-tunnel_darwin_arm64.tar.gz | tar xz
sudo mv htn-tunnel /usr/local/bin/

# Windows — tải file .zip từ trang Releases, giải nén, thêm vào PATH
```

#### Bước 2: Đăng ký tài khoản

Mở dashboard trong trình duyệt:

```
https://dashboard.33.id.vn/_dashboard/
```

1. Nhấn **Register**
2. Nhập **tên** và **subdomain** muốn dùng (ví dụ: `myapp`)
3. Nhấn **Create Account**
4. **Copy API key** (bắt đầu bằng `htk_...`) — lưu lại cẩn thận!

> Chỉ cần đăng ký 1 lần. Subdomain của bạn được giữ vĩnh viễn.

#### Bước 3: Lưu API key

```bash
htn-tunnel auth htk_key_của_bạn --server 33.id.vn:4443
```

Key được lưu vào `~/.htn-tunnel/config.yaml`. Chỉ cần làm 1 lần.

#### Bước 4: Mở tunnel

```bash
# Expose web server local port 3000
htn-tunnel http 3000 --subdomain myapp
```

Kết quả:
```
htn-tunnel vdev

  Tunnel:    https://myapp.33.id.vn -> localhost:3000
  Status:    connected
```

Mở `https://myapp.33.id.vn` trên trình duyệt — hiển thị app local của bạn!

#### Bước 5: (Tùy chọn) TCP tunnel

```bash
# Expose PostgreSQL port 5432
htn-tunnel tcp 5432
```

Kết nối từ bất kỳ đâu: `psql -h 33.id.vn -p 34567 -U postgres`

---

### Quản lý tài khoản

#### Dashboard

Mở `https://dashboard.33.id.vn/_dashboard/` và đăng nhập bằng API key để:
- Xem subdomain (online/offline)
- Thêm hoặc xóa subdomain
- Xem thống kê tunnel (uptime, bandwidth)
- Copy lệnh quick-start

#### CLI

```bash
htn-tunnel status    # Xem thông tin tài khoản, subdomain
```

---

### Hướng dẫn sử dụng CLI

| Lệnh | Mô tả |
|------|-------|
| `htn-tunnel http <port>` | Tạo HTTP tunnel tới localhost:port |
| `htn-tunnel tcp <port>` | Tạo TCP tunnel tới localhost:port |
| `htn-tunnel auth <token>` | Lưu API key vào config |
| `htn-tunnel status` | Xem thông tin tài khoản |

**Flags:**

| Flag | Mô tả |
|------|-------|
| `--subdomain <tên>` | Yêu cầu subdomain cố định (chỉ HTTP) |
| `--server <host:port>` | Địa chỉ server |
| `--token <key>` | Override auth token |

---

### Cài đặt Server (cho admin VPS)

Xem [Deployment Guide](docs/deployment-guide.md) chi tiết.

**Admin dashboard:** `https://dashboard.33.id.vn/_admin/` — đăng nhập bằng admin key để quản lý users, tunnels, config.

### Tài liệu

- [Deployment Guide](docs/deployment-guide.md) — Hướng dẫn deploy VPS đầy đủ
- [Publish Guide](docs/publish-guide.md) — Hướng dẫn publish npm + GitHub Releases
- [Commands Reference](docs/commands.md) — Tất cả lệnh quản trị VPS

### License

[MIT](LICENSE)
