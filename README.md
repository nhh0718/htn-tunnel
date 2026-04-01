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

## Tieng Viet

### htn-tunnel la gi?

Cong cu tunnel tu host — thay the ngrok/Cloudflare Tunnel. Chay server tren VPS, cai client tren may ban, expose bat ky port local nao ra internet voi URL HTTPS.

```
May ban (localhost:3000) --tunnel--> VPS cua ban --internet--> https://myapp.domain.com
```

### Tinh nang

- **HTTP tunnels** — subdomain HTTPS (random hoac co dinh vinh vien)
- **TCP tunnels** — forward port bat ky (database, SSH, ...)
- **Auto-TLS** — wildcard Let's Encrypt cert qua DNS-01 (Cloudflare)
- **WebSocket** — proxy day du, ho tro HMR (Next.js, Vite, ...)
- **API key** — user tu dang ky qua dashboard
- **Subdomain co dinh** — claim 1 lan, dung mai mai
- **User dashboard** — dang ky, dang nhap, quan ly subdomain
- **Admin dashboard** — quan ly users, tunnels, sua config server
- **Tu dong ket noi lai** — heartbeat + exponential backoff
- **Single binary** — khong can cai them gi, chay moi nen tang

---

### Huong dan su dung (tung buoc)

#### Buoc 1: Cai client

Chon 1 trong 3 cach:

**Cach A: Go install** (neu da cai Go)

```bash
go install github.com/nhh0718/htn-tunnel/cmd/htn-tunnel@latest
```

**Cach B: npm** (neu da cai Node.js)

```bash
npm install -g htn-tunnel
```

**Cach C: Tai binary** tu [GitHub Releases](https://github.com/nhh0718/htn-tunnel/releases)

```bash
# Linux
curl -L https://github.com/nhh0718/htn-tunnel/releases/latest/download/htn-tunnel_linux_amd64.tar.gz | tar xz
sudo mv htn-tunnel /usr/local/bin/

# macOS
curl -L https://github.com/nhh0718/htn-tunnel/releases/latest/download/htn-tunnel_darwin_arm64.tar.gz | tar xz
sudo mv htn-tunnel /usr/local/bin/

# Windows — tai file .zip tu trang Releases, giai nen, them vao PATH
```

#### Buoc 2: Dang ky tai khoan

Mo dashboard tren browser:

```
https://<dia-chi-server>:1807/_dashboard/
```

1. Nhan **Register**
2. Nhap **ten** va **subdomain** muon dung (vi du: `myapp`)
3. Nhan **Create Account**
4. **Copy API key** (bat dau bang `htk_...`) — luu lai can than!

> Chi can dang ky 1 lan. Subdomain cua ban duoc giu vinh vien.

#### Buoc 3: Luu API key

```bash
htn-tunnel auth htk_key_cua_ban --server 33.id.vn:4443
```

Key duoc luu vao `~/.htn-tunnel/config.yaml`. Chi can lam 1 lan.

#### Buoc 4: Mo tunnel

```bash
# Expose web server local port 3000
htn-tunnel http 3000 --subdomain myapp
```

Ket qua:
```
htn-tunnel vdev

  Tunnel:    https://myapp.33.id.vn -> localhost:3000
  Status:    connected
```

Mo `https://myapp.33.id.vn` tren browser — hien thi app local cua ban!

#### Buoc 5: (Tuy chon) TCP tunnel

```bash
# Expose PostgreSQL port 5432
htn-tunnel tcp 5432
```

Ket noi tu bat ky dau: `psql -h 33.id.vn -p 34567 -U postgres`

---

### Quan ly tai khoan

#### Dashboard

Mo `https://<server>:1807/_dashboard/` va dang nhap bang API key de:
- Xem subdomain (online/offline)
- Them hoac xoa subdomain
- Xem thong ke tunnel (uptime, bandwidth)
- Copy lenh quick-start

#### CLI

```bash
htn-tunnel status    # Xem thong tin tai khoan, subdomain
```

---

### Huong dan su dung CLI

| Lenh | Mo ta |
|------|-------|
| `htn-tunnel http <port>` | Tao HTTP tunnel toi localhost:port |
| `htn-tunnel tcp <port>` | Tao TCP tunnel toi localhost:port |
| `htn-tunnel auth <token>` | Luu API key vao config |
| `htn-tunnel status` | Xem thong tin tai khoan |

**Flags:**

| Flag | Mo ta |
|------|-------|
| `--subdomain <ten>` | Yeu cau subdomain co dinh (chi HTTP) |
| `--server <host:port>` | Dia chi server |
| `--token <key>` | Override auth token |

---

### Cai dat Server (cho admin VPS)

Xem [Deployment Guide](docs/deployment-guide.md) chi tiet.

**Admin dashboard:** `https://<server>:1807/_admin/` — dang nhap bang admin key de quan ly users, tunnels, config.

### Tai lieu

- [Deployment Guide](docs/deployment-guide.md) — Huong dan deploy VPS day du
- [Publish Guide](docs/publish-guide.md) — Huong dan publish npm + GitHub Releases
- [Commands Reference](docs/commands.md) — Tat ca lenh quan tri VPS

### License

[MIT](LICENSE)
