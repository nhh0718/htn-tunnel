# Phase 1: CLI đẹp + Live Request Log

## Priority: HIGH | Effort: 4-5h

## Hiện tại

```
htn-tunnel vdev
  Status:    connecting...
  Server:    33.id.vn:4443
  Ctrl+C to disconnect

  Tunnel:    https://myapp.33.id.vn → localhost:3000
  Status:    connected
```

Không có thông tin request, không có traffic stats, không có uptime.

## Mục tiêu

```
╭──────────────────────────────────────────────────╮
│  htn-tunnel v0.1.2                               │
│                                                  │
│  Tunnel:   https://myapp.33.id.vn                │
│  Forward:  → localhost:3000                      │
│  Server:   33.id.vn:4443                         │
│  Status:   ● connected                           │
│  Uptime:   2h 15m | Requests: 142                │
│  Traffic:  ↓ 1.2 MB  ↑ 45 KB                    │
╰──────────────────────────────────────────────────╯

  12:34:05  GET   /api/users       200  45ms   1.2KB
  12:34:06  GET   /static/app.js   200  12ms   89KB
  12:34:07  POST  /api/login       401  89ms   0.3KB
  12:34:08  WS    /_next/webpack   101  ↔ open
  12:34:10  GET   /favicon.ico     404  2ms    0B
```

## Thiết kế

### 1. Request log protocol

Server gửi request info qua **control stream** (yamux stream 0) về client sau mỗi request.

Thêm protocol message:

```go
MsgRequestLog MsgType = 0x10

type RequestLogMsg struct {
    Method   string `json:"method"`
    Path     string `json:"path"`
    Status   int    `json:"status"`
    Duration int    `json:"duration_ms"`
    Size     int64  `json:"size"`
}
```

**Flow:**
1. Server proxy request → nhận response
2. Server gửi `MsgRequestLog` về client qua control stream
3. Client nhận → in ra terminal

### 2. Box drawing (client)

Tạo `internal/client/display.go`:

```go
func printBox(tunnel, forward, server, status string) {
    // Box chars: ╭ ─ ╮ │ ╰ ╯
    // Auto-width theo nội dung
    // Color: green=connected, yellow=connecting, red=disconnected
}

func printRequestLog(log RequestLogMsg) {
    // Format: HH:MM:SS  METHOD  PATH  STATUS  DURATION  SIZE
    // Color: 2xx=green, 3xx=cyan, 4xx=yellow, 5xx=red
}
```

### 3. Color output

Dùng ANSI escape codes trực tiếp (không thêm dependency):

```go
const (
    colorReset  = "\033[0m"
    colorGreen  = "\033[32m"
    colorYellow = "\033[33m"
    colorRed    = "\033[31m"
    colorCyan   = "\033[36m"
    colorGray   = "\033[90m"
)
```

Windows: Go 1.20+ hỗ trợ ANSI trên Windows Terminal mặc định.

### 4. Uptime + traffic counter

Client tự track:
- `startTime` → tính uptime
- `requestCount` → đếm request log nhận được
- `bytesIn`, `bytesOut` → cộng dồn từ request log

Cập nhật box header mỗi 5s hoặc khi nhận request log.

## Implementation Steps

### Server side

1. **`internal/protocol/message.go`** — thêm `MsgRequestLog`, `RequestLogMsg`

2. **`internal/server/http_proxy.go`** — sau khi proxy response, gửi request log:
   ```go
   // In serveRequest(), after proxy.ServeHTTP():
   s.sendRequestLog(ts, r.Method, r.URL.Path, statusCode, duration, responseSize)
   ```

   Cần wrap `ResponseWriter` để capture status code + size.

3. **`internal/server/server.go`** — thêm method gửi log qua control stream:
   ```go
   func (s *Server) sendRequestLog(ts *TunnelSession, ...) {
       // Gửi qua control stream encoder của session
   }
   ```

   **Lưu ý:** Control stream encoder thuộc session, cần store reference.

### Client side

4. **`internal/client/display.go`** (new) — box drawing + request log formatting

5. **`internal/client/reconnect.go`** — thay `fmt.Printf` bằng display functions:
   - `printBox()` khi connected
   - `printRequestLog()` khi nhận `MsgRequestLog`

6. **`internal/client/reconnect.go`** — xử lý `MsgRequestLog` trong heartbeat/serve loop:
   ```go
   // Trong runWithHeartbeat, thêm goroutine đọc control messages
   case protocol.MsgRequestLog:
       var log protocol.RequestLogMsg
       json.Unmarshal(raw, &log)
       display.PrintRequestLog(log)
   ```

## Files cần tạo
- `internal/client/display.go`

## Files cần sửa
- `internal/protocol/message.go` — thêm `MsgRequestLog`
- `internal/server/http_proxy.go` — capture + gửi request log
- `internal/server/server.go` — store control stream encoder per session
- `internal/client/reconnect.go` — nhận + hiển thị request log
- `internal/server/tunnel_manager.go` — thêm control encoder vào TunnelSession

## Rủi ro
- Control stream encoder chia sẻ giữa heartbeat + request log → cần mutex hoặc channel
- Request log nhiều quá → flood terminal → cần rate limit hoặc option `--quiet`
