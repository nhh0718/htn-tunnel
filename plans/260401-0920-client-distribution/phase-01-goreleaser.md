# Phase 1: GoReleaser + GitHub Releases

## Priority: HIGH | Status: Not started | Effort: 3-5h

## Overview
Foundation for all distribution channels. GoReleaser builds binaries for 6 platforms, creates GitHub Releases with checksums.

## Requirements
- Build binaries for: darwin/amd64, darwin/arm64, linux/amd64, linux/arm64, windows/amd64, windows/arm64
- Auto-generate changelog from git commits
- SHA256 checksums for all artifacts
- GitHub Actions CI/CD on tag push

## Implementation Steps

### 1. Rename client entrypoint
Current: `cmd/client/main.go` → binary name defaults to `client`
Need: users should get `htn-tunnel` binary name

```bash
# Option A: rename directory
mv cmd/client cmd/htn-tunnel

# Option B: set binary name in GoReleaser (preferred, no code change)
```

### 2. Create `.goreleaser.yaml`

```yaml
version: 2

project_name: htn-tunnel

builds:
  - id: client
    main: ./cmd/client
    binary: htn-tunnel
    env:
      - CGO_ENABLED=0
    goos:
      - darwin
      - linux
      - windows
    goarch:
      - amd64
      - arm64
    ldflags:
      - -s -w -X main.version={{.Version}}

  - id: server
    main: ./cmd/server
    binary: htn-server
    env:
      - CGO_ENABLED=0
    goos:
      - linux
    goarch:
      - amd64
      - arm64
    ldflags:
      - -s -w -X main.version={{.Version}}

archives:
  - id: client
    builds: [client]
    format_overrides:
      - goos: windows
        format: zip
    name_template: "htn-tunnel_{{ .Version }}_{{ .Os }}_{{ .Arch }}"

  - id: server
    builds: [server]
    name_template: "htn-server_{{ .Version }}_{{ .Os }}_{{ .Arch }}"

checksum:
  name_template: "checksums.txt"

changelog:
  sort: asc
  filters:
    exclude:
      - "^docs:"
      - "^test:"
      - "^chore:"

release:
  github:
    owner: htn-sys
    name: htn-tunnel
```

### 3. Create GitHub Actions workflow

```yaml
# .github/workflows/release.yml
name: Release

on:
  push:
    tags: ["v*"]

permissions:
  contents: write

jobs:
  release:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0

      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod

      - uses: goreleaser/goreleaser-action@v6
        with:
          version: latest
          args: release --clean
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

### 4. Test locally

```bash
# Install goreleaser
go install github.com/goreleaser/goreleaser/v2@latest

# Dry run (no publish)
goreleaser release --snapshot --clean

# Check output
ls dist/
```

### 5. Tag and release

```bash
git tag v0.1.0
git push origin v0.1.0
# GitHub Actions runs goreleaser automatically
```

## Success Criteria
- [ ] `goreleaser release --snapshot` builds 8 artifacts (6 client + 2 server)
- [ ] GitHub Release created with all binaries + checksums
- [ ] Downloaded binary runs on macOS, Linux, Windows

## Files to Create
- `.goreleaser.yaml`
- `.github/workflows/release.yml`

## Files to Modify
- None (GoReleaser handles binary naming)
