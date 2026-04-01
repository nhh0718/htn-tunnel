# Phase 5: Client CLI

## Context Links
- [Plan Overview](plan.md)
- [Phase 1: Wire Protocol](phase-01-project-setup-wire-protocol.md)
- [Deployment & UX Research](../reports/researcher-260331-2319-deployment-ux-patterns.md)

## Overview
- **Priority:** P1
- **Status:** completed
- **Effort:** 2h
- **Description:** Cobra-based CLI client. Single binary. Simple commands: `hwang-tunnel http 3000`, `hwang-tunnel tcp 5432`, `hwang-tunnel auth <token>`. Connects to server, authenticates, requests tunnel, displays URL, proxies traffic to local port.

## Key Insights
- ngrok's killer UX: `ngrok http 3000` → prints URL in <2 seconds
- Client stores auth token in `~/.hwang-tunnel/config.yaml` — set once via `hwang-tunnel auth`
- Server address configurable via config file or `--server` flag
- Client is the yamux CLIENT (server.go creates yamux Server); client accepts streams opened by server
- For HTTP tunnel: client receives yamux stream → reads HTTP request → dials localhost → proxies
- For TCP tunnel: client receives yamux stream → dials localhost:port → io.Copy bidirectional

## Requirements

### Functional
- Commands:
  - `hwang-tunnel http <port>` — create HTTP tunnel to localhost:<port>
  - `hwang-tunnel tcp <port>` — create TCP tunnel to localhost:<port>
  - `hwang-tunnel auth <token>` — save auth token to config
  - `hwang-tunnel status` — show active tunnel info (nice-to-have)
- Flags:
  - `--server <addr>` — override server address
  - `--subdomain <name>` — request specific subdomain (HTTP only)
  - `--token <token>` — override auth token
- Config file: `~/.hwang-tunnel/config.yaml`
- Output: print tunnel URL immediately on success
- Graceful shutdown: Ctrl+C closes tunnel cleanly

### Non-Functional
- Binary < 15MB (standard Go binary)
- Connect + auth + display URL in < 3 seconds
- Clear error messages on failure (connection refused, auth failed, etc.)

## Architecture

### Client Flow
```
1. Parse CLI args (port, flags)
2. Load config (token, server addr)
3. TCP connect to server control port
4. Send Auth message → receive AuthResponse
5. Send TunnelRequest → receive TunnelResponse
6. Print tunnel URL/port
7. Enter stream accept loop:
   For HTTP: accept yamux streams → proxy HTTP to localhost:port
   For TCP:  accept yamux streams → dial localhost:port → io.Copy
8. On Ctrl+C: close yamux session → exit
```

### yamux Role Clarification
```
Server side: yamux.Server(conn)  — opens streams TO client
Client side: yamux.Client(conn)  — accepts streams FROM server

HTTP tunnel:
  Browser → Server → server opens yamux stream → client accepts → dials localhost

TCP tunnel:
  Remote user → allocated port → server opens yamux stream → client accepts → dials localhost
```

### Output Format
```
hwang-tunnel v0.1.0

  Status:    connected
  Tunnel:    https://abc123.example.com → localhost:3000
  Server:    tunnel.example.com:4443

  Ctrl+C to disconnect
```

For TCP:
```
hwang-tunnel v0.1.0

  Status:    connected
  Tunnel:    tcp://tunnel.example.com:34567 → localhost:5432
  Server:    tunnel.example.com:4443

  Ctrl+C to disconnect
```

## Related Code Files

### Files to Create
- `internal/client/client.go` — Client struct, Connect(), auth handshake
- `internal/client/tunnel.go` — stream accept loop, local port forwarding
- `cmd/client/main.go` — Cobra root command + subcommands

### Files to Modify
- `internal/config/config.go` — add ClientConfig with Load/Save helpers

## Implementation Steps

1. **Add ClientConfig to `internal/config/config.go`**
   - `ClientConfig` struct: `ServerAddr string`, `Token string`
   - `LoadClientConfig() (*ClientConfig, error)` — reads `~/.hwang-tunnel/config.yaml`
   - `SaveClientConfig(cfg *ClientConfig) error` — writes config file
   - Create config dir if not exists

2. **Create `internal/client/client.go`**
   - `Client` struct: `config *ClientConfig`, `conn net.Conn`, `session *yamux.Session`, `encoder`, `decoder`
   - `NewClient(config *ClientConfig) *Client`
   - `Client.Connect(ctx context.Context) error`
     - TCP dial to `config.ServerAddr`
     - Create protocol encoder/decoder on conn
     - Send `Auth{Token: config.Token}`
     - Read `AuthResponse` — return error if not success
     - Create yamux.Client(conn) — client is the yamux initiator but ACCEPTS streams
     - Store session
   - `Client.Close() error` — close session + conn

3. **Create `internal/client/tunnel.go`**
   - `Client.RequestHTTPTunnel(localPort int, subdomain string) (*TunnelInfo, error)`
     - Open control stream on yamux session
     - Send `TunnelRequest{type: "http", subdomain: subdomain, local_port: localPort}`
     - Read `TunnelResponse` — extract URL
     - Return `TunnelInfo{URL, LocalPort}`
   - `Client.RequestTCPTunnel(localPort int) (*TunnelInfo, error)`
     - Same pattern but type: "tcp", extract remote_port
   - `Client.ServeHTTPTunnel(ctx context.Context, localPort int) error`
     - Loop: accept yamux streams
     - Per stream goroutine:
       - Dial `localhost:{localPort}`
       - Bidirectional io.Copy (stream ↔ local conn)
       - Close both on done
   - `Client.ServeTCPTunnel(ctx context.Context, localPort int) error`
     - Same as ServeHTTPTunnel — identical logic for TCP
     - (HTTP and TCP both just proxy bytes; server handles protocol differences)

4. **Create `cmd/client/main.go` with Cobra**
   - Root command: `hwang-tunnel`
   - `http` subcommand:
     ```
     hwang-tunnel http <port> [--subdomain name] [--server addr] [--token token]
     ```
     - Load config, override with flags
     - Connect, request HTTP tunnel, print URL
     - ServeHTTPTunnel in foreground
     - Handle SIGINT → graceful close
   - `tcp` subcommand:
     ```
     hwang-tunnel tcp <port> [--server addr] [--token token]
     ```
     - Same flow but RequestTCPTunnel
   - `auth` subcommand:
     ```
     hwang-tunnel auth <token> [--server addr]
     ```
     - Save token (and optionally server addr) to config file
     - Print confirmation

5. **Implement signal handling**
   - `signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)`
   - Pass ctx to ServeHTTPTunnel/ServeTCPTunnel
   - On cancel: close yamux session → accept loop breaks → exit

6. **Implement output formatting**
   - Print banner with version
   - Print status, tunnel URL, server address
   - Use `fmt.Fprintf` — no color libraries (keep deps minimal)

## Todo List
- [x] Add ClientConfig with load/save
- [x] Implement Client.Connect() with auth handshake
- [x] Implement tunnel request (HTTP + TCP)
- [x] Implement stream accept loop + local forwarding
- [x] Create Cobra commands (http, tcp, auth)
- [x] Implement signal handling + graceful shutdown
- [x] Implement output formatting
- [x] Test: full tunnel round-trip (client → server → browser/telnet → localhost)

## Success Criteria
- `hwang-tunnel auth <token>` saves token to config
- `hwang-tunnel http 3000` connects, prints URL in < 3s
- `hwang-tunnel tcp 5432` connects, prints port in < 3s
- Ctrl+C cleanly disconnects
- `--server` and `--subdomain` flags work
- Clear error message on connection refused / auth failure

## Risk Assessment
- **Risk:** yamux client/server role confusion (who opens streams?)
  - **Mitigation:** Document clearly: server opens streams to client. Client accepts streams. This is the reverse of typical yamux usage. Test this assumption early.
- **Risk:** Local service not running → connection refused on dial
  - **Mitigation:** Client logs "connection to localhost:{port} refused" per failed stream, doesn't crash. Retry on next stream.

## Security Considerations
- Config file permissions: `0600` (owner read/write only)
- Token visible in config file — warn user in `auth` command output
- Never log the full token; mask: `tok_xxxx...xxxx`

## Next Steps
→ Phase 6: Connection Resilience — adds heartbeat + reconnection to the client
