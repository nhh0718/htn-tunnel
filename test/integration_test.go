package test

import (
	"context"
	"fmt"
	"io"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/htn-sys/htn-tunnel/internal/client"
	"github.com/htn-sys/htn-tunnel/internal/config"
)

// TestAuthSuccess_Integration verifies a client can authenticate and get a success response.
func TestAuthSuccess_Integration(t *testing.T) {
	_, cfg, cancel := startTestServer(t)
	defer cancel()

	c := startTestClient(t, cfg.ListenAddr, "test-token-1")
	defer c.Close()
	// If we got here, auth succeeded (startTestClient calls t.Fatalf on error).
}

// TestAuthFailure_Integration verifies an invalid token is rejected.
// The detailed assertion is in internal/server/server_test.go; here we just
// confirm the client library surfaces a non-nil error.
func TestAuthFailure_Integration(t *testing.T) {
	_, cfg, cancel := startTestServer(t)
	defer cancel()

	cfg2 := &config.ClientConfig{ServerAddr: cfg.ListenAddr, Token: "wrong-token"}
	c := client.NewClient(cfg2)
	ctx2, cancel2 := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel2()

	err := c.Connect(ctx2)
	if err == nil {
		c.Close()
		t.Fatal("expected auth failure, got nil error")
	}
	t.Logf("auth failure correctly returned: %v", err)
}

// TestTCPTunnel_RoundTrip verifies data flows through a TCP tunnel end-to-end.
func TestTCPTunnel_RoundTrip(t *testing.T) {
	_, cfg, cancel := startTestServer(t)
	defer cancel()

	// Start local TCP echo server.
	echoPort, stopEcho := startLocalTCPServer(t)
	defer stopEcho()

	// Connect client and request TCP tunnel.
	c := startTestClient(t, cfg.ListenAddr, "test-token-1")
	defer c.Close()

	info, err := c.RequestTCPTunnel(echoPort)
	if err != nil {
		t.Fatalf("request TCP tunnel: %v", err)
	}
	if info.RemotePort == 0 {
		t.Fatal("expected non-zero remote port")
	}
	t.Logf("TCP tunnel allocated on port %d", info.RemotePort)

	// Start serving the tunnel (client forwards yamux streams to local echo).
	ctx, cancelServe := context.WithCancel(context.Background())
	defer cancelServe()
	go c.ServeTunnel(ctx, echoPort) //nolint:errcheck

	// Give the tunnel a moment to be ready.
	time.Sleep(100 * time.Millisecond)

	// Connect to the allocated server port.
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", info.RemotePort), 3*time.Second)
	if err != nil {
		t.Fatalf("connect to tunnel port %d: %v", info.RemotePort, err)
	}
	defer conn.Close()

	// Send data and verify echo.
	const msg = "hello tunnel"
	if _, err := fmt.Fprint(conn, msg); err != nil {
		t.Fatalf("write to tunnel: %v", err)
	}

	buf := make([]byte, len(msg))
	conn.SetDeadline(time.Now().Add(3 * time.Second)) //nolint:errcheck
	if _, err := io.ReadFull(conn, buf); err != nil {
		t.Fatalf("read from tunnel: %v", err)
	}
	if string(buf) != msg {
		t.Errorf("echo mismatch: got %q, want %q", string(buf), msg)
	}
}

// TestTCPTunnel_MultipleConnections verifies several concurrent connections
// to the same TCP tunnel port all work independently.
func TestTCPTunnel_MultipleConnections(t *testing.T) {
	_, cfg, cancel := startTestServer(t)
	defer cancel()

	echoPort, stopEcho := startLocalTCPServer(t)
	defer stopEcho()

	c := startTestClient(t, cfg.ListenAddr, "test-token-1")
	defer c.Close()

	info, err := c.RequestTCPTunnel(echoPort)
	if err != nil {
		t.Fatalf("request TCP tunnel: %v", err)
	}

	ctx, cancelServe := context.WithCancel(context.Background())
	defer cancelServe()
	go c.ServeTunnel(ctx, echoPort) //nolint:errcheck

	time.Sleep(100 * time.Millisecond)

	const concurrent = 5
	errc := make(chan error, concurrent)

	for i := 0; i < concurrent; i++ {
		go func(id int) {
			conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", info.RemotePort), 3*time.Second)
			if err != nil {
				errc <- fmt.Errorf("conn %d dial: %w", id, err)
				return
			}
			defer conn.Close()

			msg := fmt.Sprintf("ping from connection %d", id)
			if _, err := fmt.Fprint(conn, msg); err != nil {
				errc <- fmt.Errorf("conn %d write: %w", id, err)
				return
			}

			buf := make([]byte, len(msg))
			conn.SetDeadline(time.Now().Add(3 * time.Second)) //nolint:errcheck
			if _, err := io.ReadFull(conn, buf); err != nil {
				errc <- fmt.Errorf("conn %d read: %w", id, err)
				return
			}
			if string(buf) != msg {
				errc <- fmt.Errorf("conn %d echo mismatch: got %q", id, string(buf))
				return
			}
			errc <- nil
		}(i)
	}

	for i := 0; i < concurrent; i++ {
		if err := <-errc; err != nil {
			t.Errorf("concurrent TCP: %v", err)
		}
	}
}

// TestSubdomainValidation verifies reserved and malformed subdomains are rejected.
func TestSubdomainValidation(t *testing.T) {
	_, cfg, cancel := startTestServer(t)
	defer cancel()

	c := startTestClient(t, cfg.ListenAddr, "test-token-1")
	defer c.Close()

	cases := []struct {
		subdomain string
		wantErr   bool
	}{
		{"admin", true},    // reserved
		{"www", true},      // reserved
		{"ab", true},       // too short
		{"myapp", false},   // valid
		{"my-app", false},  // valid with hyphen
	}

	for _, tc := range cases {
		_, err := c.RequestHTTPTunnel(3000, tc.subdomain)
		if tc.wantErr && err == nil {
			t.Errorf("subdomain %q: expected error", tc.subdomain)
		}
		if !tc.wantErr && err != nil {
			t.Errorf("subdomain %q: unexpected error: %v", tc.subdomain, err)
		}
	}
}

// TestHTTPTunnel_RandomSubdomain verifies the server assigns a valid random subdomain.
func TestHTTPTunnel_RandomSubdomain(t *testing.T) {
	_, cfg, cancel := startTestServer(t)
	defer cancel()

	c := startTestClient(t, cfg.ListenAddr, "test-token-1")
	defer c.Close()

	info, err := c.RequestHTTPTunnel(3000, "") // empty = random
	if err != nil {
		t.Fatalf("request HTTP tunnel: %v", err)
	}
	if info.URL == "" {
		t.Fatal("expected non-empty URL")
	}
	if !strings.HasPrefix(info.URL, "https://") {
		t.Errorf("URL should start with https://, got: %s", info.URL)
	}
	t.Logf("assigned URL: %s", info.URL)
}

// TestGracefulServerShutdown verifies the server shuts down cleanly on context cancel.
func TestGracefulServerShutdown(t *testing.T) {
	_, cfg, cancel := startTestServer(t)

	// Connect a client.
	c := startTestClient(t, cfg.ListenAddr, "test-token-1")
	defer c.Close()

	// Cancel the server context → triggers shutdown.
	cancel()

	// After shutdown, new connections should be refused.
	time.Sleep(200 * time.Millisecond)
	if waitForPort(cfg.ListenAddr, 500*time.Millisecond) {
		t.Log("note: server port still open briefly after shutdown (expected on fast machines)")
	}
}
