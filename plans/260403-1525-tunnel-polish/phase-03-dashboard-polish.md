# Phase 3: Dashboard Polish — Tiếng Việt, Fix Bugs, Reactive Stats

## Priority: HIGH | Effort: 4-5h

## Bugs cần fix

### Bug 1: Nút Copy token không hoạt động
**Nguyên nhân:** `navigator.clipboard.writeText()` chỉ hoạt động trên HTTPS hoặc localhost. Dashboard qua HTTP proxy có thể fail.
**Fix:** Fallback dùng `document.execCommand('copy')` với textarea ẩn:
```js
function copyKey() {
  const key = document.getElementById('reg-key').textContent;
  if (navigator.clipboard) {
    navigator.clipboard.writeText(key).then(() => showCopied()).catch(() => fallbackCopy(key));
  } else {
    fallbackCopy(key);
  }
}
function fallbackCopy(text) {
  const ta = document.createElement('textarea');
  ta.value = text;
  ta.style.position = 'fixed';
  ta.style.opacity = '0';
  document.body.appendChild(ta);
  ta.select();
  document.execCommand('copy');
  document.body.removeChild(ta);
  showCopied();
}
function showCopied() {
  // Đổi text nút thành "Đã copy!" 2 giây rồi về "Copy"
}
```

### Bug 2: Domain suffix hiện `.example.com`
**Nguyên nhân:** `index.html` hardcode `.example.com`, JS chưa update vì domain chưa fetch được khi ở trang register (chưa login).
**Fix:** Server trả domain trong response `/register`. JS cập nhật suffix khi page load bằng cách gọi API nhỏ.

Thêm endpoint public:
```
GET /_dashboard/api/info → {"domain": "33.id.vn", "registration_enabled": true}
```

### Bug 3: Text tiếng Anh → cần tiếng Việt
Toàn bộ UI text cần chuyển sang tiếng Việt.

## Cải tiến UI

### 1. Trang Landing

**Hiện tại:**
```
Expose your localhost to the internet
[Register] [Login]
```

**Mục tiêu:**
```
╭─────────────────────────────────────╮
│        🚀 htn-tunnel               │
│   Expose localhost ra internet      │
│   Nhanh • Bảo mật • Miễn phí      │
│                                     │
│   [Đăng ký]    [Đăng nhập]        │
╰─────────────────────────────────────╯
```

### 2. Trang Đăng ký

**Text tiếng Việt:**
- "Create your account" → "Tạo tài khoản"
- "Name" → "Tên hiển thị"
- "Subdomain" → "Subdomain của bạn"
- Suffix: `.33.id.vn` (từ API, không hardcode)
- "Create Account" → "Tạo tài khoản"
- "Already have a key?" → "Đã có API key?"

**Copy key success:**
```
🎉 Tạo tài khoản thành công!

API Key của bạn (lưu lại cẩn thận!):
┌─────────────────────────────────────┐
│ htk_a1b2c3d4e5f6...                │  [Copy]
└─────────────────────────────────────┘

Bắt đầu nhanh:
  npm i -g htn-tunnel
  htn-tunnel auth htk_a1b2... --server 33.id.vn:4443
  htn-tunnel http 3000 --subdomain myapp

[Vào Dashboard →]
```

### 3. Trang Đăng nhập

**Text:**
- "Login" → "Đăng nhập"
- "API Key" → "Nhập API Key"
- placeholder: `htk_...`
- "Need an account?" → "Chưa có tài khoản?"

### 4. Trang Panel (sau đăng nhập)

**Subdomain list — reactive với online/offline:**
```
┌──────────────────────────────────────────────┐
│ Subdomain của bạn                            │
├──────────────────────────────────────────────┤
│ 🟢 myapp.33.id.vn    → localhost:3000  [Xóa]│
│ ⚪ dev.33.id.vn      offline           [Xóa]│
├──────────────────────────────────────────────┤
│ [____________] [+ Thêm subdomain]            │
└──────────────────────────────────────────────┘
```

**Traffic stats (reactive, cập nhật mỗi 3s):**
```
┌──────────────┬──────────────┬──────────────┐
│  Requests    │  Traffic ↓   │  Traffic ↑   │
│  142         │  1.2 MB      │  45 KB       │
└──────────────┴──────────────┴──────────────┘
```

**Active tunnels — chi tiết hơn:**
```
┌──────────────────────────────────────────────┐
│ Tunnels đang hoạt động                       │
├──────────────────────────────────────────────┤
│ myapp.33.id.vn → localhost:3000              │
│ Uptime: 2h 15m | Requests: 142              │
│ ↓ 1.2 MB  ↑ 45 KB                          │
└──────────────────────────────────────────────┘
```

**Quick start — tiếng Việt:**
```
Hướng dẫn nhanh:
  htn-tunnel auth htk_... --server 33.id.vn:4443
  htn-tunnel http 3000 --subdomain myapp
```

## Implementation Steps

### 1. Thêm API public info endpoint

**`internal/dashboard/handler.go`:**
```go
h.mux.HandleFunc("GET /_dashboard/api/info", h.handlePublicInfo)

func (h *Handler) handlePublicInfo(w http.ResponseWriter, r *http.Request) {
    writeJSON(w, map[string]any{
        "domain": h.domain,
        "registration_enabled": true,  // from config
    })
}
```

### 2. Rewrite user dashboard HTML

**`internal/dashboard/static/user/index.html`:**
- Toàn bộ text sang tiếng Việt
- Form đăng ký: domain suffix dynamic
- Stats grid trong panel
- Responsive mobile

### 3. Rewrite user dashboard JS

**`internal/dashboard/static/user/app.js`:**
- Gọi `/api/info` khi load → lấy domain → cập nhật suffix
- Fix `copyKey()` với fallback
- Reactive stats: poll `/api/tunnels` mỗi 3s, cập nhật counters
- Subdomain online/offline: match tunnel list với subdomain list
- Animate số thay đổi (optional)

### 4. Update CSS

**`internal/dashboard/static/user/style.css`:**
- Stats grid cards
- Better tunnel cards
- Mobile breakpoints
- Smooth transitions

## Files cần sửa
- `internal/dashboard/handler.go` — thêm `/api/info` endpoint
- `internal/dashboard/static/user/index.html` — toàn bộ text tiếng Việt + layout mới
- `internal/dashboard/static/user/app.js` — fix bugs + reactive stats
- `internal/dashboard/static/user/style.css` — UI polish

## Text cần dịch (user dashboard)

| Tiếng Anh | Tiếng Việt |
|-----------|------------|
| Expose your localhost to the internet | Expose localhost ra internet |
| Register | Đăng ký |
| Login | Đăng nhập |
| Create your account | Tạo tài khoản |
| Name | Tên hiển thị |
| Subdomain | Subdomain |
| Create Account | Tạo tài khoản |
| Account created! | Tạo tài khoản thành công! |
| Your API Key (save this!) | API Key của bạn (lưu lại cẩn thận!) |
| Copy | Sao chép |
| Quick start | Bắt đầu nhanh |
| Go to Dashboard | Vào Dashboard |
| Already have a key? Login | Đã có API key? Đăng nhập |
| API Key | API Key |
| Need an account? Register | Chưa có tài khoản? Đăng ký |
| Your Subdomains | Subdomain của bạn |
| Active Tunnels | Tunnels đang hoạt động |
| No active tunnels | Chưa có tunnel nào |
| Add | Thêm |
| Remove | Xóa |
| Logout | Đăng xuất |
| online | trực tuyến |
| offline | ngoại tuyến |

## Thứ tự thực hiện
1. Thêm `/api/info` endpoint (5 phút)
2. Fix copyKey bug (10 phút)
3. Rewrite HTML tiếng Việt (30 phút)
4. Rewrite JS — domain fetch, reactive stats (1-2h)
5. Update CSS — stats grid, cards, mobile (1h)
6. Test trên browser + mobile (30 phút)
