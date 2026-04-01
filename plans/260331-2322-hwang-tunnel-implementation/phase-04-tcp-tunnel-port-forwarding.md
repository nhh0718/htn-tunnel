# Phase 4: TCP Tunnel — Port Forwarding

## Context Links
- [Plan Overview](plan.md)
- [Phase 2: Server Core](phase-02-server-core-listener-auth.md)

## Overview
- **Priority:** P2
- **Status:** completed
- **Effort:** 2h
- **Description:** Raw TCP port forwarding. Server allocates a random public port, listens on it, and forwards all connections through yamux streams to the client's local port.

## Key Insights
- Simpler than HTTP tunneling — no TLS, no headers, just bidirectional byte copy
- Server allocates port from configurable range (default 10000-65535)
- Each incoming TCP connection on the allocated port → new yamux stream
- Client receives yamux stream → dials local port → `io.Copy` in both directions
- Port released immediately when tunnel closes

## Requirements

### Functional
- Client sends `TunnelRequest{type: "tcp", local_port: 5432}`
- Server allocates random port in range → starts TCP listener on that port
- Server responds with `TunnelResponse{remote_port: 34567}`
- For each connection on port 34567: open yamux stream → proxy to client
- Client receives stream → connects to `localhost:5432` → bidirectional copy
- Port listener closes when tunnel session ends
- Max ports per token (default: 5)

### Non-Functional
- Port allocation < 1ms
- Zero-copy proxying where possible (`io.Copy` with `net.TCPConn` splice)
- Handle 100+ concurrent connections per allocated port

## Architecture

### TCP Tunnel Flow
```
Remote User → server:34567 (allocated port)
  → Server accepts TCP connection
  → Server opens yamux stream on tunnel session
  → Server sends small header on stream: "new TCP connection"
  → Bidirectional io.Copy: remote conn ↔ yamux stream
  → Client receives yamux stream
  → Client dials localhost:5432
  → Bidirectional io.Copy: yamux stream ↔ local conn
```

### Port Lifecycle
```
1. Client requests TCP tunnel
2. TunnelManager.RegisterTCP() → allocates port, starts listener
3. Listener accepts connections → opens yamux streams
4. Client disconnects OR tunnel closed
5. TunnelManager.UnregisterTCP() → closes listener, releases port
6. All active connections on that port drain and close
```

## Related Code Files

### Files to Create
- `internal/server/tcp-proxy.go` — TCPProxy struct, port listener, connection forwarding

### Files to Modify
- `internal/server/tunnel-manager.go` — port allocation logic, TCP listener management
- `internal/server/server.go` — integrate TCP tunnel handling in control stream

## Implementation Steps

1. **Create `internal/server/tcp-proxy.go`**

2. **Implement `TCPProxy` struct**
   - Fields: `tunnelManager *TunnelManager`
   - No long-running listener — each tunnel gets its own goroutine

3. **Implement `StartTCPTunnel(session *TunnelSession) error`**
   - Allocate port via `TunnelManager.RegisterTCP()`
   - Start TCP listener on `0.0.0.0:{allocated_port}`
   - Spawn goroutine for accept loop:
     ```go
     for {
         conn, err := listener.Accept()
         if err != nil { break }  // listener closed
         go handleTCPConnection(conn, session)
     }
     ```
   - Store listener in TunnelSession for cleanup

4. **Implement `handleTCPConnection(conn net.Conn, session *TunnelSession)`**
   - Open new yamux stream: `session.Session.Open()`
   - If yamux open fails: close conn, return (client may be disconnected)
   - Spawn two goroutines for bidirectional copy:
     ```go
     go io.Copy(stream, conn)   // remote → client
     go io.Copy(conn, stream)   // client → remote
     ```
   - Wait for both to finish (use `sync.WaitGroup` or `errgroup`)
   - Close both stream and conn

5. **Implement `StopTCPTunnel(port int)`**
   - Close the port listener (breaks accept loop)
   - TunnelManager.UnregisterTCP(port)
   - Active connections drain naturally (io.Copy returns on close)

6. **Update `TunnelManager.RegisterTCP`**
   - Allocate random port in `config.TCPPortRange`
   - Check port not already in `tcpTunnels` map
   - Try `net.Listen("tcp", fmt.Sprintf(":%d", port))` to verify available
   - If listen fails: retry with different port (max 100 attempts)
   - Store listener reference in TunnelSession
   - Return allocated port

7. **Update control stream handler in `server.go`**
   - On `TunnelRequest{type: "tcp"}`:
     - Check rate limit + tunnel count
     - Call `tcpProxy.StartTCPTunnel()`
     - Send `TunnelResponse{remote_port: port}`
   - On session close:
     - Close all TCP tunnel listeners for that session

8. **Add cleanup on session disconnect**
   - When yamux session closes (client disconnect):
     - Iterate all tunnels owned by that session
     - Call `StopTCPTunnel()` for each
     - Release tunnel count in TokenStore

## Todo List
- [x] Create TCPProxy struct
- [x] Implement port allocation with listener verification
- [x] Implement per-port accept loop
- [x] Implement bidirectional io.Copy proxy
- [x] Implement tunnel stop + port release
- [x] Update control stream handler for TCP requests
- [x] Add session disconnect cleanup
- [x] Test: TCP tunnel round-trip (telnet → server port → client local)

## Success Criteria
- Client requests TCP tunnel → server allocates port + responds
- `telnet server:34567` connects through to client's `localhost:5432`
- Data flows bidirectionally without corruption
- Port released immediately when tunnel closed
- Multiple concurrent connections to same port work independently
- Session disconnect cleans up all allocated ports

## Risk Assessment
- **Risk:** Port exhaustion (attacker requests many TCP tunnels)
  - **Mitigation:** Max ports per token (5). Max total TCP tunnels configurable.
- **Risk:** Port conflict with system services
  - **Mitigation:** Default range 10000-65535 avoids well-known ports. Verify with net.Listen before committing.
- **Risk:** Slow client causes memory buildup (buffered data in yamux)
  - **Mitigation:** yamux has built-in flow control. Set max stream window size.

## Security Considerations
- TCP tunnels expose raw ports — no TLS termination (user's responsibility)
- **Access control for TCP tunnel ports:** Accept connections only from IPs that present a valid tunnel access token via initial handshake OR document that TCP ports are fully open and users must implement their own auth at the application layer
- Log allocated port + token for audit trail via `log/slog`
- Firewall: only open port range in iptables/ufw
> [!RED-TEAM] TCP tunnel ports are exposed to entire internet without auth. For MVP: document this clearly as a known limitation. Users tunneling databases/SSH must use application-level auth. Future: optional PROXY protocol header for IP allowlisting.

## Next Steps
→ Phase 5: Client CLI — implements the client side that dials local ports on yamux stream arrival
