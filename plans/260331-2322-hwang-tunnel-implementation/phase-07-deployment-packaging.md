# Phase 7: Deployment & Packaging

## Context Links
- [Plan Overview](plan.md)
- [Deployment & UX Research](../reports/researcher-260331-2319-deployment-ux-patterns.md)

## Overview
- **Priority:** P2
- **Status:** completed
- **Effort:** 2h
- **Description:** Cross-platform build targets, Docker packaging, server config via env vars, and basic README with quickstart.

## Key Insights
- Go cross-compilation is trivial: `GOOS=linux GOARCH=amd64 go build`
- Multi-stage Docker build: golang builder → alpine runtime (~20MB final image)
- Server config via env vars is 12-factor compliant, Docker-friendly
- VPS requirements: $2-5/month, 512MB RAM, public IP, domain with wildcard DNS

## Requirements

### Functional
- Makefile: build-server, build-client, build-all, test, lint, clean
- Cross-compile: linux/amd64, darwin/amd64, darwin/arm64, windows/amd64
- Dockerfile: multi-stage build for server
- docker-compose.yml: server with env config
- Server config via env vars (override YAML)
- README: project description, quickstart (server + client setup), config reference

### Non-Functional
- Server Docker image < 30MB
- Build reproducible (Go module cache, pinned versions)
- Binary names: `htn-tunnel-server`, `htn-tunnel-client`

## Architecture

### Build Matrix
```
Target              OS      Arch     Binary
htn-tunnel-server linux   amd64    dist/htn-tunnel-server-linux-amd64
htn-tunnel-server darwin  amd64    dist/htn-tunnel-server-darwin-amd64
htn-tunnel-server darwin  arm64    dist/htn-tunnel-server-darwin-arm64
htn-tunnel-server windows amd64    dist/htn-tunnel-server-windows-amd64.exe
htn-tunnel-client linux   amd64    dist/htn-tunnel-client-linux-amd64
htn-tunnel-client darwin  amd64    dist/htn-tunnel-client-darwin-amd64
htn-tunnel-client darwin  arm64    dist/htn-tunnel-client-darwin-arm64
htn-tunnel-client windows amd64    dist/htn-tunnel-client-windows-amd64.exe
```

### Docker Architecture
```
Stage 1: Build (golang:1.22-alpine)
  → go build -o /server ./cmd/server
  → go build -o /client ./cmd/client

Stage 2: Runtime (alpine:3.19)
  → COPY /server /usr/local/bin/htn-tunnel-server
  → EXPOSE 443 80 4443 10000-65535
  → ENTRYPOINT ["/usr/local/bin/htn-tunnel-server"]
```

## Related Code Files

### Files to Create
- `Makefile` — build targets
- `Dockerfile` — multi-stage server build
- `docker-compose.yml` — server deployment config
- `README.md` — project overview + quickstart
- `server.example.yaml` — example server config

### Files to Modify
- `internal/config/config.go` — env var override support

## Implementation Steps

1. **Create `Makefile`**
   ```makefile
   VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
   LDFLAGS := -ldflags "-s -w -X main.version=$(VERSION)"

   .PHONY: build-server build-client build-all test lint clean

   build-server:
       go build $(LDFLAGS) -o dist/htn-tunnel-server ./cmd/server

   build-client:
       go build $(LDFLAGS) -o dist/htn-tunnel-client ./cmd/client

   build-all: build-server build-client

   cross-compile:
       # loop over GOOS/GOARCH combinations

   test:
       go test -v -race ./...

   lint:
       go vet ./...

   clean:
       rm -rf dist/
   ```

2. **Create `Dockerfile`**
   - Stage 1: `FROM golang:1.22-alpine AS builder`
     - `WORKDIR /app`
     - `COPY go.mod go.sum ./` → `RUN go mod download`
     - `COPY . .`
     - `RUN CGO_ENABLED=0 go build -o /server ./cmd/server`
   - Stage 2: `FROM alpine:3.19`
     - `RUN apk add --no-cache ca-certificates`
     - `COPY --from=builder /server /usr/local/bin/htn-tunnel-server`
     - `EXPOSE 443 80 4443`
     - `ENTRYPOINT ["/usr/local/bin/htn-tunnel-server"]`

3. **Create `docker-compose.yml`**
   ```yaml
   services:
     server:
       build: .
       network_mode: host    # REQUIRED: avoids 55K iptables rules for TCP port range
       environment:
         - HTN_DOMAIN=tunnel.example.com
         - HTN_TOKENS=tok_abc123,tok_def456
         - HTN_LISTEN_ADDR=:4443
         - HTN_EMAIL=admin@example.com
       volumes:
         - certs:/var/lib/htn-tunnel/certs
       restart: unless-stopped
   volumes:
     certs:
   ```
   > [!RED-TEAM] Mapping 55K ports via Docker port rules OOMs a 512MB VPS. `network_mode: host` is the only viable option for TCP tunneling.

4. **Update `internal/config/config.go`** — env var overrides
   - After loading YAML: check `os.Getenv("HTN_*")` for each field
   - Env vars take precedence over YAML
   - `HTN_TOKENS` split by comma
   - `HTN_TCP_PORT_MIN`, `HTN_TCP_PORT_MAX` for port range

5. **Create `server.example.yaml`**
   ```yaml
   listen_addr: ":4443"
   domain: "tunnel.example.com"
   email: "admin@example.com"
   tokens:
     - "tok_abc123"
     - "tok_def456"
   max_tunnels_per_token: 10
   rate_limit: 100
   global_rate_limit: 1000
   tcp_port_range: [10000, 65535]
   dev_mode: false
   ```

6. **Create `README.md`**
   - Project description (self-hosted ngrok alternative)
   - Features list
   - Quick start: server setup (Docker or binary), DNS config, client install
   - CLI reference
   - Configuration reference (YAML + env vars)
   - Architecture diagram (ASCII)

7. **Add version injection to `cmd/server/main.go` and `cmd/client/main.go`**
   - `var version = "dev"` — overridden by ldflags at build time
   - `--version` flag prints version

## Todo List
- [x] Create Makefile with build/test/lint/cross-compile targets
- [x] Create Dockerfile (multi-stage)
- [x] Create docker-compose.yml
- [x] Add env var override support in config
- [x] Create server.example.yaml
- [x] Create README.md with quickstart
- [x] Add version injection via ldflags
- [x] Test: `make build-all`, `docker build`, `docker-compose up`

## Success Criteria
- `make build-all` produces server + client binaries
- `make cross-compile` produces 8 binaries (4 OS/arch × 2 binaries)
- `docker build .` produces < 30MB image
- `docker-compose up` starts server with env config
- README quickstart is followable end-to-end
- `--version` flag prints correct version

## Risk Assessment
- **Risk:** Docker port range mapping (10000-65535) may be slow or unsupported
  - **Mitigation:** Use `--network host` for TCP tunnels in Docker. Document alternative.
- **Risk:** certmagic needs port 80/443 for ACME challenge — conflicts if another service uses them
  - **Mitigation:** Document: server needs exclusive access to 80/443. Use DNS-01 challenge to avoid port requirement.

## Security Considerations
- Don't embed tokens in Dockerfile or docker-compose.yml committed to git
- Use Docker secrets or .env file (gitignored)
- Alpine image: minimal attack surface
- `ca-certificates` package needed for Let's Encrypt HTTPS validation

## Next Steps
→ Phase 8: Testing & Hardening — integration tests, load tests, security hardening
