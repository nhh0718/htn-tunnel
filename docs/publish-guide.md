# htn-tunnel — Publish Guide

## Overview

Khi push tag `v*`, GitHub Actions tự động:
1. GoReleaser build binaries → GitHub Releases
2. Build + publish npm packages (client only)

User cài: `npm i -g htn-tunnel`

---

## Setup (1 lần duy nhất)

### 1. Tạo npm org

1. Vào https://www.npmjs.com → Sign up / Login
2. Click avatar → **Add Organization** → tên: `htn-tunnel`
3. Plan: **Unlimited public packages (free)**

### 2. Tạo npm access token

1. npmjs.com → avatar → **Access Tokens**
2. **Generate New Token** → **Automation** (cho CI/CD)
3. Copy token (bắt đầu bằng `npm_...`)

### 3. Add secrets vào GitHub repo

Vào https://github.com/nhh0718/htn-tunnel/settings/secrets/actions:

| Secret name | Value |
|-------------|-------|
| `NPM_TOKEN` | npm automation token từ bước 2 |

> `GITHUB_TOKEN` tự có sẵn, không cần tạo.

### 4. Update repo URL trong configs

Các file cần đúng repo URL `nhh0718/htn-tunnel`:

**`.goreleaser.yaml`** — không cần sửa (GoReleaser tự detect từ git remote).

**`npm/htn-tunnel/package.json`:**
```json
"repository": {
  "type": "git",
  "url": "https://github.com/nhh0718/htn-tunnel"
}
```

**`scripts/publish-npm.sh`:**
```bash
REPO="nhh0718/htn-tunnel"
```

### 5. Update Go module path (nếu cần)

Nếu module path trong `go.mod` khác repo URL:
```
module github.com/nhh0718/htn-tunnel
```

Cần update tất cả import paths trong code. Nếu `go.mod` đã đúng → bỏ qua bước này.

---

## Publish Release

### Bước 1: Đảm bảo code sạch

```bash
go build ./cmd/htn-tunnel && go build ./cmd/server
go test ./...
```

### Bước 2: Tag version

```bash
git add -A
git commit -m "feat: v0.1.0 release"
git tag v0.1.0
git push origin main
git push origin v0.1.0
```

### Bước 3: GitHub Actions tự chạy

Vào https://github.com/nhh0718/htn-tunnel/actions → xem workflow **Release**.

Workflow thực hiện:

**Job 1: `goreleaser`**
- Build 6 client binaries (darwin/linux/windows × amd64/arm64)
- Build 2 server binaries (linux × amd64/arm64)
- Upload tất cả lên GitHub Releases kèm checksums

**Job 2: `publish-npm`** (sau Job 1)
- Build 6 platform-specific npm packages
- Publish `@htn-tunnel/darwin-arm64`, `@htn-tunnel/darwin-x64`, etc.
- Publish main package `htn-tunnel`

### Bước 4: Verify

```bash
# Check GitHub Releases
# → https://github.com/nhh0718/htn-tunnel/releases/tag/v0.1.0

# Check npm
npm view htn-tunnel
npm view @htn-tunnel/darwin-arm64

# Test install
npm i -g htn-tunnel
htn-tunnel --version
```

---

## Publish thủ công (không qua CI)

Nếu GitHub Actions fail hoặc muốn test local:

### npm packages

```bash
# Cần npm login trước
npm login

# Build + publish tất cả
chmod +x scripts/publish-npm.sh
./scripts/publish-npm.sh 0.1.0
```

### GitHub Releases (GoReleaser local)

```bash
# Cài goreleaser
go install github.com/goreleaser/goreleaser/v2@latest

# Dry run (không publish)
goreleaser release --snapshot --clean
ls dist/

# Publish thật (cần GITHUB_TOKEN)
export GITHUB_TOKEN=ghp_your_token
goreleaser release --clean
```

---

## Version mới

Mỗi lần release version mới:

```bash
# 1. Commit changes
git add -A
git commit -m "feat: description of changes"

# 2. Tag
git tag v0.2.0

# 3. Push
git push origin main
git push origin v0.2.0

# CI tự publish. Done.
```

---

## Cấu trúc npm packages

```
npm registry:
├── htn-tunnel@0.1.0                   # Main package (JS shim)
│   ├── bin/htn-tunnel.js              # Detect platform → exec binary
│   ├── lib/install.js                 # Fallback download from GitHub
│   └── optionalDependencies:
│       ├── @htn-tunnel/darwin-arm64   # macOS Apple Silicon
│       ├── @htn-tunnel/darwin-x64     # macOS Intel
│       ├── @htn-tunnel/linux-arm64    # Linux ARM
│       ├── @htn-tunnel/linux-x64      # Linux x64
│       ├── @htn-tunnel/win32-arm64    # Windows ARM
│       └── @htn-tunnel/win32-x64     # Windows x64
```

Install flow:
1. `npm i -g htn-tunnel` → npm auto-installs correct platform package
2. `htn-tunnel http 3000` → JS shim detects OS → loads binary → exec
3. Nếu optionalDeps bị disable → postinstall downloads từ GitHub Releases

---

## Checklist trước khi publish lần đầu

- [ ] npm org `@htn-tunnel` đã tạo trên npmjs.com
- [ ] npm automation token đã tạo
- [ ] GitHub secret `NPM_TOKEN` đã add
- [ ] `go.mod` module path match repo URL
- [ ] `go build ./cmd/htn-tunnel` + `go build ./cmd/server` pass
- [ ] `go test ./...` pass
- [ ] `.goreleaser.yaml` có trong repo
- [ ] `.github/workflows/release.yml` có trong repo
- [ ] `npm/htn-tunnel/` directory có trong repo
- [ ] `scripts/publish-npm.sh` có trong repo và executable

---

## Troubleshooting

### npm publish 403 Forbidden
- Check npm token còn valid: `npm whoami`
- Check org membership: `npm org ls htn-tunnel`
- Check package name chưa bị taken: `npm view htn-tunnel`

### GoReleaser fails
- Check `GITHUB_TOKEN` permission: needs `contents: write`
- Check tag format: must be `v*` (e.g., `v0.1.0`)
- Local test: `goreleaser release --snapshot --clean`

### npm install fails on user machine
- postinstall fallback cần GitHub Releases tồn tại → publish GoReleaser trước npm
- Check network: user phải access được github.com và registry.npmjs.org

### Binary not found after npm install
- `npm root -g` → check binary symlink exists
- `which htn-tunnel` hoặc `where htn-tunnel` (Windows)
- Reinstall: `npm i -g htn-tunnel --force`
