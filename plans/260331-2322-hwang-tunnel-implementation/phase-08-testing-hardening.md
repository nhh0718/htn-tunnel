# Phase 8: Testing & Hardening

## Context Links
- [Plan Overview](plan.md)
- All previous phases

## Overview
- **Priority:** P2
- **Status:** completed
- **Effort:** 2h
- **Description:** Integration tests (end-to-end tunnel round-trip), load testing, security hardening, and graceful server shutdown.

## Key Insights
- Unit tests in Phase 1 cover codec; this phase covers integration (client ↔ server ↔ local service)
- Use `net.Pipe()` or localhost for integration tests — no real VPS needed
- Load test: spin up N clients, each with HTTP tunnel, blast requests
- Security: enforce limits aggressively (connections, bandwidth, tunnel lifetime)
- Graceful shutdown: stop accepting new connections, drain existing ones with timeout

## Requirements

### Functional
- Integration test: HTTP tunnel round-trip (client → server → HTTP request → localhost → response)
- Integration test: TCP tunnel round-trip (client → server → TCP data → localhost → echo back)
- Integration test: auth failure (bad token → rejected)
- Integration test: rate limiting (exceed limit → 429)
- Integration test: reconnection (kill connection → client reconnects → tunnel works again)
- Load test: 50 concurrent tunnels, 100 req/s per tunnel
- Graceful server shutdown: SIGTERM → drain connections (30s timeout) → exit

### Non-Functional
- Integration tests complete in < 30s
- Load test targets: 5000 req/s total throughput
- Zero goroutine leaks (verified via `goleak`)

## Architecture

### Test Setup
```
Integration Test Process
  ├── Start server (localhost:4443, DevMode=true)
  ├── Start local HTTP server (localhost:9999)
  ├── Create client, connect, request HTTP tunnel
  ├── HTTP GET https://{subdomain}.localhost → proxied to localhost:9999
  ├── Assert response matches expected
  └── Cleanup: close client, stop server
```

### Test Matrix
```
Test                              Type         Priority
Codec round-trip                  Unit         P1 (Phase 1)
Auth success                      Integration  P1
Auth failure (bad token)          Integration  P1
Auth failure (rate limited)       Integration  P1
HTTP tunnel round-trip            Integration  P1
HTTP tunnel — subdomain routing   Integration  P1
HTTP tunnel — forwarding headers  Integration  P2
TCP tunnel round-trip             Integration  P1
TCP tunnel — multiple connections Integration  P2
Reconnection recovery             Integration  P2
Graceful shutdown                 Integration  P2
50 concurrent tunnels             Load         P2
Goroutine leak check              Stress       P2
```

## Related Code Files

### Files to Create
- `internal/server/server_test.go` — integration tests
- `internal/client/client_test.go` — client integration tests
- `test/integration_test.go` — end-to-end tests (top-level)
- `test/load_test.go` — load/stress tests

### Files to Modify
- `internal/server/server.go` — graceful shutdown improvements
- `internal/server/tunnel-manager.go` — connection limits, tunnel lifetime

## Implementation Steps

1. **Create test helpers**
   - `test/helpers.go`:
     - `startTestServer(t) (*server.Server, *config.ServerConfig)` — DevMode, random ports
     - `startTestClient(t, serverAddr, token) *client.Client`
     - `startLocalHTTPServer(t, port) *http.Server` — simple echo handler
     - `startLocalTCPServer(t, port)` — echo TCP server

2. **Write auth integration tests** (`internal/server/server_test.go`)
   - `TestAuthSuccess`: valid token → AuthResponse{success: true}
   - `TestAuthFailure`: invalid token → AuthResponse{success: false}, conn closed
   - `TestAuthRateLimit`: send 101 auth attempts → last rejected

3. **Write HTTP tunnel integration tests** (`test/integration_test.go`)
   - `TestHTTPTunnelRoundTrip`:
     - Start server (DevMode) + local HTTP echo server
     - Client connects, requests HTTP tunnel
     - HTTP GET to tunnel URL → expect echo response
   - `TestHTTPTunnelSubdomainRouting`:
     - Two clients, different subdomains
     - Requests route to correct local service
   - `TestHTTPTunnelForwardingHeaders`:
     - Verify X-Forwarded-For, X-Forwarded-Proto present

4. **Write TCP tunnel integration tests**
   - `TestTCPTunnelRoundTrip`:
     - Start server + local TCP echo server
     - Client connects, requests TCP tunnel
     - Connect to allocated port → send data → expect echo
   - `TestTCPTunnelMultipleConnections`:
     - 10 concurrent TCP connections to same port → all get responses

5. **Write reconnection test**
   - `TestReconnection`:
     - Client connects, establishes tunnel
     - Force-close server-side connection
     - Wait for client reconnect (poll tunnel URL)
     - Verify tunnel works after reconnect
     - Verify same subdomain reclaimed

6. **Write load test** (`test/load_test.go`)
   - `TestLoadConcurrentTunnels`:
     - Start server
     - 50 goroutines: each creates client + HTTP tunnel + makes 100 requests
     - Assert all 5000 requests succeed
     - Measure: p50/p95/p99 latency, throughput
   - Use `testing.B` for benchmark variant

7. **Implement graceful server shutdown**
   - `Server.Shutdown(ctx context.Context) error`:
     - Stop accepting new connections (close listener)
     - Signal all active sessions to drain
     - Wait for sessions to close (or ctx deadline)
     - Close TunnelManager (release all ports)
   - 30s default drain timeout
   - Active yamux streams finish current request, then close

8. **Add security hardening**
   - Max connections per IP: 10 (prevent single IP exhaustion)
   - Max tunnel lifetime: 24h (configurable, 0 = unlimited)
   - Max message size: 1MB (already in codec)
   - Max concurrent streams per session: 256
   - Connection timeout: 5s for auth, 30s idle without heartbeat

9. **Add goroutine leak detection**
   - Use `go.uber.org/goleak` in `TestMain`
   - Verify zero leaked goroutines after each test

10. **Add `go vet` and `staticcheck` to CI**
    - Makefile `lint` target: `go vet ./... && staticcheck ./...`

## Todo List
- [x] Create test helpers (server/client/local service starters)
- [x] Write auth integration tests (success, failure, rate limit)
- [x] Write HTTP tunnel integration tests (round-trip, subdomain, headers)
- [x] Write TCP tunnel integration tests (round-trip, concurrent)
- [x] Write reconnection integration test
- [x] Write load test (50 tunnels, 5000 requests)
- [x] Implement graceful server shutdown with drain
- [x] Add security limits (per-IP, lifetime, streams)
- [x] Add goroutine leak detection
- [x] Run full test suite, fix failures

## Success Criteria
- All integration tests pass
- Load test: 5000 req/s with < 100ms p99 latency (localhost)
- Zero goroutine leaks
- `go vet` + `staticcheck` clean
- Graceful shutdown drains within 30s
- Auth failures logged with IP

## Risk Assessment
- **Risk:** Integration tests flaky due to port conflicts
  - **Mitigation:** Use random ports for all test servers. Use `net.Listen(":0")` for OS-assigned ports.
- **Risk:** Load test too slow in CI
  - **Mitigation:** Tag load tests as `//go:build load` — run only locally or in nightly CI.

## Security Considerations
- All hardening limits configurable (not hardcoded)
- Log abuse attempts (rate limit hits, auth failures) with timestamp + IP
- Consider `fail2ban`-style IP blocking for repeated auth failures (post-MVP)

## Next Steps
After Phase 8 is complete, the project is ready for:
- Beta deployment on VPS
- DNS configuration (wildcard A record)
- First real tunnel test over the internet
- GitHub release with cross-compiled binaries
