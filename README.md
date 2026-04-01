<p align="center">
  <h1 align="center">htn-tunnel</h1>
  <p align="center">
    Self-hosted tunnel — expose localhost to the internet
    <br />
    <a href="#-quick-start"><strong>Quick Start</strong></a> &middot;
    <a href="#-hướng-dẫn-nhanh"><strong>Tiếng Việt</strong></a> &middot;
    <a href="https://dashboard.33.id.vn/_dashboard/">Dashboard</a> &middot;
    <a href="docs/deployment-guide.md">Deploy Guide</a>
  </p>
</p>

<p align="center">
  <a href="https://github.com/nhh0718/htn-tunnel/releases"><img src="https://img.shields.io/github/v/release/nhh0718/htn-tunnel?style=flat-square&color=blue" alt="Release"></a>
  <a href="https://www.npmjs.com/package/htn-tunnel"><img src="https://img.shields.io/npm/v/htn-tunnel?style=flat-square&color=red" alt="npm"></a>
  <a href="https://go.dev/"><img src="https://img.shields.io/github/go-mod/go-version/nhh0718/htn-tunnel?style=flat-square" alt="Go"></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/license-MIT-green?style=flat-square" alt="License"></a>
</p>

<p align="center">
  <img src="https://img.shields.io/badge/Windows-0078D6?style=flat-square&logo=windows&logoColor=white" alt="Windows">
  <img src="https://img.shields.io/badge/macOS-000000?style=flat-square&logo=apple&logoColor=white" alt="macOS">
  <img src="https://img.shields.io/badge/Linux-FCC624?style=flat-square&logo=linux&logoColor=black" alt="Linux">
</p>

---

## What is htn-tunnel?

A self-hosted alternative to ngrok / Cloudflare Tunnel. Run the server on your VPS, install the client on your machine, and expose any local port to the internet.

```
localhost:3000  ──tunnel──▶  Your VPS  ──internet──▶  https://myapp.your-domain.com
```

### Features

| Feature | Description |
|---------|-------------|
| **HTTP tunnels** | HTTPS subdomain routing (random or permanent) |
| **TCP tunnels** | Raw port forwarding (databases, SSH, etc.) |
| **Auto-TLS** | Wildcard Let's Encrypt via DNS-01 (Cloudflare) |
| **WebSocket** | Full proxy with Origin rewrite (HMR, dev servers) |
| **API keys** | Self-service registration via web dashboard |
| **Fixed subdomains** | Claim once, yours forever |
| **User dashboard** | Register, login, manage subdomains |
| **Admin dashboard** | Manage users, tunnels, server config |
| **Auto-reconnect** | Heartbeat + exponential backoff |
| **Single binary** | No dependencies, cross-platform |

---

## Install

<table>
<tr><td><b>go install</b></td><td><b>npm</b></td><td><b>Binary</b></td></tr>
<tr>
<td>

```bash
go install github.com/nhh0718/htn-tunnel/cmd/htn-tunnel@latest
```

</td>
<td>

```bash
npm install -g htn-tunnel
```

</td>
<td>

Download from [Releases](https://github.com/nhh0718/htn-tunnel/releases)

</td>
</tr>
</table>

<details>
<summary><b>Platform-specific download commands</b></summary>

```bash
# Linux (amd64)
curl -L https://github.com/nhh0718/htn-tunnel/releases/latest/download/htn-tunnel_linux_amd64.tar.gz | tar xz
sudo mv htn-tunnel /usr/local/bin/

# macOS (Apple Silicon)
curl -L https://github.com/nhh0718/htn-tunnel/releases/latest/download/htn-tunnel_darwin_arm64.tar.gz | tar xz
sudo mv htn-tunnel /usr/local/bin/

# Windows — download .zip from Releases, extract, add to PATH
```
</details>

---

## Quick Start

### 1. Register

Open the dashboard in your browser and create an account:

> **https://dashboard.33.id.vn/_dashboard/**

Click **Register** → enter your name + desired subdomain → **Create Account** → copy your API key (`htk_...`)

### 2. Save your key

```bash
htn-tunnel auth htk_your_key_here --server 33.id.vn:4443
```

### 3. Start tunneling

```bash
htn-tunnel http 3000 --subdomain myapp
```

```
  Tunnel:    https://myapp.33.id.vn → localhost:3000
  Status:    connected
```

That's it! Open `https://myapp.33.id.vn` in any browser.

> Your subdomain is permanently yours. Reconnect anytime with the same `--subdomain`.

### TCP tunnel

```bash
htn-tunnel tcp 5432
#  Tunnel:    tcp://33.id.vn:34567 → localhost:5432
```

---

## CLI Reference

```
htn-tunnel http <port> [--subdomain name]    Create HTTP tunnel
htn-tunnel tcp  <port>                       Create TCP tunnel
htn-tunnel auth <key>  [--server host:port]  Save API key
htn-tunnel status                            Show account info
```

| Flag | Description |
|------|-------------|
| `--subdomain <name>` | Fixed subdomain (HTTP only) |
| `--server <host:port>` | Server address |
| `--token <key>` | Override auth token |

---

## Dashboard

| | URL | Auth |
|---|---|---|
| **User** | `https://dashboard.33.id.vn/_dashboard/` | API key |
| **Admin** | `https://dashboard.33.id.vn/_admin/` | Admin key |

**User dashboard:** register, manage subdomains, view tunnel stats

**Admin dashboard:** manage all users, view all tunnels, edit server config

---

## Architecture

```
Internet → Port 443 (nginx stream, SNI routing)
├── dashboard.33.id.vn  → nginx:4430 → proxy → :1807 (dashboard)
├── *.33.id.vn           → TLS passthrough → :8443 (tunnel proxy)
└── * (other domains)    → nginx:4430 (certbot TLS)

htn-tunnel server:
  :4443   Control plane (yamux multiplexing)
  :8443   HTTP tunnel proxy (certmagic wildcard TLS)
  :1807   Dashboard (user + admin)
```

---

## Server Setup

See **[Deployment Guide](docs/deployment-guide.md)** for full instructions.

```bash
GOOS=linux GOARCH=amd64 go build -o htn-server ./cmd/server
scp htn-server root@your-vps:/usr/local/bin/
# See deployment guide for server.yaml, systemd, nginx, DNS
```

---

## Documentation

| Doc | Description |
|-----|-------------|
| [Deployment Guide](docs/deployment-guide.md) | VPS setup, nginx, DNS, systemd |
| [Publish Guide](docs/publish-guide.md) | npm + GitHub Releases CI/CD |
| [Commands Reference](docs/commands.md) | All VPS management commands |

---

## License

[MIT](LICENSE) — use it however you want.

---

<br>

<h1 align="center">htn-tunnel <sub><sup>Tiếng Việt</sup></sub></h1>

<p align="center">Công cụ tunnel tự host — expose localhost ra internet qua VPS của bạn</p>

---

## Htn-tunnel là gì?

Thay thế ngrok / Cloudflare Tunnel. Chạy server trên VPS, cài client trên máy bạn, expose bất kỳ port local nào ra internet với URL HTTPS.

```
Máy bạn (localhost:3000)  ──tunnel──▶  VPS  ──internet──▶  https://myapp.33.id.vn
```

### Tính năng

| Tính năng | Mô tả |
|-----------|-------|
| **HTTP tunnels** | Subdomain HTTPS (random hoặc cố định vĩnh viễn) |
| **TCP tunnels** | Forward port bất kỳ (database, SSH, ...) |
| **Auto-TLS** | Wildcard Let's Encrypt qua DNS-01 (Cloudflare) |
| **WebSocket** | Proxy đầy đủ, hỗ trợ HMR (Next.js, Vite) |
| **API key** | Người dùng tự đăng ký qua dashboard |
| **Subdomain cố định** | Claim 1 lần, dùng mãi mãi |
| **User dashboard** | Đăng ký, đăng nhập, quản lý subdomain |
| **Admin dashboard** | Quản lý users, tunnels, sửa config |
| **Tự động kết nối lại** | Heartbeat + exponential backoff |
| **Single binary** | Không cần cài thêm gì, chạy mọi nền tảng |

---

## Cài đặt

<table>
<tr><td><b>go install</b></td><td><b>npm</b></td><td><b>Binary</b></td></tr>
<tr>
<td>

```bash
go install github.com/nhh0718/htn-tunnel/cmd/htn-tunnel@latest
```

</td>
<td>

```bash
npm install -g htn-tunnel
```

</td>
<td>

Tải từ [Releases](https://github.com/nhh0718/htn-tunnel/releases)

</td>
</tr>
</table>

<details>
<summary><b>Lệnh tải theo nền tảng</b></summary>

```bash
# Linux
curl -L https://github.com/nhh0718/htn-tunnel/releases/latest/download/htn-tunnel_linux_amd64.tar.gz | tar xz
sudo mv htn-tunnel /usr/local/bin/

# macOS (Apple Silicon)
curl -L https://github.com/nhh0718/htn-tunnel/releases/latest/download/htn-tunnel_darwin_arm64.tar.gz | tar xz
sudo mv htn-tunnel /usr/local/bin/

# Windows — tải .zip từ Releases, giải nén, thêm vào PATH
```
</details>

---

## Hướng dẫn nhanh

### 1. Đăng ký tài khoản

Mở dashboard trong trình duyệt:

> **https://dashboard.33.id.vn/_dashboard/**

Nhấn **Register** → nhập tên + subdomain → **Create Account** → copy API key (`htk_...`)

### 2. Lưu API key

```bash
htn-tunnel auth htk_key_của_bạn --server 33.id.vn:4443
```

### 3. Mở tunnel

```bash
htn-tunnel http 3000 --subdomain myapp
```

```
  Tunnel:    https://myapp.33.id.vn → localhost:3000
  Status:    connected
```

Xong! Mở `https://myapp.33.id.vn` trên bất kỳ trình duyệt nào.

> Subdomain của bạn là vĩnh viễn. Kết nối lại bất kỳ lúc nào với cùng `--subdomain`.

### TCP tunnel

```bash
htn-tunnel tcp 5432
#  Tunnel:    tcp://33.id.vn:34567 → localhost:5432
```

---

## Hướng dẫn CLI

```
htn-tunnel http <port> [--subdomain tên]     Tạo HTTP tunnel
htn-tunnel tcp  <port>                       Tạo TCP tunnel
htn-tunnel auth <key>  [--server host:port]  Lưu API key
htn-tunnel status                            Xem thông tin tài khoản
```

| Flag | Mô tả |
|------|-------|
| `--subdomain <tên>` | Subdomain cố định (chỉ HTTP) |
| `--server <host:port>` | Địa chỉ server |
| `--token <key>` | Override auth token |

---

## Quản lý tài khoản

| | URL | Xác thực |
|---|---|---|
| **Người dùng** | `https://dashboard.33.id.vn/_dashboard/` | API key |
| **Admin** | `https://dashboard.33.id.vn/_admin/` | Admin key |

**User dashboard:** đăng ký, quản lý subdomain, xem thống kê tunnel

**Admin dashboard:** quản lý users, tunnels, sửa cấu hình server

---

## Tài liệu

| Tài liệu | Mô tả |
|-----------|-------|
| [Deployment Guide](docs/deployment-guide.md) | Hướng dẫn deploy VPS đầy đủ |
| [Publish Guide](docs/publish-guide.md) | Publish npm + GitHub Releases |
| [Commands Reference](docs/commands.md) | Tất cả lệnh quản trị VPS |

---

## License

[MIT](LICENSE)
