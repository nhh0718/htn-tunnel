# Phase 5: Client CLI Update

## Priority: LOW | Status: Not started | Effort: 1-2h

## Overview
Simplify client CLI. User registers via web dashboard (Phase 3), CLI chỉ cần auth + use.

## Updated Commands

### `htn-tunnel auth <key>`
Save key to config (same as before).
```bash
htn-tunnel auth htk_a1b2c3... --server 33.id.vn:4443
# → Saved to ~/.htn-tunnel/config.yaml
```

### `htn-tunnel http <port>`
```bash
htn-tunnel http 3000 --subdomain hoang
# → https://hoang.33.id.vn → localhost:3000
```

### `htn-tunnel status` (new)
Show current key info + subdomains by calling server API.
```bash
htn-tunnel status
# Key:        htk_a1b2...d4e5
# Name:       Hoang
# Subdomains: hoang.33.id.vn, myapp.33.id.vn
# Server:     33.id.vn:4443
```

## Implementation Steps

### 1. Add `status` command

**`cmd/htn-tunnel/main.go`:**
```go
func statusCmd() *cobra.Command {
    return &cobra.Command{
        Use:   "status",
        Short: "Show your account info and subdomains",
        RunE: func(cmd *cobra.Command, args []string) error {
            cfg, err := loadClientCfg()
            if err != nil { return err }

            c := client.NewClient(cfg)
            ctx := context.Background()
            if err := c.Connect(ctx); err != nil { return err }
            defer c.Close()

            info, err := c.GetAccountInfo()
            if err != nil { return err }

            fmt.Printf("Key:        %s...%s\n", cfg.Token[:8], cfg.Token[len(cfg.Token)-4:])
            fmt.Printf("Name:       %s\n", info.Name)
            fmt.Printf("Subdomains: %s\n", strings.Join(info.Subdomains, ", "))
            fmt.Printf("Server:     %s\n", cfg.ServerAddr)
            return nil
        },
    }
}
```

### 2. Register commands

```go
root.AddCommand(httpCmd(), tcpCmd(), authCmd(), statusCmd())
```

### 3. Remove `register` and `subdomain` CLI commands
User manages these via web dashboard instead. Cleaner CLI, less protocol messages.

### 4. Add protocol message for account info

**`internal/protocol/message.go`:**
```go
MsgAccountInfo     MsgType = 14
MsgAccountInfoResp MsgType = 15

type AccountInfoMsg struct{}
type AccountInfoRespMsg struct {
    Name       string   `json:"name"`
    Subdomains []string `json:"subdomains"`
    MaxTunnels int      `json:"max_tunnels"`
}
```

## Files to Modify
- `cmd/htn-tunnel/main.go` — add `status` command
- `internal/client/tunnel.go` — add `GetAccountInfo` method
- `internal/protocol/message.go` — add account info messages
- `internal/server/server.go` — handle account info request

## Success Criteria
- [ ] `htn-tunnel auth` + `htn-tunnel http` work as before
- [ ] `htn-tunnel status` shows key info + subdomains
- [ ] No register/subdomain CLI commands (web dashboard handles this)
