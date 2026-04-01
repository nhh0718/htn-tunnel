# Client Distribution Plan

## Goal
Make `htn-tunnel` client installable in 1 command on any platform.

## Status Overview

| Phase | Description | Status | Effort |
|-------|-------------|--------|--------|
| 1 | GitHub Releases + GoReleaser | Not started | 3-5h |
| 2 | npm package (optionalDeps + shim) | Not started | 6-8h |
| 3 | go install (passive) | Not started | 1h |

## Target UX

```bash
# npm (primary - works everywhere)
npm i -g htn-tunnel
htn-tunnel http 3000

# npx (no install needed)
npx htn-tunnel http 3000

# Go developers
go install github.com/htn-sys/htn-tunnel/cmd/client@latest

# Manual (GitHub Releases)
# Download from https://github.com/htn-sys/htn-tunnel/releases
```

## Architecture Decision

**npm with optionalDependencies** (esbuild pattern):
- Main package `htn-tunnel` → JS shim that detects platform → loads binary from platform-specific package
- Platform packages `@htn-tunnel/darwin-arm64`, `@htn-tunnel/linux-x64`, etc → contains prebuilt Go binary
- postinstall fallback downloads from GitHub Releases if optionalDeps disabled

**Why not Homebrew/Scoop first?** User base is primarily web developers (Next.js, React). npm is already installed. One channel covers all platforms.

## Phases

- [Phase 1: GoReleaser + GitHub Releases](./phase-01-goreleaser.md)
- [Phase 2: npm package distribution](./phase-02-npm-package.md)
- [Phase 3: go install support](./phase-03-go-install.md)

## Key Decisions
- npm scope: `@htn-tunnel/` (org) for platform packages
- 6 platform variants: darwin-arm64, darwin-x64, linux-arm64, linux-x64, win32-arm64, win32-x64
- GoReleaser automates binary builds + GitHub Releases
- CI/CD: GitHub Actions on tag push (`v*`)

## Research
- [Client Distribution Research](../reports/researcher-260401-0920-client-distribution.md)
