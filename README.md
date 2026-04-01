# htn-tunnel

Self-hosted tunneling tool — expose localhost to the internet via a public VPS. HTTP subdomain routing, raw TCP port forwarding, auto-TLS via Let's Encrypt, embedded web dashboard.

```
htn-tunnel http 3000   # → https://abc123.tunnel.example.com
htn-tunnel tcp 5432    # → tcp://tunnel.example.com:34567
```

## Features

- **HTTP tunnels** — HTTPS subdomain routing with custom or random subdomains
- **TCP tunnels** — raw port forwarding for any TCP service (databases, SSH, etc.)
- **Auto-TLS** — wildcard Let's Encrypt cert via DNS-01 challenge (Cloudflare)
- **WebSocket support** — proxies Upgrade requests (HMR, dev servers)
- **Token auth + rate limiting** — bcrypt-hashed tokens, per-token and global limits
- **Auto-reconnect** — heartbeat keepalive + exponential backoff reconnection
- **Embedded dashboard** — live tunnel stats at `http://server:8080/_dashboard/`
- **Single binary** — no runtime dependencies; cross-compiled for Linux/macOS/Windows

## Architecture

```
Internet → Server (VPS) → Client CLI (behind NAT) → Local Services
           ├─ TLS termination (certmagic / Let's Encrypt)
           ├─ SNI subdomain routing
           ├─ TCP port allocation + forwarding
           ├─ Token auth + rate limiting
           └─ yamux multiplexing over single TLS conn
```

## Quick Start

### 1. Server Setup (VPS)

**Prerequisites:**
- VPS with a public IP and ports 80, 443, 4443 open
- Domain with Cloudflare DNS (for wildcard cert)
- Cloudflare API token with `Zone:DNS:Edit` permission

```bash
# Clone and build
git clone https://github.com/htn-sys/htn-tunnel
cd htn-tunnel
make build-server

# Or run with Docker
cp server.example.yaml server.yaml
# Edit server.yaml: set domain, email, tokens, dns_api_token

docker compose up -d
```

**DNS setup (Cloudflare):**
```
A    tunnel.example.com    → <VPS IP>
A    *.tunnel.example.com  → <VPS IP>
```

### 2. Client Setup

```bash
# Download binary or build from source
make build-client

# Save auth token
./dist/htn-tunnel-client auth tok_your_token_here --server tunnel.example.com:4443

# Create HTTP tunnel
./dist/htn-tunnel-client http 3000

# Create TCP tunnel
./dist/htn-tunnel-client tcp 5432
```

## CLI Reference

```
htn-tunnel http <port> [flags]
  --subdomain string   request a specific subdomain
  --server string      override server address
  --token string       override auth token

htn-tunnel tcp <port> [flags]
  --server string      override server address
  --token string       override auth token

htn-tunnel auth <token> [flags]
  --server string      save server address to config
```

## Configuration Reference

### Server (server.yaml)

| Field | Env Var | Default | Description |
|-------|---------|---------|-------------|
| `listen_addr` | `HTN_LISTEN_ADDR` | `:4443` | Control plane TLS address |
| `domain` | `HTN_DOMAIN` | — | Base domain for HTTP tunnels |
| `email` | `HTN_EMAIL` | — | Let's Encrypt email |
| `tokens` | `HTN_TOKENS` | — | Comma-separated auth tokens |
| `max_tunnels_per_token` | `HTN_MAX_TUNNELS_PER_TOKEN` | `10` | Per-token tunnel limit |
| `rate_limit` | `HTN_RATE_LIMIT` | `100` | Per-token req/min |
| `global_rate_limit` | `HTN_GLOBAL_RATE_LIMIT` | `1000` | Server-wide req/min |
| `tcp_port_range` | `HTN_TCP_PORT_MIN/MAX` | `[10000,65535]` | TCP port range |
| `dns_provider` | `HTN_DNS_PROVIDER` | `cloudflare` | DNS-01 provider |
| `dns_api_token` | `HTN_DNS_API_TOKEN` | — | DNS provider API token |
| `dev_mode` | `HTN_DEV_MODE` | `false` | Self-signed cert (dev only) |
| `dashboard_enabled` | `HTN_DASHBOARD_ENABLED` | `true` | Enable web dashboard |
| `dashboard_addr` | `HTN_DASHBOARD_ADDR` | `:8080` | Dashboard address |
| `admin_token` | `HTN_ADMIN_TOKEN` | — | Dashboard admin token |

### Client (~/.htn-tunnel/config.yaml)

| Field | Description |
|-------|-------------|
| `server_addr` | Server address (host:port) |
| `token` | Auth token |

## Building

```bash
make build-all        # build server + client for current platform
make cross-compile    # build all 8 OS/arch targets to dist/
make test             # run tests
make lint             # go vet
make clean            # remove dist/
```

## Security Notes

- TCP tunnel ports are exposed to the internet without auth — use application-level auth (SSH keys, database passwords, etc.)
- Store auth tokens securely; they are saved in plaintext in `~/.htn-tunnel/config.yaml` (chmod 600)
- Never commit `server.yaml` or `.env` files containing real tokens to git
