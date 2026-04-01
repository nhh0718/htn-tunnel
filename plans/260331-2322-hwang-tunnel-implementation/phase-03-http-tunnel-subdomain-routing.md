# Phase 3: HTTP Tunnel — Subdomain Routing

## Context Links
- [Plan Overview](plan.md)
- [Phase 2: Server Core](phase-02-server-core-listener-auth.md)
- [Deployment & UX Research](../reports/researcher-260331-2319-deployment-ux-patterns.md)

## Overview
- **Priority:** P1
- **Status:** completed
- **Effort:** 3h
- **Description:** TLS listener with certmagic (auto Let's Encrypt wildcard certs), SNI-based subdomain routing, HTTP reverse proxy that forwards requests through yamux streams to the client.

## Key Insights
- certmagic handles wildcard cert issuance via DNS-01 challenge (required for `*.example.com`)
- SNI inspection happens during TLS handshake BEFORE decryption — extract subdomain from ClientHello
- After TLS termination, use standard `httputil.ReverseProxy` to forward to yamux stream
- HTTP/80 listener redirects to HTTPS or uses Host header for routing (fallback)
- One wildcard cert covers all subdomains; no per-tunnel cert issuance needed

## Requirements

### Functional
- TLS listener on port 443 with auto Let's Encrypt via certmagic
- Wildcard certificate for `*.{domain}` + base `{domain}`
- SNI extraction: `abc.example.com` → subdomain `abc` → lookup tunnel session
- HTTP reverse proxy: decrypt HTTPS → open yamux stream → proxy request → relay response
- HTTP/80 listener: redirect to HTTPS (301) OR route via Host header
- Subdomain validation: alphanumeric + hyphens, 3-63 chars, no reserved names
- Custom subdomain: client can request specific subdomain
- Random subdomain: server assigns 8-char random string if not specified
- Add `X-Forwarded-For`, `X-Forwarded-Proto`, `X-Forwarded-Host` headers

### Non-Functional
- TLS handshake < 100ms (warm cache)
- Cert renewal automatic, zero-downtime
- Handle 500+ concurrent HTTP tunnel connections

## Architecture

### Request Flow
```
Browser → abc.example.com:443
  → TLS ClientHello (SNI: abc.example.com)
  → certmagic provides wildcard cert
  → TLS handshake completes
  → Server extracts subdomain "abc" from SNI
  → TunnelManager.LookupHTTP("abc") → TunnelSession
  → Open new yamux stream on session
  → Proxy HTTP request through stream
  → Client receives on yamux stream → dials localhost:3000
  → Response flows back: localhost → client → yamux → server → browser
```

### Component Layout
```
Port 443 (TLS)           Port 80 (HTTP)
     │                        │
     ▼                        ▼
 TLS Listener            HTTP Listener
 (certmagic)             (redirect/route)
     │                        │
     ▼                        ▼
 SNI Router ◄─────────── Host Router
     │
     ▼
 TunnelManager.LookupHTTP(subdomain)
     │
     ▼
 yamux.Session.Open() → new stream
     │
     ▼
 httputil.ReverseProxy (request → stream, stream → response)
```

### certmagic Setup
- DNS-01 challenge for wildcard certs (requires DNS provider API access)
- **DNS provider dependency required:** add `github.com/libdns/cloudflare` (or `libdns/route53`, `libdns/digitalocean`) to `go.mod`
- Config fields: `dns_provider` (string), `dns_api_token` (string) — used to configure certmagic DNS solver
- Storage: local filesystem (`/var/lib/hwang-tunnel/certs/`) or certmagic default
- Fallback: self-signed cert for dev/testing mode
> [!RED-TEAM] DNS-01 challenge REQUIRES a libdns provider package. Without it, wildcard certs cannot be issued. Plan originally omitted this critical dependency.

## Related Code Files

### Files to Create
- `internal/server/http-proxy.go` — HTTPProxy struct, TLS listener, SNI routing, reverse proxy

### Files to Modify
- `internal/server/server.go` — integrate HTTPProxy into server startup
- `internal/config/config.go` — add TLS/domain config fields
- `internal/server/tunnel-manager.go` — add subdomain validation

## Implementation Steps

1. **Add certmagic config to `ServerConfig`**
   - Fields: `Domain string`, `Email string` (for Let's Encrypt), `CertStorage string`, `DevMode bool`
   - DevMode=true → use self-signed cert (skip Let's Encrypt for local testing)

2. **Create `internal/server/http-proxy.go`**

3. **Implement `HTTPProxy` struct**
   - Fields: `config`, `tunnelManager *TunnelManager`, `tlsListener net.Listener`, `httpListener net.Listener`
   - `NewHTTPProxy(config, tunnelManager) *HTTPProxy`

4. **Implement TLS listener with certmagic**
   ```go
   certmagic.DefaultACME.Email = config.Email
   certmagic.DefaultACME.Agreed = true
   tlsConfig := certmagic.NewDefault().TLSConfig()
   tlsConfig.GetCertificate = certmagic handler
   listener = tls.Listen("tcp", ":443", tlsConfig)
   ```
   - For DevMode: generate self-signed cert with `crypto/tls` + `crypto/x509`

5. **Implement SNI extraction**
   - Use `tls.Config.GetConfigForClient` callback
   - Extract `clientHello.ServerName` → parse subdomain
   - Subdomain = first label of FQDN: `abc.example.com` → `abc`
   - Validate subdomain format (regex: `^[a-z0-9][a-z0-9-]{1,61}[a-z0-9]$`)

6. **Implement HTTP reverse proxy**
   - For each accepted TLS connection:
     - Parse HTTP request
     - Extract subdomain from `req.Host` header (fallback if SNI missed)
     - Lookup tunnel via `tunnelManager.LookupHTTP(subdomain)`
     - If not found: return 502 "Tunnel not found"
     - Open yamux stream: `tunnelSession.Session.Open()`
     - Use `httputil.ReverseProxy` with custom `Transport` that writes to yamux stream
   - Custom `Transport.RoundTrip`:
     - Open yamux stream
     - Write HTTP request to stream
     - Read HTTP response from stream
     - Return response

7. **Implement WebSocket upgrade support**
   - Detect `Connection: Upgrade` + `Upgrade: websocket` headers
   - Hijack the HTTP connection (`http.Hijacker`)
   - Forward raw bytes bidirectionally (bypass `httputil.ReverseProxy` for WS)
   - Required for HMR (Vite/Next.js/React dev servers) — primary use case
   > [!RED-TEAM] `httputil.ReverseProxy` does NOT handle WebSocket upgrades. Without this, dev server HMR silently breaks.

8. **Implement forwarding headers**
   - `X-Forwarded-For`: client IP
   - `X-Forwarded-Proto`: "https"
   - `X-Forwarded-Host`: original Host header
   - Strip hop-by-hop headers

8. **Implement HTTP/80 listener**
   - Simple HTTP server on port 80
   - Default: 301 redirect to HTTPS equivalent
   - Optional: route via Host header (for non-TLS HTTP tunnels)

9. **Implement subdomain validation in TunnelManager**
   - Reserved subdomains: `www`, `api`, `admin`, `mail`, `ftp`, `localhost`, `tunnel`
   - Min 3 chars, max 63 chars
   - Alphanumeric + hyphens only, can't start/end with hyphen

10. **Implement random subdomain generation**
    - 8 lowercase alphanumeric chars
    - Retry if collision (unlikely with 36^8 space)

11. **Integrate into `server.go`**
    - Server.Start() spawns HTTPProxy.Start() alongside control listener
    - Server.Shutdown() closes both listeners

## Todo List
- [x] Add TLS/domain config fields
- [x] Implement certmagic TLS listener (+ DevMode self-signed fallback)
- [x] Implement SNI extraction from ClientHello
- [x] Implement subdomain parsing and validation
- [x] Implement random subdomain generation
- [x] Implement HTTP reverse proxy via yamux streams
- [x] Add WebSocket upgrade proxy (Connection: Upgrade handling)
- [x] Add forwarding headers (X-Forwarded-*)
- [x] Add libdns DNS provider dependency (cloudflare/route53/digitalocean)
- [x] Implement HTTP/80 redirect listener
- [x] Integrate HTTPProxy into Server lifecycle
- [x] Test: end-to-end HTTP tunnel (client → server → browser)

## Success Criteria
- `https://abc.example.com` routes to client's localhost:3000 through tunnel
- Let's Encrypt cert auto-issued on first request (or DevMode self-signed works)
- Invalid subdomain returns 502
- Reserved subdomains rejected at registration
- Forwarding headers present on proxied requests
- HTTP/80 redirects to HTTPS

## Risk Assessment
- **Risk:** DNS-01 challenge requires DNS provider API (not all providers supported)
  - **Mitigation:** Support multiple DNS providers via certmagic plugins. Fallback: manual cert path config.
- **Risk:** Cert rate limits from Let's Encrypt (50 certs/domain/week)
  - **Mitigation:** Wildcard cert covers ALL subdomains — only 1 cert needed, not per-subdomain.
- **Risk:** SNI missing (old clients, curl without SNI)
  - **Mitigation:** Fallback to HTTP Host header parsing after TLS termination.

## Security Considerations
- TLS 1.2 minimum (disable SSLv3, TLS 1.0, 1.1)
- HSTS headers on all responses
- Cert private key stored securely (certmagic handles permissions)
- DoS: limit concurrent connections per subdomain

## Next Steps
→ Phase 4: TCP Tunnel — similar proxy but for raw TCP port allocation
