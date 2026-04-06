package server

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"net"
	"testing"
	"time"

	"github.com/nhh0718/htn-tunnel/internal/config"
	"github.com/nhh0718/htn-tunnel/internal/protocol"
)

// dialTestServer connects to addr with TLS (skipping cert verification for dev certs).
func dialTestServer(t *testing.T, addr string) net.Conn {
	t.Helper()
	conn, err := tls.Dial("tcp", addr,
		&tls.Config{InsecureSkipVerify: true}) //nolint:gosec
	if err != nil {
		t.Fatalf("dial %s: %v", addr, err)
	}
	return conn
}

// startLocalServer creates a test server on a random port and returns its address + cancel.
func startLocalServer(t *testing.T, tokens []string) (string, context.CancelFunc) {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("find port: %v", err)
	}
	addr := ln.Addr().String()
	ln.Close()

	cfg := &config.ServerConfig{
		ListenAddr:         addr,
		Domain:             "localhost",
		Tokens:             tokens,
		MaxTunnelsPerToken: 5,
		RateLimit:          1000,
		GlobalRateLimit:    10000,
		TCPPortRange:       [2]int{30000, 39999},
		DevMode:            true,
	}

	srv, err := NewServer(cfg, "", "test")
	if err != nil {
		t.Fatalf("new server: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	go srv.Start(ctx) //nolint:errcheck

	// Wait for server to accept connections.
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if c, err := tls.Dial("tcp", addr, &tls.Config{InsecureSkipVerify: true}); err == nil { //nolint:gosec
			c.Close()
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	return addr, cancel
}

func TestAuthSuccess(t *testing.T) {
	addr, cancel := startLocalServer(t, []string{"good-token"})
	defer cancel()

	conn := dialTestServer(t, addr)
	defer conn.Close()

	enc := protocol.NewEncoder(conn)
	dec := protocol.NewDecoder(conn)

	if err := enc.Encode(protocol.MsgAuth, protocol.AuthMsg{Token: "good-token"}); err != nil {
		t.Fatal(err)
	}

	msgType, raw, err := dec.Decode()
	if err != nil {
		t.Fatalf("decode auth response: %v", err)
	}
	if msgType != protocol.MsgAuthResponse {
		t.Fatalf("expected AuthResponse, got %d", msgType)
	}

	var resp protocol.AuthResponseMsg
	if err := json.Unmarshal(raw, &resp); err != nil {
		t.Fatal(err)
	}
	if !resp.Success {
		t.Errorf("expected success=true, got: %s", resp.Message)
	}
}

func TestAuthFailure_InvalidToken(t *testing.T) {
	addr, cancel := startLocalServer(t, []string{"good-token"})
	defer cancel()

	conn := dialTestServer(t, addr)
	defer conn.Close()

	enc := protocol.NewEncoder(conn)
	dec := protocol.NewDecoder(conn)

	if err := enc.Encode(protocol.MsgAuth, protocol.AuthMsg{Token: "bad-token"}); err != nil {
		t.Fatal(err)
	}

	msgType, raw, err := dec.Decode()
	if err != nil {
		t.Fatalf("decode auth response: %v", err)
	}
	if msgType != protocol.MsgAuthResponse {
		t.Fatalf("expected AuthResponse, got %d", msgType)
	}

	var resp protocol.AuthResponseMsg
	if err := json.Unmarshal(raw, &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Success {
		t.Error("expected success=false for invalid token")
	}
}

func TestAuthFailure_WrongMessageType(t *testing.T) {
	addr, cancel := startLocalServer(t, []string{"good-token"})
	defer cancel()

	conn := dialTestServer(t, addr)
	defer conn.Close()

	enc := protocol.NewEncoder(conn)

	// Send a Heartbeat instead of Auth — server should close the connection.
	if err := enc.Encode(protocol.MsgHeartbeat, nil); err != nil {
		t.Fatal(err)
	}

	// Server should close the connection without sending a response.
	dec := protocol.NewDecoder(conn)
	conn.SetDeadline(time.Now().Add(2 * time.Second)) //nolint:errcheck
	_, _, err := dec.Decode()
	if err == nil {
		t.Error("expected connection close after wrong initial message type")
	}
}

func TestTunnelManager_HTTPRegistration(t *testing.T) {
	tm := NewTunnelManager(40000, 49999)

	ts := &TunnelSession{Token: "tok1", Type: "http"}

	// Register a new subdomain.
	if err := tm.RegisterHTTP("myapp", ts); err != nil {
		t.Fatalf("register: %v", err)
	}

	// Lookup should return the session.
	if got := tm.LookupHTTP("myapp"); got != ts {
		t.Errorf("lookup: got %v, want %v", got, ts)
	}

	// Duplicate registration (different session) should fail.
	ts2 := &TunnelSession{Token: "tok2", Type: "http"}
	if err := tm.RegisterHTTP("myapp", ts2); err == nil {
		t.Error("expected error on duplicate subdomain registration")
	}

	// Unregister then re-register should succeed.
	tm.UnregisterHTTP("myapp")
	if got := tm.LookupHTTP("myapp"); got != nil {
		t.Errorf("lookup after unregister: got %v, want nil", got)
	}
	if err := tm.RegisterHTTP("myapp", ts); err != nil {
		t.Errorf("re-register after unregister: %v", err)
	}
}

func TestTunnelManager_ReservationReclaim(t *testing.T) {
	tm := NewTunnelManager(50000, 59999)

	ts := &TunnelSession{Token: "tok1", Type: "http"}
	if err := tm.RegisterHTTP("myapp", ts); err != nil {
		t.Fatal(err)
	}

	// Reserve the subdomain (simulates client disconnect).
	tm.ReserveHTTP("myapp", 60*time.Second)

	// Same-token reclaim should succeed.
	ts2 := &TunnelSession{Token: "tok1", Type: "http"}
	if err := tm.RegisterHTTP("myapp", ts2); err != nil {
		t.Errorf("same-token reclaim failed: %v", err)
	}

	// Reserve again, then different-token claim should fail.
	tm.ReserveHTTP("myapp", 60*time.Second)
	ts3 := &TunnelSession{Token: "tok2", Type: "http"}
	if err := tm.RegisterHTTP("myapp", ts3); err == nil {
		t.Error("expected error: different token trying to claim reserved subdomain")
	}
}

func TestTunnelManager_InvalidSubdomain(t *testing.T) {
	tm := NewTunnelManager(60000, 65535)
	ts := &TunnelSession{Token: "tok", Type: "http"}

	cases := []struct {
		sub string
		ok  bool
	}{
		{"ab", false},          // too short
		{"goodsub", true},
		{"admin", false},       // reserved
		{"my-app-123", true},
		{"-badstart", false},   // starts with hyphen
		{"badend-", false},     // ends with hyphen
		{"ALL-CAPS", false},    // uppercase not matched
		{"ok-sub", true},
	}

	for _, tc := range cases {
		err := tm.RegisterHTTP(tc.sub, ts)
		if tc.ok && err != nil {
			t.Errorf("subdomain %q: expected ok, got: %v", tc.sub, err)
		}
		if !tc.ok && err == nil {
			t.Errorf("subdomain %q: expected error, got nil", tc.sub)
		}
		if err == nil {
			tm.UnregisterHTTP(tc.sub)
		}
	}
}

func TestTokenStore_Validation(t *testing.T) {
	ts, err := NewTokenStore([]string{"valid-token-abc"}, 5)
	if err != nil {
		t.Fatal(err)
	}

	if !ts.Validate("valid-token-abc") {
		t.Error("expected valid token to pass validation")
	}
	if ts.Validate("invalid-token") {
		t.Error("expected invalid token to fail validation")
	}
	if ts.Validate("") {
		t.Error("expected empty token to fail validation")
	}
}

func TestTokenStore_TunnelCountLimit(t *testing.T) {
	ts, err := NewTokenStore([]string{"tok"}, 2)
	if err != nil {
		t.Fatal(err)
	}

	if err := ts.IncrementTunnels("tok"); err != nil {
		t.Fatalf("increment 1: %v", err)
	}
	if err := ts.IncrementTunnels("tok"); err != nil {
		t.Fatalf("increment 2: %v", err)
	}
	// Third increment should fail.
	if err := ts.IncrementTunnels("tok"); err == nil {
		t.Error("expected error at tunnel limit")
	}

	// After decrement, increment should succeed again.
	ts.DecrementTunnels("tok")
	if err := ts.IncrementTunnels("tok"); err != nil {
		t.Errorf("increment after decrement: %v", err)
	}
}
