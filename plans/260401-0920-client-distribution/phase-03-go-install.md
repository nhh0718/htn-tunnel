# Phase 3: go install Support

## Priority: LOW | Status: Not started | Effort: 1h

## Overview
Enable `go install github.com/htn-sys/htn-tunnel/cmd/client@latest` for Go developers. Mostly passive — just need correct module structure and git tags.

## Implementation Steps

### 1. Verify module path works

```bash
# Test from clean env
GOBIN=/tmp/gotest go install github.com/htn-sys/htn-tunnel/cmd/client@latest
/tmp/gotest/client --version
```

Binary name will be `client` (from directory name). Two options:

**Option A: Rename directory** (recommended)
```bash
# Rename cmd/client → cmd/htn-tunnel
git mv cmd/client cmd/htn-tunnel
# Update all imports/references
# Then: go install github.com/htn-sys/htn-tunnel/cmd/htn-tunnel@latest
# Binary name: htn-tunnel ✓
```

**Option B: Keep as-is, document alias**
```bash
go install github.com/htn-sys/htn-tunnel/cmd/client@latest
# Binary: ~/go/bin/client
# User creates alias: alias htn-tunnel=client
```

### 2. Ensure git tags follow semver

```bash
git tag v0.1.0
git push origin v0.1.0
# go install uses git tags for versioning
```

### 3. Add to README

```markdown
## Install

### Go developers
go install github.com/htn-sys/htn-tunnel/cmd/htn-tunnel@latest
```

## Success Criteria
- [ ] `go install ...@latest` produces working binary
- [ ] Binary name is `htn-tunnel` (not `client`)
- [ ] Version flag shows correct version

## Files to Modify
- `cmd/client/` → rename to `cmd/htn-tunnel/` (if Option A)
- `README.md` — add install instructions

## Impact on Other Phases
- Phase 1 (GoReleaser): update `main: ./cmd/htn-tunnel` in `.goreleaser.yaml`
- Phase 2 (npm): update `go build ./cmd/htn-tunnel` in publish script

## Note
If renaming `cmd/client` → `cmd/htn-tunnel`, also keep `cmd/server` as-is (server binary name `htn-server` is set by GoReleaser).
