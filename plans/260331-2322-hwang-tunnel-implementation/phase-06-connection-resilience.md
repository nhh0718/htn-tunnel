# Phase 6: Connection Resilience

## Context Links
- [Plan Overview](plan.md)
- [Phase 5: Client CLI](phase-05-client-cli.md)
- [Tunneling Protocols Research](../reports/researcher-260331-2319-tunneling-protocols.md)

## Overview
- **Priority:** P2
- **Status:** completed
- **Effort:** 2h
- **Description:** Heartbeat keepalive, dead connection detection, exponential backoff reconnection, and tunnel state recovery.

## Key Insights
- Network interruptions are inevitable — client MUST auto-reconnect
- Heartbeat detects dead connections faster than TCP keepalive (default 2h on Linux)
- Exponential backoff prevents thundering herd on server restart
- Subdomain reservation: server holds subdomain for 60s after disconnect → client can reclaim
- yamux has built-in keepalive, but we add application-level heartbeat for reliability

## Requirements

### Functional
- Client sends Heartbeat every 30s
- Server responds HeartbeatAck within 5s
- 3 missed heartbeats → connection considered dead
- Client auto-reconnects with exponential backoff: 1s, 2s, 4s, 8s, 16s, 32s, 60s (cap)
- On reconnect: re-authenticate, re-request same tunnel (same subdomain/type)
- Server holds subdomain reservation for 60s after client disconnect
- Reconnect status printed to terminal

### Non-Functional
- Heartbeat overhead: < 100 bytes/30s per connection
- Reconnect latency: < reconnect_delay + 2s (auth + tunnel request)
- No data loss during reconnect (active streams are lost; new streams work)

## Architecture

### Heartbeat Flow
```
Client                          Server
  │                               │
  │── Heartbeat ──────────────►   │
  │                               │── (reset timeout)
  │   ◄──────────── HeartbeatAck ─│
  │── (reset timeout)             │
  │                               │
  │   ... 30s later ...           │
  │                               │
  │── Heartbeat ──────────────►   │
  │   ◄──────────── HeartbeatAck ─│
  │                               │
  │   ... network dies ...        │
  │                               │
  │── Heartbeat ────── X          │
  │   (no ack, attempt 1)        │── (no heartbeat for 90s)
  │── Heartbeat ────── X          │── (mark session dead)
  │   (no ack, attempt 2)        │── (release tunnels after 60s hold)
  │── Heartbeat ────── X          │
  │   (no ack, attempt 3)        │
  │── DEAD: start reconnect      │
```

### Reconnection Flow
```
1. Connection detected dead (3 missed heartbeats OR io error)
2. Close yamux session + TCP conn
3. Print "Disconnected. Reconnecting..."
4. Wait backoff delay (starts 1s)
5. TCP dial server
6. Send Auth → validate
7. Send TunnelRequest (same subdomain) → if still reserved, success
8. If subdomain taken: server assigns new random subdomain
9. Print new tunnel info
10. Reset backoff to 1s
11. Resume stream accept loop
```

## Related Code Files

### Files to Create
- `internal/client/reconnect.go` — Reconnector struct, backoff logic, heartbeat sender

### Files to Modify
- `internal/client/client.go` — integrate heartbeat + reconnect
- `internal/server/server.go` — heartbeat responder, session timeout
- `internal/server/tunnel-manager.go` — subdomain reservation with TTL

## Implementation Steps

1. **Create `internal/client/reconnect.go`**
   - `Reconnector` struct:
     - `client *Client`
     - `tunnelType string` (http/tcp)
     - `localPort int`
     - `subdomain string`
     - `backoff time.Duration`
     - `maxBackoff time.Duration` (60s)
     - `baseBackoff time.Duration` (1s)
   - `NewReconnector(client, tunnelType, localPort, subdomain) *Reconnector`
   - `Reconnector.Run(ctx context.Context) error`
     - Loop until ctx cancelled:
       - Try `client.Connect()`
       - If fail: print error, sleep backoff, double backoff (cap at max), continue
       - If success: request tunnel (same subdomain), reset backoff
       - Start heartbeat + serve loop
       - On serve error (disconnect): continue loop (reconnect)

2. **Implement heartbeat sender in client**
   - `Client.startHeartbeat(ctx context.Context) error`
   - Goroutine: every 30s send Heartbeat via encoder
   - After send: wait 5s for HeartbeatAck
   - Track consecutive misses; after 3 → return error (triggers reconnect)
   - Run in parallel with serve loop; first error cancels both

3. **Implement heartbeat responder in server**
   - In `handleControlStream()`: when receiving Heartbeat message
   - Send HeartbeatAck immediately
   - Reset server-side session timeout timer

4. **Implement server-side session timeout**
   - Per session: `time.AfterFunc(90s, markDead)`
   - Reset timer on every heartbeat received
   - On timeout:
     - Close yamux session
     - Mark tunnels as "reserved" (not "active")
     - Start 60s reservation timer
     - On reservation expire: fully unregister tunnels

5. **Update TunnelManager for subdomain reservation (atomic)**
   - `TunnelSession` add field: `Reserved bool`, `ReservedUntil time.Time`
   - `LookupHTTP()`: skip reserved sessions (return nil)
   - `RegisterHTTP()`: **atomic check-and-reserve under single mutex lock** — check reservation status + register in one critical section to prevent race
   - If subdomain is reserved by same token → reclaim
   - If subdomain is reserved by different token → reject
   - Background goroutine: clean expired reservations every 30s — **also holds mutex during cleanup** to prevent race with concurrent reclaim
   > [!RED-TEAM] Original design had race between cleanup goroutine and reconnecting client. Atomic check-and-reserve fixes this.

6. **Integrate reconnect into CLI commands**
   - `http` command: wrap serve loop in `Reconnector.Run(ctx)`
   - `tcp` command: same
   - Print status changes: "Disconnected", "Reconnecting (attempt 2, next in 4s)...", "Connected"

7. **Add yamux config tuning**
   - `yamux.Config.EnableKeepAlive = true`
   - `yamux.Config.KeepAliveInterval = 30s` (belt + suspenders with app heartbeat)
   - `yamux.Config.ConnectionWriteTimeout = 10s`

## Todo List
- [x] Create Reconnector with exponential backoff
- [x] Implement client heartbeat sender (30s interval, 3 miss threshold)
- [x] Implement server heartbeat responder + session timeout
- [x] Add subdomain reservation with 60s TTL
- [x] Integrate Reconnector into CLI commands
- [x] Tune yamux keepalive settings
- [x] Test: kill connection → verify auto-reconnect + subdomain recovery

## Success Criteria
- Network interruption → client reconnects within backoff + 2s
- Same subdomain reclaimed after reconnect (within 60s)
- No panic/crash on disconnect
- Backoff caps at 60s
- Server cleans up dead sessions after timeout
- Terminal shows clear reconnection status

## Risk Assessment
- **Risk:** Heartbeat floods on many clients
  - **Mitigation:** 1 heartbeat/30s = negligible. 1000 clients = 33 heartbeats/sec.
- **Risk:** Subdomain squatting via reservation
  - **Mitigation:** 60s TTL is short. Only same-token can reclaim. Different token gets random subdomain.
- **Risk:** Race condition: client reconnects while server still cleaning up
  - **Mitigation:** Server checks reservation token match before reclaim. Mutex protects TunnelManager.

## Security Considerations
- Heartbeat doesn't carry auth info (session already authenticated)
- Reservation tied to token — prevents subdomain theft during reconnect window

## Next Steps
→ Phase 7: Deployment — package for production use
