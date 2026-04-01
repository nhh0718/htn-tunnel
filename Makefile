VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -ldflags "-s -w -X main.version=$(VERSION)"

.PHONY: build-server build-client build-all cross-compile test lint clean

build-server:
	go build $(LDFLAGS) -o dist/htn-tunnel-server ./cmd/server

build-client:
	go build $(LDFLAGS) -o dist/htn-tunnel-client ./cmd/client

build-all: build-server build-client

cross-compile:
	GOOS=linux   GOARCH=amd64 go build $(LDFLAGS) -o dist/htn-tunnel-server-linux-amd64   ./cmd/server
	GOOS=darwin  GOARCH=amd64 go build $(LDFLAGS) -o dist/htn-tunnel-server-darwin-amd64  ./cmd/server
	GOOS=darwin  GOARCH=arm64 go build $(LDFLAGS) -o dist/htn-tunnel-server-darwin-arm64  ./cmd/server
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o dist/htn-tunnel-server-windows-amd64.exe ./cmd/server
	GOOS=linux   GOARCH=amd64 go build $(LDFLAGS) -o dist/htn-tunnel-client-linux-amd64   ./cmd/client
	GOOS=darwin  GOARCH=amd64 go build $(LDFLAGS) -o dist/htn-tunnel-client-darwin-amd64  ./cmd/client
	GOOS=darwin  GOARCH=arm64 go build $(LDFLAGS) -o dist/htn-tunnel-client-darwin-arm64  ./cmd/client
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o dist/htn-tunnel-client-windows-amd64.exe ./cmd/client

test:
	go test -v -race ./...

lint:
	go vet ./...

clean:
	rm -rf dist/
