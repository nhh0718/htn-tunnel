## Stage 1: Build
FROM golang:1.22-alpine AS builder

WORKDIR /app

# Download dependencies first (cache layer)
COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags "-s -w" -o /server ./cmd/server
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags "-s -w" -o /client ./cmd/client

## Stage 2: Runtime
FROM alpine:3.19

# ca-certificates required for Let's Encrypt HTTPS validation
RUN apk add --no-cache ca-certificates

COPY --from=builder /server /usr/local/bin/htn-tunnel-server
COPY --from=builder /client /usr/local/bin/htn-tunnel-client

# Control plane, HTTPS, HTTP
EXPOSE 4443 443 80

# Dashboard
EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/htn-tunnel-server"]
