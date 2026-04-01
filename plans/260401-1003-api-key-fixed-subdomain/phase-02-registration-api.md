# Phase 2: Registration API + Fixed Subdomain

## Priority: HIGH | Status: Not started | Effort: 3-4h

## Overview
Server-side API cho user tự register key + claim subdomain. Không cần admin token để register — open registration.

## API Endpoints (trên control plane port 4443, qua TLS)

### Register (new protocol message)

Client sends `MsgRegister`:
```json
{
  "name": "Hoang",
  "subdomain": "hoang"
}
```

Server responds `MsgRegisterResponse`:
```json
{
  "success": true,
  "key": "htk_a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4",
  "subdomains": ["hoang"],
  "message": ""
}
```

Error cases:
- Subdomain taken: `{"success": false, "message": "subdomain 'hoang' is already owned"}`
- Subdomain invalid: `{"success": false, "message": "subdomain must be 3-63 chars..."}`
- Registration disabled: `{"success": false, "message": "registration is disabled"}`

### Claim additional subdomain (authenticated)

Client sends `MsgClaimSubdomain` (requires valid key):
```json
{
  "subdomain": "myapp"
}
```

Server responds `MsgClaimResponse`:
```json
{
  "success": true,
  "subdomains": ["hoang", "myapp"]
}
```

## Implementation Steps

### 1. Add protocol messages

**`internal/protocol/message.go`:**
```go
const (
    // ... existing ...
    MsgRegister         MsgType = 10
    MsgRegisterResponse MsgType = 11
    MsgClaimSubdomain   MsgType = 12
    MsgClaimResponse    MsgType = 13
)

type RegisterMsg struct {
    Name      string `json:"name"`
    Subdomain string `json:"subdomain"`
}

type RegisterResponseMsg struct {
    Success    bool     `json:"success"`
    Key        string   `json:"key,omitempty"`
    Subdomains []string `json:"subdomains,omitempty"`
    Message    string   `json:"message,omitempty"`
}

type ClaimSubdomainMsg struct {
    Subdomain string `json:"subdomain"`
}

type ClaimResponseMsg struct {
    Success    bool     `json:"success"`
    Subdomains []string `json:"subdomains,omitempty"`
    Message    string   `json:"message,omitempty"`
}
```

### 2. Handle registration on server

**`internal/server/server.go`** — in `handleClient` (before auth):

```go
// Read first message — could be Auth or Register
msgType, raw, err := dec.Decode()
switch msgType {
case protocol.MsgAuth:
    // existing auth flow
case protocol.MsgRegister:
    s.handleRegister(conn, enc, dec, raw, remoteIP)
    return // registration is a one-shot connection
default:
    // error
}
```

```go
func (s *Server) handleRegister(conn net.Conn, enc, dec, raw []byte, ip string) {
    if !s.cfg.AllowRegistration {
        enc.Encode(protocol.MsgRegisterResponse, RegisterResponseMsg{
            Success: false, Message: "registration is disabled",
        })
        return
    }

    var msg protocol.RegisterMsg
    json.Unmarshal(raw, &msg)

    // Validate subdomain
    if err := ValidateSubdomain(msg.Subdomain); err != nil {
        // respond error
        return
    }

    // Check subdomain not owned
    if owner := s.keyStore.FindSubdomainOwner(msg.Subdomain); owner != "" {
        // respond: subdomain taken
        return
    }

    // Create key
    key, err := s.keyStore.CreateKey(msg.Name, []string{msg.Subdomain}, 10)
    // respond with key
}
```

### 3. Add config field

**`internal/config/config.go`:**
```go
AllowRegistration bool `yaml:"allow_registration"` // default: true
```

### 4. Fixed subdomain validation in tunnel request

Reuse logic from original phase-02 plan:
- Key owner requests owned subdomain → OK
- Key owner requests unowned subdomain → treated as random
- Someone else requests owned subdomain → REJECT
- Pre-register owned subdomains on server start

### 5. Claim subdomain (post-auth)

In control session message loop, add handler for `MsgClaimSubdomain`:

```go
case protocol.MsgClaimSubdomain:
    var msg protocol.ClaimSubdomainMsg
    json.Unmarshal(raw, &msg)
    // validate + add to key's subdomains
    s.keyStore.AddSubdomain(token, msg.Subdomain)
    // respond with updated subdomain list
```

## Files to Create
- None (all changes in existing files)

## Files to Modify
- `internal/protocol/message.go` — new message types
- `internal/server/server.go` — register handler, claim handler
- `internal/server/key_store.go` — `AddSubdomain`, `FindSubdomainOwner`
- `internal/server/tunnel_manager.go` — permanent reservation for owned subdomains
- `internal/config/config.go` — `AllowRegistration` field

## Success Criteria
- [ ] Unauthenticated user can register and get key + subdomain
- [ ] Authenticated user can claim additional subdomains
- [ ] Owned subdomains permanently reserved
- [ ] Registration can be disabled via config
- [ ] Subdomain conflicts properly rejected
