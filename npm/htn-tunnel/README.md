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
| **1-command setup** | `htn-server init` — interactive wizard, zero manual config |
| **Auto-reconnect** | Heartbeat + exponential backoff |
| **Self-upgrade** | `htn-server upgrade` — auto-update from GitHub Releases |
| **Backup/restore** | `htn-server backup` / `restore` for migration |
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

### Option A: Use our free cloud (33.id.vn)

No server setup needed. Install and tunnel in seconds.

**Step 1 — Install:**

```bash
npm install -g htn-tunnel
```

**Step 2 — Connect to our cloud:**

```bash
htn-tunnel setup
# Enter server: 33.id.vn:4443
```

**Step 3 — Tunnel!**

```bash
htn-tunnel http 3000           # anonymous, random subdomain, expires 2h
```

Want a permanent subdomain? Register once:

```bash
htn-tunnel login               # opens browser → register → key auto-saved
htn-tunnel http 3000:myapp     # https://myapp.33.id.vn → localhost:3000
```

> Your subdomain is permanent. Reconnect anytime. If you use a custom subdomain without logging in, htn-tunnel auto-prompts you to register.

**TCP tunnel:**

```bash
htn-tunnel tcp 5432            # tcp://33.id.vn:34567 → localhost:5432
```

**Dashboard:** [dashboard.33.id.vn](https://dashboard.33.id.vn/_dashboard/) — manage subdomains, view live traffic, analytics

---

### Option B: Self-host your own server

Run your own tunnel server on any VPS. Full control, your domain, your rules.

**Step 1 — Install server on VPS:**

```bash
curl -L https://github.com/nhh0718/htn-tunnel/releases/latest/download/htn-server_linux_amd64.tar.gz | tar xz
sudo mv htn-server /usr/local/bin/
```

**Step 2 — Run wizard (1 command):**

```bash
sudo htn-server init
# Asks: domain, email, Cloudflare token, admin password
# Auto: validates DNS → generates config → creates systemd → starts server → requests TLS cert
```

**Step 3 — Tell your team:**

```bash
npm install -g htn-tunnel
htn-tunnel setup               # enter: tunnel.yourdomain.com:4443
htn-tunnel login                # register on your server
htn-tunnel http 3000:myapp      # https://myapp.tunnel.yourdomain.com
```

**Server management:**

```
htn-server init         Interactive setup wizard
htn-server health       Check server health
htn-server status       Show config + live stats
htn-server upgrade      Self-update to latest version
htn-server backup       Export config + keys to tar.gz
htn-server restore <f>  Restore from backup file
```

**Prerequisites:** VPS with public IP, domain on Cloudflare (free tier), wildcard DNS `*.tunnel.yourdomain.com → VPS IP`

See **[Deployment Guide](docs/deployment-guide.md)** for advanced setup (nginx, Docker, manual config).

---

## CLI Reference

```
htn-tunnel setup                             Configure server connection (first time)
htn-tunnel http <port>[:<subdomain>]         Create HTTP tunnel (anonymous or authenticated)
htn-tunnel tcp  <port>                       Create TCP tunnel (requires auth)
htn-tunnel login                             Register/login via browser
htn-tunnel logout                            Clear saved auth key
htn-tunnel dashboard                         Open web dashboard in browser
htn-tunnel status                            Show account info (or anonymous state)
htn-tunnel auth <key>                        Save API key manually
```

| Flag | Description |
|------|-------------|
| `--server <host:port>` | Override server address |
| `--token <key>` | Override auth token |

---

## Dashboard

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
| **1 lệnh cài server** | `htn-server init` — wizard tương tác, không cần config thủ công |
| **Tự động kết nối lại** | Heartbeat + exponential backoff |
| **Tự nâng cấp** | `htn-server upgrade` — tự tải bản mới từ GitHub |
| **Sao lưu/phục hồi** | `htn-server backup` / `restore` để migrate |
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

### Cách A: Dùng cloud miễn phí (33.id.vn)

Không cần setup server. Cài và tunnel ngay.

**Bước 1 — Cài đặt:**

```bash
npm install -g htn-tunnel
```

**Bước 2 — Kết nối cloud:**

```bash
htn-tunnel setup
# Nhập server: 33.id.vn:4443
```

**Bước 3 — Tunnel!**

```bash
htn-tunnel http 3000           # ẩn danh, subdomain ngẫu nhiên, hết hạn 2h
```

Muốn subdomain riêng? Đăng ký 1 lần:

```bash
htn-tunnel login               # mở browser → đăng ký → key tự lưu
htn-tunnel http 3000:myapp     # https://myapp.33.id.vn → localhost:3000
```

> Subdomain của bạn là vĩnh viễn. Kết nối lại bất kỳ lúc nào.

**TCP tunnel:**

```bash
htn-tunnel tcp 5432            # tcp://33.id.vn:34567 → localhost:5432
```

**Dashboard:** [dashboard.33.id.vn](https://dashboard.33.id.vn/_dashboard/) — quản lý subdomain, xem traffic real-time

---

### Cách B: Tự host server riêng

Chạy tunnel server trên VPS của bạn. Toàn quyền kiểm soát, domain riêng.

**Bước 1 — Cài server trên VPS:**

```bash
curl -L https://github.com/nhh0718/htn-tunnel/releases/latest/download/htn-server_linux_amd64.tar.gz | tar xz
sudo mv htn-server /usr/local/bin/
```

**Bước 2 — Chạy wizard (1 lệnh):**

```bash
sudo htn-server init
# Hỏi: domain, email, Cloudflare token, mật khẩu admin
# Tự động: kiểm tra DNS → tạo config → tạo systemd → khởi động → xin cert TLS
```

**Bước 3 — Gửi cho team:**

```bash
npm install -g htn-tunnel
htn-tunnel setup               # nhập: tunnel.domain.com:4443
htn-tunnel login                # đăng ký trên server của bạn
htn-tunnel http 3000:myapp      # https://myapp.tunnel.domain.com
```

**Quản lý server:**

```
htn-server init         Wizard cài đặt tương tác
htn-server health       Kiểm tra sức khỏe server
htn-server status       Xem config + thống kê
htn-server upgrade      Tự cập nhật phiên bản mới
htn-server backup       Sao lưu config + keys
htn-server restore <f>  Phục hồi từ file backup
```

**Yêu cầu:** VPS có IP public, domain trên Cloudflare (free tier OK), DNS wildcard `*.tunnel.domain.com → IP VPS`

Xem **[Deployment Guide](docs/deployment-guide.md)** cho hướng dẫn nâng cao (nginx, Docker, config thủ công).

---

## Hướng dẫn CLI

```
htn-tunnel setup                             Cấu hình kết nối server (lần đầu)
htn-tunnel http <port>[:<subdomain>]         Tạo HTTP tunnel (ẩn danh hoặc đã đăng ký)
htn-tunnel tcp  <port>                       Tạo TCP tunnel (cần đăng ký)
htn-tunnel login                             Đăng ký/đăng nhập qua browser
htn-tunnel logout                            Xóa key đã lưu
htn-tunnel dashboard                         Mở dashboard trên browser
htn-tunnel status                            Xem thông tin tài khoản
htn-tunnel auth <key>                        Lưu API key thủ công
```

| Flag | Mô tả |
|------|-------|
| `--server <host:port>` | Override địa chỉ server |
| `--token <key>` | Override auth token |

---

## Dashboard

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
