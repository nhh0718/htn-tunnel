# Phase 2: Subdomain Validation + Interactive Picker

## Priority: HIGH | Effort: 3-4h

## Cú pháp mới

```
htn-tunnel http <port>:<subdomain>
```

Ví dụ:
```bash
htn-tunnel http 3000:hoang     # port 3000, subdomain "hoang"
htn-tunnel http 3000           # port 3000, không chỉ định subdomain
```

**Bỏ flag `--subdomain`**, gộp vào argument duy nhất `port:subdomain`.

## Hiện tại

```bash
htn-tunnel http 3000                        # → random subdomain
htn-tunnel http 3000 --subdomain anything   # → flag riêng
```

## Mục tiêu

### Case 1: Chỉ port, không subdomain → interactive picker

```
$ htn-tunnel http 3000

  Chọn subdomain:

  [1] myapp.33.id.vn
  [2] dev.33.id.vn
  [3] Subdomain ngẫu nhiên

  Nhập số (1-3): 1

╭──────────────────────────────────────────╮
│  Tunnel:  https://myapp.33.id.vn        │
│  Forward: → localhost:3000              │
│  Status:  ● connected                   │
╰──────────────────────────────────────────╯
```

### Case 2: port:subdomain đúng (owned) → chạy luôn

```
$ htn-tunnel http 3000:myapp
╭──────────────────────────────────────────╮
│  Tunnel:  https://myapp.33.id.vn        │
│  ...                                    │
╰──────────────────────────────────────────╯
```

### Case 3: port:subdomain sai (không owned) → báo lỗi + gợi ý

```
$ htn-tunnel http 3000:other

  ✗ Subdomain "other" không thuộc tài khoản của bạn.

  Subdomain của bạn:
    myapp.33.id.vn
    dev.33.id.vn

  Bạn muốn:
  [1] Dùng myapp (mặc định)
  [2] Dùng dev
  [3] Subdomain ngẫu nhiên
  [4] Hủy

  Nhập số (1-4):
```

## Thiết kế

### 1. Lấy danh sách subdomain owned

Client kết nối → auth → gửi `MsgAccountInfo` → nhận `AccountInfoRespMsg` chứa `Subdomains []string`.

Đã có sẵn: `client.GetAccountInfo()` trả về `AccountInfo{Subdomains}`.

### 2. Parse argument `port:subdomain`

```go
// Trong httpCmd():
func parsePortSubdomain(arg string) (int, string, error) {
    // "3000:hoang" → 3000, "hoang"
    // "3000"       → 3000, ""
    parts := strings.SplitN(arg, ":", 2)
    port, err := strconv.Atoi(parts[0])
    if err != nil || port < 1 || port > 65535 {
        return 0, "", fmt.Errorf("invalid port %q", parts[0])
    }
    subdomain := ""
    if len(parts) == 2 {
        subdomain = parts[1]
    }
    return port, subdomain, nil
}
```

### 3. Validation logic

```go
func resolveSubdomain(cfg *config.ClientConfig, requested string) (string, error) {
    c := client.NewClient(cfg)
    c.Connect(ctx)
    info, _ := c.GetAccountInfo()
    c.Close()

    owned := info.Subdomains

    if requested != "" {
        if contains(owned, requested) {
            return requested, nil // Case 2: owned → OK
        }
        return promptSubdomainFallback(owned, requested) // Case 3: not owned
    }

    if len(owned) == 0 {
        return "", nil // random
    }
    return promptSubdomainPicker(owned) // Case 1: picker
}
```

### 3. Interactive prompt

```go
func promptSubdomainPicker(owned []string, domain string) (string, error) {
    fmt.Println("\n  Chọn subdomain:\n")
    for i, sub := range owned {
        fmt.Printf("  [%d] %s.%s\n", i+1, sub, domain)
    }
    fmt.Printf("  [%d] Subdomain ngẫu nhiên\n", len(owned)+1)
    fmt.Printf("\n  Nhập số (1-%d): ", len(owned)+1)

    var choice int
    fmt.Scan(&choice)

    if choice >= 1 && choice <= len(owned) {
        return owned[choice-1], nil
    }
    return "", nil // random
}

func promptSubdomainFallback(owned []string, requested, domain string) (string, error) {
    fmt.Printf("\n  ✗ Subdomain \"%s\" không thuộc tài khoản của bạn.\n\n", requested)
    fmt.Println("  Subdomain của bạn:")
    for _, sub := range owned {
        fmt.Printf("    %s.%s\n", sub, domain)
    }
    fmt.Println()

    options := make([]string, 0, len(owned)+2)
    for i, sub := range owned {
        fmt.Printf("  [%d] Dùng %s\n", i+1, sub)
        options = append(options, sub)
    }
    fmt.Printf("  [%d] Subdomain ngẫu nhiên\n", len(owned)+1)
    fmt.Printf("  [%d] Hủy\n", len(owned)+2)
    fmt.Printf("\n  Nhập số (1-%d): ", len(owned)+2)

    var choice int
    fmt.Scan(&choice)

    if choice == len(owned)+2 {
        return "", fmt.Errorf("cancelled")
    }
    if choice >= 1 && choice <= len(owned) {
        return owned[choice-1], nil
    }
    return "", nil // random
}
```

### 4. Extract domain từ server address

```go
// "33.id.vn:4443" → "33.id.vn"
func extractDomain(serverAddr string) string {
    host, _, _ := net.SplitHostPort(serverAddr)
    return host
}
```

### 5. Cần server trả domain trong AccountInfoResp

Thêm `Domain` field vào `AccountInfoRespMsg`:
```go
type AccountInfoRespMsg struct {
    Name       string   `json:"name"`
    Subdomains []string `json:"subdomains"`
    MaxTunnels int      `json:"max_tunnels"`
    Domain     string   `json:"domain"`     // NEW
}
```

Server fill từ `cfg.Domain`.

## Implementation Steps

1. **`internal/protocol/message.go`** — thêm `Domain` vào `AccountInfoRespMsg`

2. **`internal/server/server.go`** — fill `Domain` trong `handleAccountInfo`

3. **`internal/client/tunnel.go`** — update `AccountInfo` struct với `Domain`

4. **`internal/client/subdomain_picker.go`** (new) — interactive picker + fallback prompt logic

5. **`cmd/htn-tunnel/main.go`** — trong `httpCmd()`:
   - Parse arg `port:subdomain` thay vì `port` + `--subdomain` flag
   - Connect → GetAccountInfo → resolve subdomain → pass to Reconnector
   - Bỏ `--subdomain` flag, giữ backward compatible (nếu ai dùng `--subdomain` cũ vẫn hoạt động)

### CLI usage mới:
```
htn-tunnel http <port>[:subdomain]    Tạo HTTP tunnel
  Ví dụ:
    htn-tunnel http 3000              # interactive picker
    htn-tunnel http 3000:hoang        # chạy luôn subdomain "hoang"
    htn-tunnel http 8080:dev          # port 8080, subdomain "dev"
```

## Files cần tạo
- `internal/client/subdomain_picker.go`

## Files cần sửa
- `internal/protocol/message.go` — Domain field
- `internal/server/server.go` — fill Domain
- `internal/client/tunnel.go` — AccountInfo.Domain
- `cmd/htn-tunnel/main.go` — validation flow trong httpCmd

## Edge cases
- User không có subdomain nào → skip picker, dùng random
- `3000:other` + not owned → prompt fallback
- `--subdomain` flag cũ vẫn hoạt động (backward compat), ưu tiên `port:sub` nếu cả hai có
- Server không trả domain (old version) → fallback extract từ server address
