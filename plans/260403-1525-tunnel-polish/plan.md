# Tunnel Polish — CLI + Subdomain Validation + Dashboard

## 3 hướng chính

| # | Hướng | Mô tả | Effort |
|---|-------|-------|--------|
| 1 | [CLI đẹp + request log](./phase-01-cli-beautify.md) | Live request log, colorful output, traffic stats | 4-5h |
| 2 | [Subdomain validation](./phase-02-subdomain-validation.md) | Validate owned subdomains, interactive picker | 3-4h |
| 3 | [Dashboard polish](./phase-03-dashboard-polish.md) | Tiếng Việt, domain hiển thị, fix copy, reactive stats | 4-5h |

**Tổng: ~11-14h**

## Phase 1: CLI đẹp + Request log

**Hiện tại:**
```
htn-tunnel vdev
  Tunnel:    https://myapp.33.id.vn → localhost:3000
  Status:    connected
```

**Mục tiêu:**
```
╭─────────────────────────────────────────────╮
│  htn-tunnel v0.1.2                          │
│                                             │
│  Tunnel:  https://myapp.33.id.vn            │
│  Forward: → localhost:3000                  │
│  Status:  ● connected                       │
│  Uptime:  2h 15m                            │
╰─────────────────────────────────────────────╯

  Requests:
  12:34:05  GET  /api/users       200  45ms
  12:34:06  GET  /static/app.js   200  12ms
  12:34:07  POST /api/login       401  89ms
  12:34:08  WS   /ws/chat         101  ↔ open
```

## Phase 2: Subdomain validation

**Cú pháp mới: `port:subdomain`** (bỏ `--subdomain` flag)

```bash
# Chỉ port → interactive picker
$ htn-tunnel http 3000
  Chọn subdomain:
  [1] myapp.33.id.vn
  [2] dev.33.id.vn
  [3] Subdomain ngẫu nhiên
  > 1
  ● connected: https://myapp.33.id.vn → localhost:3000

# port:subdomain đúng (owned) → chạy luôn
$ htn-tunnel http 3000:myapp
  ● connected: https://myapp.33.id.vn → localhost:3000

# port:subdomain sai (không owned) → báo lỗi + gợi ý
$ htn-tunnel http 3000:other
  ✗ Subdomain "other" không thuộc tài khoản của bạn.
  Subdomain của bạn: myapp, dev
  Dùng subdomain mặc định? [myapp/random/cancel]:
```

## Phase 3: Dashboard polish

**Bugs cần fix:**
- Nút Copy token không hoạt động (CSP hoặc logic)
- Domain suffix hiện `.example.com` thay vì domain thật
- UI text bằng tiếng Anh → cần tiếng Việt

**Cải tiến:**
- Reactive traffic stats (bytes in/out, request count) cập nhật realtime
- Subdomain online/offline status chính xác
- Form đăng ký hiện domain thật (e.g. `.33.id.vn`)
- Toàn bộ text tiếng Việt
- Mobile responsive tốt hơn
