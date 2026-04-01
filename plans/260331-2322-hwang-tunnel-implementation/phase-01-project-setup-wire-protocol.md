# Phase 1: Project Setup & Wire Protocol

## Context Links
- [Plan Overview](plan.md)
- [Tunneling Protocols Research](../reports/researcher-260331-2319-tunneling-protocols.md)

## Overview
- **Priority:** P1
- **Status:** completed
- **Effort:** 2h
- **Description:** Initialize Go module, install dependencies, define wire protocol message types, implement length-prefixed binary codec with unit tests.

## Key Insights
- Bore uses ~400 LOC with custom binary protocol — proves simplicity works
- Length-prefixed framing avoids delimiter escaping issues
- yamux handles multiplexing; our protocol only needs control messages (auth, tunnel requests, heartbeat)
- Data frames flow through yamux streams directly — no need to wrap them in our protocol

## Requirements

### Functional
- Go module initialized with all dependencies
- Message types: `Auth`, `AuthResponse`, `TunnelRequest`, `TunnelResponse`, `Heartbeat`, `HeartbeatAck`
- Binary codec: encode message → `[4-byte length][1-byte type][payload]` → decode message
- Round-trip encode/decode for every message type

### Non-Functional
- Codec must handle partial reads (network fragmentation)
- Zero allocations for heartbeat encode/decode (hot path)

## Architecture

### Wire Protocol Format
```
┌──────────────┬─────────────┬──────────┬─────────────────┐
│ Length (4B)   │ Version (1B)│ Type (1B)│ Payload (N B)   │
│ big-endian   │ 0x01        │ enum     │ JSON or binary   │
└──────────────┴─────────────┴──────────┴─────────────────┘
```

**Length** = size of Version + Type + Payload (excludes the 4-byte length field itself).
**Version** = protocol version byte (starts at `0x01`). Reject unknown versions with error.
> [!RED-TEAM] Added version byte to prevent breaking changes without negotiation path.

### Message Types (1-byte enum)
```
0x01  Auth            { token: string }
0x02  AuthResponse    { success: bool, message: string }
0x03  TunnelRequest   { type: "http"|"tcp", subdomain?: string, local_port: int (1-65535) }
0x04  TunnelResponse  { success: bool, url?: string, remote_port?: int, message?: string }
```
> [!RED-TEAM] `local_port` MUST be validated: range 1-65535, reject 0 or negative. Subdomain validated server-side (alphanumeric+hyphens, 3-63 chars).
```
0x05  Heartbeat       {} (empty payload)
0x06  HeartbeatAck    {} (empty payload)
```

### Payload Encoding
- Use JSON for control messages (Auth, TunnelRequest, etc.) — simple, debuggable
- Heartbeat/HeartbeatAck have zero-length payload — just the type byte
- Actual tunnel data flows through yamux streams, NOT through this protocol

## Related Code Files

### Files to Create
- `go.mod` — module definition + dependencies
- `internal/protocol/message.go` — message type constants, structs
- `internal/protocol/codec.go` — `Encoder` and `Decoder` types wrapping `io.Writer`/`io.Reader`
- `internal/protocol/codec_test.go` — unit tests

## Implementation Steps

1. **Init Go module**
   ```bash
   go mod init github.com/htn-sys/htn-tunnel
   ```

2. **Add dependencies**
   ```bash
   go get github.com/hashicorp/yamux
   go get github.com/spf13/cobra
   go get github.com/caddyserver/certmagic
   go get golang.org/x/time/rate
   go get gopkg.in/yaml.v3
   ```

3. **Create `internal/protocol/message.go`**
   - Define `MsgType` as `uint8` with constants for each message
   - Define structs: `AuthMsg`, `AuthResponseMsg`, `TunnelRequestMsg`, `TunnelResponseMsg`
   - Each struct has JSON tags for encoding
   - `TunnelRequestMsg.Type` field: `"http"` or `"tcp"`

4. **Create `internal/protocol/codec.go`**
   - `Encoder` struct wrapping `io.Writer` + `sync.Mutex` (thread-safe writes)
   - `Encoder.Encode(msgType MsgType, payload interface{}) error`
     - JSON-marshal payload (skip for heartbeat)
     - Write 4-byte big-endian length
     - Write 1-byte type
     - Write payload bytes
   - `Decoder` struct wrapping `io.Reader`
   - `Decoder.Decode() (MsgType, []byte, error)`
     - Read 4-byte length
     - Read 1-byte type
     - Read payload bytes
     - Return type + raw JSON bytes (caller unmarshals)
   - Helper: `DecodeAs[T](decoder) (T, error)` — generic decode + unmarshal

5. **Create `internal/protocol/codec_test.go`**
   - Test round-trip for each message type
   - Test heartbeat (zero payload)
   - Test oversized message rejection (>1MB limit)
   - Test concurrent encode (mutex safety)
   - Benchmark heartbeat encode/decode

6. **PoC: yamux stream direction spike** *(Critical — validates core assumption)*
   - Write a minimal test: server creates `yamux.Server`, then calls `session.Open()` to push stream to client
   - Client creates `yamux.Client`, calls `session.AcceptStream()`
   - Confirm bidirectional data flow (server-opens-to-client pattern)
   - If this fails: redesign to client-opens-stream pattern in Phase 2
   > [!RED-TEAM] yamux stream direction must be validated before building Phases 2-5 on this assumption.

7. **Create placeholder `cmd/server/main.go` and `cmd/client/main.go`**
   - Just `package main` + `func main()` stubs

8. **Create `Makefile`** with basic targets
   - `build-server`, `build-client`, `test`, `lint`

9. **Run tests, verify compilation**

## Todo List
- [x] Init Go module + dependencies
- [x] Define message types and structs (`message.go`)
- [x] Implement encoder/decoder (`codec.go`)
- [x] Write unit tests (`codec_test.go`)
- [x] Create cmd entrypoint stubs
- [x] Create Makefile
- [x] PoC: yamux stream direction spike (server→client Open())
- [x] Add input validation for TunnelRequest fields
- [x] Verify `go build ./...` and `go test ./...` pass

## Success Criteria
- `go build ./...` compiles without errors
- All codec tests pass
- Round-trip encode/decode preserves all fields
- Heartbeat encode/decode allocates zero bytes (verified via benchmark)

## Risk Assessment
- **Risk:** JSON encoding overhead for control messages
  - **Mitigation:** Only control messages use JSON; data flows through yamux directly. Control messages are infrequent.
- **Risk:** Message size attacks (client sends 4GB length)
  - **Mitigation:** Hard cap at 1MB per control message. Reject + close connection on violation.

## Security Considerations
- Max message size enforced in decoder (1MB)
- Codec is symmetric — same code used by server and client (shared `internal/protocol` package)

## Next Steps
→ Phase 2: Server Core — use this codec for the control connection handshake
