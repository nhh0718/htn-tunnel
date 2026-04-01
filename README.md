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

A self-hosted alternative to ngrok/Cloudflare Tunnel. HTTP subdomain routing, raw TCP port forwarding, auto-TLS via Let's Encrypt, API key management, user/admin dashboards.

```bash
htn-tunnel http 3000           # https://yourname.33.id.vn
htn-tunnel http 3000 --subdomain myapp   # https://myapp.33.id.vn (permanent)
htn-tunnel tcp 5432            # tcp://server:34567
```

### Features

- **HTTP tunnels** — HTTPS subdomain routing (random or fixed per user)
- **TCP tunnels** — raw port forwarding for any TCP service
- **Auto-TLS** — wildcard Let's Encrypt cert via DNS-01 (Cloudflare)
- **WebSocket support** — full proxy with Origin rewrite (HMR, dev servers)
- **API key management** — self-service registration, fixed subdomains per key
- **User dashboard** — register, login, manage subdomains at `/_dashboard/`
- **Admin dashboard** — manage users, tunnels, server config at `/_admin/`
- **Auto-reconnect** — heartbeat keepalive + exponential backoff
- **Single binary** — no runtime dependencies

### Install Client

#### npm (recommended)

```bash
npm install -g htn-tunnel
```

#### GitHub Releases

Download from [Releases](https://github.com/nhh0718/htn-tunnel/releases):

```bash
# Linux
curl -L https://github.com/nhh0718/htn-tunnel/releases/latest/download/htn-tunnel_linux_amd64.tar.gz | tar xz
sudo mv htn-tunnel /usr/local/bin/

# macOS
curl -L https://github.com/nhh0718/htn-tunnel/releases/latest/download/htn-tunnel_darwin_arm64.tar.gz | tar xz
sudo mv htn-tunnel /usr/local/bin/

# Windows — download .zip from Releases page
```

#### Go install

```bash
go install github.com/nhh0718/htn-tunnel/cmd/htn-tunnel@latest
```

### Quick Start

```bash
# 1. Register (get API key + claim subdomain)
#    Open https://your-server:1807/_dashboard/ in browser
#    Or use the CLI:
htn-tunnel auth <your-api-key> --server your-server:4443

# 2. Create tunnel
htn-tunnel http 3000 --subdomain myapp
# => https://myapp.your-domain.com -> localhost:3000
```

### CLI Reference

```
htn-tunnel http <port> [flags]    Create HTTP tunnel
  --subdomain string              Request a fixed subdomain
  --server string                 Server address (host:port)
  --token string                  Override auth token

htn-tunnel tcp <port> [flags]     Create TCP tunnel

htn-tunnel auth <token> [flags]   Save auth token to config
  --server string                 Save server address

htn-tunnel status                 Show account info and subdomains
```

### Server Setup

See [Deployment Guide](docs/deployment-guide.md) for full VPS setup instructions.

```bash
# Build server
GOOS=linux GOARCH=amd64 go build -o htn-server ./cmd/server

# Upload to VPS
scp htn-server root@your-vps:/usr/local/bin/

# Configure and start
# See docs/deployment-guide.md for server.yaml, systemd, nginx, DNS setup
```

### Architecture

```
Internet -> Port 443 (nginx stream, SNI routing)
|-- *.domain.com  -> htn-tunnel:8443 (TLS passthrough)
|-- * (default)   -> nginx:4430 (other sites)

htn-tunnel server:
  :4443  Control plane (yamux, client auth)
  :8443  HTTP tunnel proxy (certmagic wildcard TLS)
  :8444  HTTP redirect
  :1807  Dashboard (user + admin)
```

### License

[MIT](LICENSE)

---

## Tieng Viet

### htn-tunnel la gi?

Cong cu tunnel tu host — thay the ngrok/Cloudflare Tunnel. Expose localhost ra internet qua VPS cua ban.

```bash
htn-tunnel http 3000           # https://tenban.33.id.vn
htn-tunnel http 3000 --subdomain myapp   # https://myapp.33.id.vn (co dinh)
htn-tunnel tcp 5432            # tcp://server:34567
```

### Tinh nang

- **HTTP tunnels** — subdomain HTTPS (random hoac co dinh theo user)
- **TCP tunnels** — forward port bat ky (database, SSH, ...)
- **Auto-TLS** — wildcard Let's Encrypt cert qua DNS-01 (Cloudflare)
- **WebSocket** — proxy day du (HMR, dev servers)
- **API key** — user tu dang ky, co dinh subdomain rieng
- **User dashboard** — dang ky, dang nhap, quan ly subdomain tai `/_dashboard/`
- **Admin dashboard** — quan ly users, tunnels, config tai `/_admin/`
- **Auto-reconnect** — heartbeat + exponential backoff
- **Single binary** — khong can cai them gi

### Cai dat Client

#### npm (khuyen nghi)

```bash
npm install -g htn-tunnel
```

#### GitHub Releases

Tai tu [Releases](https://github.com/nhh0718/htn-tunnel/releases):

```bash
# Linux
curl -L https://github.com/nhh0718/htn-tunnel/releases/latest/download/htn-tunnel_linux_amd64.tar.gz | tar xz
sudo mv htn-tunnel /usr/local/bin/

# macOS
curl -L https://github.com/nhh0718/htn-tunnel/releases/latest/download/htn-tunnel_darwin_arm64.tar.gz | tar xz
sudo mv htn-tunnel /usr/local/bin/

# Windows — tai file .zip tu trang Releases
```

#### Go install

```bash
go install github.com/nhh0718/htn-tunnel/cmd/htn-tunnel@latest
```

### Bat dau nhanh

```bash
# 1. Dang ky (lay API key + claim subdomain)
#    Mo https://your-server:1807/_dashboard/ tren browser
#    Hoac dung CLI:
htn-tunnel auth <api-key-cua-ban> --server 33.id.vn:4443

# 2. Tao tunnel
htn-tunnel http 3000 --subdomain myapp
# => https://myapp.33.id.vn -> localhost:3000
```

### Huong dan su dung CLI

```
htn-tunnel http <port> [flags]    Tao HTTP tunnel
  --subdomain string              Yeu cau subdomain co dinh
  --server string                 Dia chi server (host:port)
  --token string                  Override auth token

htn-tunnel tcp <port> [flags]     Tao TCP tunnel

htn-tunnel auth <token> [flags]   Luu auth token vao config
  --server string                 Luu dia chi server

htn-tunnel status                 Xem thong tin tai khoan va subdomain
```

### Cai dat Server

Xem [Deployment Guide](docs/deployment-guide.md) de biet chi tiet cai dat VPS.

### Tai lieu

- [Deployment Guide](docs/deployment-guide.md) — Huong dan deploy len VPS
- [Publish Guide](docs/publish-guide.md) — Huong dan publish npm + GitHub Releases
- [Commands Reference](docs/commands.md) — Tat ca lenh tren VPS

### License

[MIT](LICENSE)
