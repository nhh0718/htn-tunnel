// Package client implements the htn-tunnel client: connects to the server,
// authenticates, requests tunnels, and proxies traffic to local services.
package client

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"

	"github.com/hashicorp/yamux"
	"github.com/nhh0718/htn-tunnel/internal/config"
	"github.com/nhh0718/htn-tunnel/internal/protocol"
)

// Client manages the control connection to the server.
type Client struct {
	cfg     *config.ClientConfig
	conn    net.Conn
	session *yamux.Session
	enc     *protocol.Encoder
	dec     *protocol.Decoder
}

// NewClient creates a Client from cfg. Call Connect() to establish the connection.
func NewClient(cfg *config.ClientConfig) *Client {
	return &Client{cfg: cfg}
}

// Connect dials the server, performs the Auth handshake, and establishes
// a yamux session. The client is the yamux initiator but ACCEPTS streams
// (the server opens streams to the client for each incoming request).
func (c *Client) Connect(ctx context.Context) error {
	dialer := &tls.Dialer{
		NetDialer: &net.Dialer{},
		Config: &tls.Config{
			// InsecureSkipVerify is only safe for self-signed dev certs.
			// Production deployments use a valid Let's Encrypt cert.
			InsecureSkipVerify: true, //nolint:gosec // see comment above
		},
	}

	conn, err := dialer.DialContext(ctx, "tcp", c.cfg.ServerAddr)
	if err != nil {
		return fmt.Errorf("dial %s: %w", c.cfg.ServerAddr, err)
	}

	c.conn = conn
	c.enc = protocol.NewEncoder(conn)
	c.dec = protocol.NewDecoder(conn)

	// Send Auth message.
	if err := c.enc.Encode(protocol.MsgAuth, protocol.AuthMsg{Token: c.cfg.Token}); err != nil {
		conn.Close()
		return fmt.Errorf("send auth: %w", err)
	}

	// Read AuthResponse.
	msgType, raw, err := c.dec.Decode()
	if err != nil {
		conn.Close()
		return fmt.Errorf("read auth response: %w", err)
	}
	if msgType != protocol.MsgAuthResponse {
		conn.Close()
		return fmt.Errorf("unexpected message type %d (want AuthResponse)", msgType)
	}

	var resp protocol.AuthResponseMsg
	if err := json.Unmarshal(raw, &resp); err != nil {
		conn.Close()
		return fmt.Errorf("decode auth response: %w", err)
	}
	if !resp.Success {
		conn.Close()
		return fmt.Errorf("authentication failed: %s", resp.Message)
	}

	// Upgrade to yamux session; client role = accepts streams from server.
	yamuxCfg := yamux.DefaultConfig()
	yamuxCfg.EnableKeepAlive = true
	yamuxCfg.KeepAliveInterval = 30 * 1e9 // 30s in nanoseconds
	yamuxCfg.ConnectionWriteTimeout = 10 * 1e9

	session, err := yamux.Client(conn, yamuxCfg)
	if err != nil {
		conn.Close()
		return fmt.Errorf("yamux client: %w", err)
	}
	c.session = session

	// Open a dedicated control stream for TunnelRequests and heartbeats.
	// yamux now owns conn; all further protocol I/O must go through streams,
	// not the raw conn — direct raw writes would corrupt yamux framing.
	controlStream, err := session.Open()
	if err != nil {
		session.Close()
		return fmt.Errorf("open control stream: %w", err)
	}
	c.enc = protocol.NewEncoder(controlStream)
	c.dec = protocol.NewDecoder(controlStream)

	slog.Info("connected to server", "addr", c.cfg.ServerAddr)
	return nil
}

// Session returns the underlying yamux session (used by tunnel.go).
func (c *Client) Session() *yamux.Session { return c.session }

// Encoder returns the protocol encoder for the control connection.
func (c *Client) Encoder() *protocol.Encoder { return c.enc }

// Decoder returns the protocol decoder for the control connection.
func (c *Client) Decoder() *protocol.Decoder { return c.dec }

// Close shuts down the yamux session and underlying TCP connection.
func (c *Client) Close() error {
	if c.session != nil {
		c.session.Close()
	}
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}
