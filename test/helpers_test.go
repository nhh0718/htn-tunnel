// Package test provides end-to-end integration tests for htn-tunnel.
// Tests use DevMode (self-signed cert) and random ports — no real VPS required.
package test

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/nhh0718/htn-tunnel/internal/client"
	"github.com/nhh0718/htn-tunnel/internal/config"
	"github.com/nhh0718/htn-tunnel/internal/server"
)

// startTestServer starts a Server with DevMode enabled on a random port.
// Returns the server, its config, and a cancel func.
func startTestServer(t *testing.T) (*server.Server, *config.ServerConfig, context.CancelFunc) {
	t.Helper()

	// Use OS-assigned port for the control listener.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("find free port: %v", err)
	}
	addr := ln.Addr().String()
	ln.Close()

	cfg := &config.ServerConfig{
		ListenAddr:         addr,
		Domain:             "localhost",
		Tokens:             []string{"test-token-1", "test-token-2"},
		MaxTunnelsPerToken: 5,
		RateLimit:          1000,
		GlobalRateLimit:    10000,
		TCPPortRange:       [2]int{20000, 29999},
		DevMode:            true,
		DashboardEnabled:   false,
	}

	srv, err := server.NewServer(cfg, "", "test")
	if err != nil {
		t.Fatalf("new server: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		if err := srv.Start(ctx); err != nil {
			// Expected on cancel.
		}
	}()

	// Wait for server to be ready.
	if !waitForPort(addr, 3*time.Second) {
		cancel()
		t.Fatalf("server did not start within 3s at %s", addr)
	}

	return srv, cfg, cancel
}

// startTestClient connects a client to the server at serverAddr with the given token.
func startTestClient(t *testing.T, serverAddr, token string) *client.Client {
	t.Helper()
	cfg := &config.ClientConfig{ServerAddr: serverAddr, Token: token}
	c := client.NewClient(cfg)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := c.Connect(ctx); err != nil {
		t.Fatalf("client connect: %v", err)
	}
	return c
}

// startLocalHTTPServer starts a simple HTTP server on a random port that
// echoes "hello from local:<port>" for every request.
func startLocalHTTPServer(t *testing.T) (port int, stop func()) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	port = ln.Addr().(*net.TCPAddr).Port

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "hello from local:%d path:%s", port, r.URL.Path)
	})
	mux.HandleFunc("/headers", func(w http.ResponseWriter, r *http.Request) {
		for k, vs := range r.Header {
			for _, v := range vs {
				fmt.Fprintf(w, "%s: %s\n", k, v)
			}
		}
	})

	srv := &http.Server{Handler: mux}
	go srv.Serve(ln) //nolint:errcheck
	return port, func() { srv.Close() }
}

// startLocalTCPServer starts a TCP echo server on a random port.
func startLocalTCPServer(t *testing.T) (port int, stop func()) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	port = ln.Addr().(*net.TCPAddr).Port

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				io.Copy(c, c) //nolint:errcheck // echo server
			}(conn)
		}
	}()

	_ = ctx
	return port, func() {
		cancel()
		ln.Close()
	}
}

// waitForPort polls addr until a TCP connection succeeds or timeout elapses.
func waitForPort(addr string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 100*time.Millisecond)
		if err == nil {
			conn.Close()
			return true
		}
		time.Sleep(50 * time.Millisecond)
	}
	return false
}
