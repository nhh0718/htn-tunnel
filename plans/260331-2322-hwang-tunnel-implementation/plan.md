---
title: "htn-tunnel — Self-Hosted Tunneling Tool"
description: "Implementation plan for a Go-based self-hosted tunneling tool with HTTP subdomain routing, TCP port forwarding, and embedded dashboard"
status: pending
priority: P1
effort: 38h
branch: main
tags: [go, networking, tunneling, self-hosted, tls, cli]
created: 2026-03-31
---

# htn-tunnel — Implementation Plan

Self-hosted tunneling tool (ngrok alternative). Go. HTTP subdomain routing + raw TCP port forwarding + embedded web dashboard. Public multi-tenant service with auth and rate limiting.

**Module path:** `github.com/htn-sys/htn-tunnel`
**CLI binary:** `htn-tunnel` (commands: `htn-tunnel http 3000`, `htn-tunnel tcp 5432`, `htn-tunnel auth <token>`)
**DNS:** Cloudflare (free tier) for DNS-01 wildcard cert challenge via `libdns/cloudflare`

## Architecture

```
Internet Users → Server Relay (VPS) → Client CLI (behind NAT) → Local Services
                 ├─ TLS termination (certmagic / Let's Encrypt)
                 ├─ SNI-based subdomain routing
                 ├─ TCP port allocation + forwarding
                 ├─ Token auth + rate limiting
                 └─ yamux multiplexing over single TCP conn
```

## Research Reports
- [Tunneling Protocols](../reports/researcher-260331-2319-tunneling-protocols.md)
- [Deployment & UX Patterns](../reports/researcher-260331-2319-deployment-ux-patterns.md)

## Phases

| # | Phase | Priority | Effort | Status |
|---|-------|----------|--------|--------|
| 1 | [Project Setup & Wire Protocol](phase-01-project-setup-wire-protocol.md) | P1 | 3h | completed |
| 2 | [Server Core — Listener & Auth](phase-02-server-core-listener-auth.md) | P1 | 5h | completed |
| 3 | [HTTP Tunnel — Subdomain Routing](phase-03-http-tunnel-subdomain-routing.md) | P1 | 6h | completed |
| 4 | [TCP Tunnel — Port Forwarding](phase-04-tcp-tunnel-port-forwarding.md) | P2 | 3h | completed |
| 5 | [Client CLI](phase-05-client-cli.md) | P1 | 3h | completed |
| 6 | [Connection Resilience](phase-06-connection-resilience.md) | P2 | 4h | completed |
| 7 | [Deployment & Packaging](phase-07-deployment-packaging.md) | P2 | 3h | completed |
| 8 | [Testing & Hardening](phase-08-testing-hardening.md) | P2 | 5h | completed |
| 9 | [Embedded Dashboard](phase-09-embedded-dashboard.md) | P2 | 6h | completed |

## Key Dependencies
- Go 1.22+
- `github.com/hashicorp/yamux` — multiplexing
- `github.com/spf13/cobra` — CLI framework
- `github.com/caddyserver/certmagic` — auto Let's Encrypt
- `github.com/libdns/cloudflare` — DNS-01 challenge provider (Cloudflare free tier)
- `golang.org/x/time/rate` — rate limiting
- `gopkg.in/yaml.v3` — config parsing

## Critical Decisions
1. **yamux over smux** — hashicorp's yamux is battle-tested (used in Consul, Nomad)
2. **certmagic over autocert** — better wildcard support, DNS challenge built-in
3. **No database** — in-memory token store + config file; sufficient for <1k concurrent tunnels
4. **No protobuf/gRPC** — simple length-prefixed binary protocol; fewer deps, easier debugging
5. **No WebSocket tunnel transport** — pure TCP; add WS wrapper later only if firewall bypass needed
6. **TLS on control port from day 1** — tokens never sent plaintext, even in MVP
7. **`network_mode: host`** for Docker — avoids 55K iptables rules OOM on VPS

## Red Team Review

### Session — 2026-03-31
**Findings:** 15 (12 accepted, 3 rejected)
**Severity breakdown:** 5 Critical, 6 High, 4 Medium

| # | Finding | Severity | Disposition | Applied To |
|---|---------|----------|-------------|------------|
| 1 | Control port no TLS — plaintext tokens | Critical | Accept | Phase 2 |
| 2 | certmagic DNS-01 provider missing | Critical | Accept | Phase 3, plan.md |
| 3 | Docker 55K port mapping OOMs VPS | Critical | Accept | Phase 7 |
| 4 | yamux stream direction untested | Critical | Accept | Phase 1 |
| 5 | TCP tunnel ports unauthenticated | Critical | Accept | Phase 4 |
| 6 | No pre-auth connection limiting | High | Accept | Phase 2 |
| 7 | No token hashing or revocation | High | Accept | Phase 2 |
| 8 | In-memory state loss on restart | High | Reject | N/A (MVP acceptable) |
| 9 | 18h estimate unrealistic | High | Accept | plan.md (→ 32h) |
| 10 | No WebSocket proxy for HMR | High | Accept | Phase 3 |
| 11 | TCP tunnels should be deferred | High | Reject | N/A (user requirement) |
| 12 | No audit logging / observability | Medium | Accept | Phase 2 |
| 13 | Subdomain reservation race | Medium | Accept | Phase 6 |
| 14 | No wire protocol version byte | Medium | Accept | Phase 1 |
| 15 | No input validation on TunnelRequest | Medium | Accept | Phase 1 |

## Project Structure
```
htn-tunnel/
├── cmd/server/main.go          # Server entrypoint
├── cmd/client/main.go          # Client entrypoint
├── internal/server/
│   ├── server.go               # Main server, listener setup
│   ├── tunnel-manager.go       # Tunnel session registry
│   ├── http-proxy.go           # HTTPS reverse proxy + SNI routing
│   ├── tcp-proxy.go            # TCP port allocation + forwarding
│   ├── auth.go                 # Token validation, rate limiting
│   └── dashboard.go            # Embedded web dashboard handler
├── internal/dashboard/
│   ├── handler.go              # HTTP handlers for dashboard API
│   ├── static/                 # Embedded HTML/JS/CSS (Go embed)
│   │   ├── index.html
│   │   ├── app.js
│   │   └── style.css
│   └── embed.go                # //go:embed directives
├── internal/client/
│   ├── client.go               # Main client, connection mgmt
│   ├── tunnel.go               # Tunnel creation + local forwarding
│   └── reconnect.go            # Heartbeat + backoff
├── internal/protocol/
│   ├── message.go              # Message type definitions
│   └── codec.go                # Encode/decode framing
├── internal/config/
│   └── config.go               # Config structs, env vars, flags
├── go.mod
├── Makefile
├── Dockerfile
└── README.md
```
