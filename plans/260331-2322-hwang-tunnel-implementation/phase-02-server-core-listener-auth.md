# Phase 2: Server Core — Listener & Auth

## Context Links
- [Plan Overview](plan.md)
- [Phase 1: Wire Protocol](phase-01-project-setup-wire-protocol.md)
- [Deployment & UX Research](../reports/researcher-260331-2319-deployment-ux-patterns.md)

## Overview
- **Priority:** P1
- **Status:** completed
- **Effort:** 3h
- **Description:** Server TCP listener, control connection handler, yamux session establishment, token-based auth, rate limiting, and tunnel session registry.

## Key Insights
- Server accepts one TCP control connection per client
- yamux multiplexes streams over that single connection
- First message on control conn MUST be `Auth` — reject if not
- Rate limiter uses `golang.org/x/time/rate` token bucket — proven, stdlib-adjacent
- Tunnel manager is a concurrent map: `subdomain → *TunnelSession` and `port → *TunnelSession`

## Requirements

### Functional
- **TLS** listener on configurable port (default 4443 for control plane) — uses same certmagic/TLS config as Phase 3
- Accept client connections over TLS, read Auth message, validate token
> [!RED-TEAM] Control port MUST use TLS. Tokens sent plaintext = showstopper for public service.
- Establish yamux session over authenticated connection
- Token store loaded from config file or env vars
- **Pre-auth per-IP rate limiter**: 10 conn/min per IP before token validation (prevents unauthenticated floods)
- Rate limiter: per-token (100 req/min) and global (1000 req/min)
- Tunnel session registry: register/unregister/lookup by subdomain or port
- Max tunnels per token (default: 10)
- Graceful connection close on auth failure
> [!RED-TEAM] Per-IP rate limit BEFORE auth prevents DoS via unauthenticated connection floods.

### Non-Functional
- Handle 1000+ concurrent control connections
- Tunnel registry operations < 1ms
- Thread-safe: all shared state protected by mutex or sync.Map

## Architecture

### Connection Flow
```
Client TLS connect → Server accepts (TLS on control port)
  → Pre-auth IP rate limit check (reject if exceeded)
  → Read Auth message (5s timeout)
  → Validate token against token store
  → If invalid: send AuthResponse{success:false}, close
  → If valid: send AuthResponse{success:true}
  → Upgrade connection to yamux session
  → Listen for TunnelRequest messages on stream 0
  → For each TunnelRequest: register tunnel, send TunnelResponse
```

### Server Components
```
Server
├── TCP Listener (port 4443)
├── Auth Validator
│   ├── Token Store (map[string]*TokenInfo)
│   └── Rate Limiter (per-token + global)
├── Tunnel Manager
│   ├── HTTP Tunnels (map[subdomain]*TunnelSession)
│   └── TCP Tunnels (map[port]*TunnelSession)
└── Session Pool (map[token][]*yamux.Session)
```

### TunnelSession struct
```go
type TunnelSession struct {
    ID        string
    Token     string          // owning auth token
    Type      string          // "http" or "tcp"
    Subdomain string          // for HTTP tunnels
    Port      int             // for TCP tunnels (server-side allocated port)
    LocalPort int             // client's local port
    Session   *yamux.Session  // yamux session to client
    CreatedAt time.Time
}
```

## Related Code Files

### Files to Create
- `internal/server/server.go` — main Server struct, Start(), accept loop
- `internal/server/auth.go` — TokenStore, RateLimiter, validateToken()
- `internal/server/tunnel-manager.go` — TunnelManager: register, unregister, lookup
- `internal/config/config.go` — ServerConfig, ClientConfig structs

### Files to Modify
- `cmd/server/main.go` — wire up Server with config

## Implementation Steps

1. **Create `internal/config/config.go`**
   - `ServerConfig` struct: `ListenAddr`, `Domain`, `Tokens []string`, `MaxTunnelsPerToken int`, `RateLimit int`, `GlobalRateLimit int`, `TCPPortRange [2]int`
   - Load from YAML file (`/etc/hwang-tunnel/server.yaml` or `./server.yaml`)
   - Override from env vars: `HTN_LISTEN_ADDR`, `HTN_DOMAIN`, `HTN_TOKENS` (comma-separated), etc.
   - `ClientConfig` struct: `ServerAddr`, `Token`
   - Load from YAML (`~/.hwang-tunnel/config.yaml`)

2. **Create `internal/server/auth.go`**
   - `TokenStore` struct with `sync.RWMutex` + `map[string]*TokenInfo`
   - `TokenInfo`: `Token string`, `TunnelCount int`, `MaxTunnels int`
   - Tokens stored as **bcrypt hashes** (not plaintext). Config file holds raw tokens; on load, hash and store hashes only.
   - `TokenStore.Validate(token string) bool` — bcrypt.CompareHashAndPassword
   > [!RED-TEAM] Token hashing prevents credential theft from memory dumps or config leaks.
   - `TokenStore.IncrementTunnels(token string) error` — returns error if at max
   - `TokenStore.DecrementTunnels(token string)`
   - `RateLimiter` struct wrapping `map[string]*rate.Limiter` + global `*rate.Limiter`
   - `RateLimiter.Allow(token string) bool` — checks per-token AND global
   - Initialize limiters lazily on first request per token

3. **Create `internal/server/tunnel-manager.go`**
   - `TunnelManager` struct with `sync.RWMutex`
   - Fields: `httpTunnels map[string]*TunnelSession`, `tcpTunnels map[int]*TunnelSession`
   - `RegisterHTTP(subdomain string, session *TunnelSession) error` — error if taken
   - `RegisterTCP(session *TunnelSession) (int, error)` — allocate random port, return it
   - `UnregisterHTTP(subdomain string)`
   - `UnregisterTCP(port int)`
   - `LookupHTTP(subdomain string) *TunnelSession`
   - `LookupTCP(port int) *TunnelSession`
   - Port allocation: random in range 10000-65535, retry if taken (max 100 attempts)

4. **Create `internal/server/server.go`**
   - `Server` struct: `config`, `listener`, `tokenStore`, `rateLimiter`, `tunnelManager`
   - `NewServer(config *ServerConfig) *Server`
   - `Server.Start(ctx context.Context) error` — listen TCP, accept loop
   - `Server.handleConnection(conn net.Conn)` — goroutine per connection
     - Set 5s read deadline for Auth message
     - Decode Auth message using protocol codec
     - Validate token, check rate limit
     - Send AuthResponse
     - If success: create yamux.Server session over conn
     - Spawn `handleControlStream()` goroutine
   - `Server.handleControlStream(session *yamux.Session, token string)`
     - Accept streams from yamux session
     - First stream = control: read TunnelRequest messages in loop
     - For each request: call tunnelManager.Register*, send TunnelResponse
   - `Server.Shutdown(ctx context.Context) error` — close listener, drain sessions

5. **Wire up `cmd/server/main.go`**
   - Parse config file path from CLI flag
   - Load config
   - Create + start server
   - Handle SIGINT/SIGTERM → graceful shutdown

## Todo List
- [x] Create config structs with YAML + env var loading
- [x] Implement TokenStore with validation + tunnel counting
- [x] Implement RateLimiter (per-token + global)
- [x] Implement TunnelManager (register/unregister/lookup for HTTP + TCP)
- [x] Implement Server accept loop + auth handshake
- [x] Implement yamux session establishment
- [x] Implement control stream handler (TunnelRequest routing)
- [x] Wire up cmd/server/main.go
- [x] Add pre-auth per-IP rate limiter
- [x] Implement bcrypt token hashing
- [x] Add TLS to control port listener
- [x] Add structured logging with log/slog
- [x] Test: auth success/failure, rate limiting, tunnel registration

## Success Criteria
- Server starts, listens on configured port
- Client can connect, send Auth, receive AuthResponse
- Invalid token → connection closed with error message
- Rate limiter rejects when limit exceeded
- Tunnel registration prevents duplicate subdomains
- Graceful shutdown closes all connections

## Risk Assessment
- **Risk:** Token brute-force attacks
  - **Mitigation:** Rate limit unauthenticated connections per IP (10/min). Close connection immediately on auth failure.
- **Risk:** Connection exhaustion (too many concurrent clients)
  - **Mitigation:** `ulimit` on server, configurable max connections, close idle sessions after timeout.
- **Risk:** yamux session leak on crash
  - **Mitigation:** Defer session.Close() in handleConnection. TunnelManager cleanup on session close.

## Security Considerations
- Auth token transmitted over TLS on control port (TLS from day 1, not deferred to Phase 3)
- Tokens stored as bcrypt hashes in memory — raw tokens only in config file
- Token store is read-only at runtime (loaded from config); no admin API to add tokens
- Failed auth attempts logged with client IP via `log/slog` structured logging
> [!RED-TEAM] Use `log/slog` (Go stdlib) for structured JSON logging throughout. Add request correlation IDs.

## Next Steps
→ Phase 3: HTTP Tunnel adds TLS listener + SNI routing that uses TunnelManager.LookupHTTP()
