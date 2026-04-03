package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"sync"

	"github.com/nhh0718/htn-tunnel/internal/protocol"
)

// TunnelInfo holds the result of a successful tunnel request.
type TunnelInfo struct {
	URL        string // https://sub.example.com (HTTP tunnels)
	RemotePort int    // allocated port number (TCP tunnels)
	LocalPort  int
}

// RequestHTTPTunnel sends a TunnelRequest for an HTTP tunnel and returns the
// public URL assigned by the server.
func (c *Client) RequestHTTPTunnel(localPort int, subdomain string) (*TunnelInfo, error) {
	req := protocol.TunnelRequestMsg{
		Type:      protocol.TunnelHTTP,
		Subdomain: subdomain,
		LocalPort: localPort,
	}
	if err := c.enc.Encode(protocol.MsgTunnelReq, req); err != nil {
		return nil, fmt.Errorf("send tunnel request: %w", err)
	}

	msgType, raw, err := c.dec.Decode()
	if err != nil {
		return nil, fmt.Errorf("read tunnel response: %w", err)
	}
	if msgType != protocol.MsgTunnelResp {
		return nil, fmt.Errorf("unexpected message type %d", msgType)
	}

	var resp protocol.TunnelResponseMsg
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("decode tunnel response: %w", err)
	}
	if !resp.Success {
		return nil, fmt.Errorf("tunnel request failed: %s", resp.Message)
	}
	return &TunnelInfo{URL: resp.URL, LocalPort: localPort}, nil
}

// RequestTCPTunnel sends a TunnelRequest for a TCP tunnel and returns the
// remote port allocated by the server.
func (c *Client) RequestTCPTunnel(localPort int) (*TunnelInfo, error) {
	req := protocol.TunnelRequestMsg{
		Type:      protocol.TunnelTCP,
		LocalPort: localPort,
	}
	if err := c.enc.Encode(protocol.MsgTunnelReq, req); err != nil {
		return nil, fmt.Errorf("send TCP tunnel request: %w", err)
	}

	msgType, raw, err := c.dec.Decode()
	if err != nil {
		return nil, fmt.Errorf("read TCP tunnel response: %w", err)
	}
	if msgType != protocol.MsgTunnelResp {
		return nil, fmt.Errorf("unexpected message type %d", msgType)
	}

	var resp protocol.TunnelResponseMsg
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("decode TCP tunnel response: %w", err)
	}
	if !resp.Success {
		return nil, fmt.Errorf("TCP tunnel request failed: %s", resp.Message)
	}
	return &TunnelInfo{RemotePort: resp.RemotePort, LocalPort: localPort}, nil
}

// AccountInfo holds the server's response to an account info request.
type AccountInfo struct {
	Name       string   `json:"name"`
	Subdomains []string `json:"subdomains"`
	MaxTunnels int      `json:"max_tunnels"`
	Domain     string   `json:"domain"`
}

// GetAccountInfo queries the server for the current key's account details.
func (c *Client) GetAccountInfo() (*AccountInfo, error) {
	if err := c.enc.Encode(protocol.MsgAccountInfo, nil); err != nil {
		return nil, fmt.Errorf("send account info request: %w", err)
	}
	msgType, raw, err := c.dec.Decode()
	if err != nil {
		return nil, fmt.Errorf("read account info response: %w", err)
	}
	if msgType != protocol.MsgAccountInfoResp {
		return nil, fmt.Errorf("unexpected message type %d", msgType)
	}
	var info AccountInfo
	if err := json.Unmarshal(raw, &info); err != nil {
		return nil, fmt.Errorf("decode account info: %w", err)
	}
	return &info, nil
}

// ServeTunnel accepts yamux streams from the server and forwards each one to
// localhost:localPort. Works identically for HTTP and TCP tunnels — both are
// raw byte streams at the yamux layer; the server handles protocol differences.
func (c *Client) ServeTunnel(ctx context.Context, localPort int) error {
	for {
		stream, err := c.session.AcceptStream()
		if err != nil {
			select {
			case <-ctx.Done():
				return nil
			default:
				return fmt.Errorf("accept stream: %w", err)
			}
		}
		go c.handleStream(stream, localPort)
	}
}

// handleStream dials localhost:localPort and proxies bytes between the yamux
// stream and the local connection. Errors on the local dial are logged but
// don't crash the client — the service may not yet be running.
func (c *Client) handleStream(stream io.ReadWriteCloser, localPort int) {
	defer stream.Close()

	local, err := net.Dial("tcp", fmt.Sprintf("localhost:%d", localPort))
	if err != nil {
		slog.Warn("local connection refused", "port", localPort, "err", err)
		return
	}
	defer local.Close()

	var wg sync.WaitGroup
	wg.Add(2)

	// server stream → local service
	go func() {
		defer wg.Done()
		io.Copy(local, stream) //nolint:errcheck
		// Signal EOF to local service if it supports half-close.
		if tc, ok := local.(*net.TCPConn); ok {
			tc.CloseWrite() //nolint:errcheck
		}
	}()

	// local service → server stream
	go func() {
		defer wg.Done()
		io.Copy(stream, local) //nolint:errcheck
		stream.Close()
	}()

	wg.Wait()
}
