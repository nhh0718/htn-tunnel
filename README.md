<p align="center">
  <h1 align="center">htn-tunnel</h1>
  <p align="center">
    Self-hosted tunnel — expose localhost to the internet
    <br />
    <a href="#-quick-start"><strong>Quick Start</strong></a> &middot;
    <a href="#tiếng-việt"><strong>Tiếng Việt</strong></a> &middot;
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
| **Zero-auth start** | `htn-tunnel http 3000` works instantly, no registration |
| **HTTP tunnels** | HTTPS subdomain routing (random or permanent) |
| **TCP tunnels** | Raw port forwarding (databases, SSH, etc.) |
| **Browser login** | `htn-tunnel login` opens browser, key auto-saved |
| **Auto-TLS** | Wildcard Let's Encrypt via DNS-01 (Cloudflare) |
| **WebSocket** | Full proxy with Origin rewrite (HMR, dev servers) |
| **Fixed subdomains** | Claim once, yours forever |
| **Live analytics** | Real-time request log, traffic charts, status breakdown |
| **User dashboard** | Register, manage subdomains, view traffic |
| **Admin dashboard** | Manage users, tunnels, server config, analytics |
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

### Instant (no registration)

```bash
htn-tunnel http 3000
```

```
  Tunnel:    https://abc12xyz.33.id.vn → localhost:3000
  Status:    connected
```

That's it! You get a random subdomain, limited to 1 tunnel, auto-expires after 2h.

### With fixed subdomain

Register via browser to claim a permanent subdomain:

```bash
htn-tunnel login --server 33.id.vn:4443
# Opens browser → register → key saved automatically

htn-tunnel http 3000:myapp
```

```
  Tunnel:    https://myapp.33.id.vn → localhost:3000
  Status:    connected
```

> Your subdomain is permanently yours. Reconnect anytime with the same name.

If you try a custom subdomain without logging in, htn-tunnel auto-prompts you to register.

### TCP tunnel

```bash
htn-tunnel tcp 5432
#  Tunnel:    tcp://33.id.vn:34567 → localhost:5432
```

---

## CLI Reference

```
htn-tunnel http <port>[:<subdomain>]         Create HTTP tunnel (anonymous or authenticated)
htn-tunnel tcp  <port>                       Create TCP tunnel (requires auth)
htn-tunnel login   [--server host:port]      Register/login via browser
htn-tunnel logout                            Clear saved auth key
htn-tunnel dashboard                         Open web dashboard in browser
htn-tunnel status                            Show account info (or anonymous state)
htn-tunnel auth <key>  [--server host:port]  Save API key manually
```

| Flag | Description |
|------|-------------|
| `--server <host:port>` | Server address |
| `--token <key>` | Override auth token |

---

## Dashboard

| | URL | Auth |
|---|---|---|
| **User** | `https://dashboard.33.id.vn/_dashboard/` | API key |
| **Admin** | `https://dashboard.33.id.vn/_admin/` | Admin key |

**User dashboard:** register, manage subdomains, live request log, traffic chart, status breakdown, top paths

**Admin dashboard:** manage all users/tunnels, edit config, server-wide analytics with live log stream

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

<a id="tiếng-việt"></a>
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
| **Dùng ngay** | `htn-tunnel http 3000` chạy ngay, không cần đăng ký |
| **HTTP tunnels** | Subdomain HTTPS (random hoặc cố định vĩnh viễn) |
| **TCP tunnels** | Forward port bất kỳ (database, SSH, ...) |
| **Đăng nhập qua browser** | `htn-tunnel login` mở browser, key tự lưu |
| **Auto-TLS** | Wildcard Let's Encrypt qua DNS-01 (Cloudflare) |
| **WebSocket** | Proxy đầy đủ, hỗ trợ HMR (Next.js, Vite) |
| **Subdomain cố định** | Claim 1 lần, dùng mãi mãi |
| **Analytics trực tiếp** | Request log real-time, biểu đồ traffic, phân tích status |
| **User dashboard** | Đăng ký, quản lý subdomain, xem traffic |
| **Admin dashboard** | Quản lý users, tunnels, config, analytics |
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

### Dùng ngay (không cần đăng ký)

```bash
htn-tunnel http 3000
```

```
  Tunnel:    https://abc12xyz.33.id.vn → localhost:3000
  Status:    connected
```

Xong! Bạn nhận subdomain ngẫu nhiên, giới hạn 1 tunnel, tự hết hạn sau 2h.

### Với subdomain cố định

Đăng ký qua browser để claim subdomain riêng:

```bash
htn-tunnel login --server 33.id.vn:4443
# Mở browser → đăng ký → key tự động lưu

htn-tunnel http 3000:myapp
```

```
  Tunnel:    https://myapp.33.id.vn → localhost:3000
  Status:    connected
```

> Subdomain của bạn là vĩnh viễn. Kết nối lại bất kỳ lúc nào.

Nếu dùng subdomain cố định mà chưa đăng nhập, htn-tunnel sẽ tự mở browser cho bạn đăng ký.

### TCP tunnel

```bash
htn-tunnel tcp 5432
#  Tunnel:    tcp://33.id.vn:34567 → localhost:5432
```

---

## Hướng dẫn CLI

```
htn-tunnel http <port>[:<subdomain>]         Tạo HTTP tunnel (ẩn danh hoặc đã đăng ký)
htn-tunnel tcp  <port>                       Tạo TCP tunnel (cần đăng ký)
htn-tunnel login   [--server host:port]      Đăng ký/đăng nhập qua browser
htn-tunnel logout                            Xóa key đã lưu
htn-tunnel dashboard                         Mở dashboard trên browser
htn-tunnel status                            Xem thông tin tài khoản
htn-tunnel auth <key>  [--server host:port]  Lưu API key thủ công
```

| Flag | Mô tả |
|------|-------|
| `--server <host:port>` | Địa chỉ server |
| `--token <key>` | Override auth token |

---

## Quản lý tài khoản

| | URL | Xác thực |
|---|---|---|
| **Người dùng** | `https://dashboard.33.id.vn/_dashboard/` | API key |
| **Admin** | `https://dashboard.33.id.vn/_admin/` | Admin key |

**User dashboard:** đăng ký, quản lý subdomain, request log real-time, biểu đồ traffic, top paths

**Admin dashboard:** quản lý users/tunnels, sửa config, analytics toàn server với live log stream

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
