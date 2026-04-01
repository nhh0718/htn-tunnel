package client

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/nhh0718/htn-tunnel/internal/config"
	"github.com/nhh0718/htn-tunnel/internal/protocol"
)

// heartbeatInterval is how often the client sends a Heartbeat to the server.
const heartbeatInterval = 30 * time.Second

// heartbeatTimeout is how long to wait for a HeartbeatAck before counting a miss.
const heartbeatTimeout = 5 * time.Second

// maxHeartbeatMisses triggers reconnect after this many consecutive missed acks.
const maxHeartbeatMisses = 3

// baseBackoff is the initial reconnect delay.
const baseBackoff = 1 * time.Second

// maxBackoff caps the exponential backoff delay.
const maxBackoff = 60 * time.Second

// Reconnector wraps a Client and implements exponential-backoff reconnection
// with heartbeat-based dead-connection detection.
type Reconnector struct {
	cfg        *config.ClientConfig
	tunnelType string // "http" or "tcp"
	localPort  int
	subdomain  string // last known subdomain (may be reclaimed on reconnect)
}

// NewReconnector creates a Reconnector for the given tunnel parameters.
func NewReconnector(cfg *config.ClientConfig, tunnelType string, localPort int, subdomain string) *Reconnector {
	return &Reconnector{
		cfg:        cfg,
		tunnelType: tunnelType,
		localPort:  localPort,
		subdomain:  subdomain,
	}
}

// Run connects, requests a tunnel, and serves traffic. On any connection error
// it reconnects with exponential backoff. Runs until ctx is cancelled.
func (r *Reconnector) Run(ctx context.Context) error {
	backoff := baseBackoff
	attempt := 0

	for {
		if ctx.Err() != nil {
			return nil
		}

		c := NewClient(r.cfg)
		if err := c.Connect(ctx); err != nil {
			attempt++
			slog.Warn("reconnect failed", "attempt", attempt, "next_in", backoff, "err", err)
			fmt.Printf("  Disconnected. Reconnecting (attempt %d, next in %s)...\n", attempt, backoff)
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(backoff):
			}
			backoff = minDuration(backoff*2, maxBackoff)
			continue
		}

		info, err := r.requestTunnel(c)
		if err != nil {
			c.Close()
			attempt++
			slog.Warn("tunnel request failed after connect", "err", err)
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(backoff):
			}
			backoff = minDuration(backoff*2, maxBackoff)
			continue
		}

		// Update subdomain in case server assigned a different one.
		if r.tunnelType == "http" && info.URL != "" {
			r.subdomain = extractSubdomainFromURL(info.URL)
			fmt.Printf("  Tunnel:    %s → localhost:%d\n", info.URL, r.localPort)
		} else if r.tunnelType == "tcp" && info.RemotePort > 0 {
			fmt.Printf("  Tunnel:    tcp://remote:%d → localhost:%d\n", info.RemotePort, r.localPort)
		}

		// Reset backoff on successful connect.
		backoff = baseBackoff
		attempt = 0
		fmt.Println("  Status:    connected")

		// Run heartbeat and serve in parallel; reconnect on either error.
		serveErr := r.runWithHeartbeat(ctx, c)
		c.Close()

		if ctx.Err() != nil {
			return nil
		}
		slog.Info("connection lost, will reconnect", "err", serveErr)
		fmt.Printf("  Disconnected. Reconnecting...\n")
	}
}

// requestTunnel sends the TunnelRequest appropriate for r.tunnelType.
// Tries to reclaim r.subdomain first; falls back to random if unavailable.
func (r *Reconnector) requestTunnel(c *Client) (*TunnelInfo, error) {
	switch r.tunnelType {
	case "http":
		return c.RequestHTTPTunnel(r.localPort, r.subdomain)
	case "tcp":
		return c.RequestTCPTunnel(r.localPort)
	default:
		return nil, fmt.Errorf("unknown tunnel type %q", r.tunnelType)
	}
}

// runWithHeartbeat runs the heartbeat sender and tunnel serve loop concurrently.
// Returns when either fails (indicating the connection is dead).
func (r *Reconnector) runWithHeartbeat(ctx context.Context, c *Client) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	errCh := make(chan error, 2)

	go func() {
		errCh <- r.runHeartbeat(ctx, c)
	}()

	go func() {
		errCh <- c.ServeTunnel(ctx, r.localPort)
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		return nil
	}
}

// runHeartbeat sends Heartbeat messages every heartbeatInterval and waits for
// HeartbeatAck. After maxHeartbeatMisses consecutive missed acks it returns an
// error to trigger reconnection.
func (r *Reconnector) runHeartbeat(ctx context.Context, c *Client) error {
	ticker := time.NewTicker(heartbeatInterval)
	defer ticker.Stop()

	misses := 0

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if err := c.enc.Encode(protocol.MsgHeartbeat, nil); err != nil {
				return fmt.Errorf("send heartbeat: %w", err)
			}

			// Wait for ack with a short deadline.
			ackCh := make(chan error, 1)
			go func() {
				msgType, _, err := c.dec.Decode()
				if err != nil {
					ackCh <- err
					return
				}
				if msgType == protocol.MsgHeartbeatAck {
					ackCh <- nil
				} else {
					// Re-process other message types (e.g. TunnelResponse from a
					// concurrent request) — for MVP we just log and ignore.
					slog.Debug("heartbeat goroutine received non-ack message", "type", msgType)
					ackCh <- nil
				}
			}()

			select {
			case err := <-ackCh:
				if err != nil {
					misses++
					slog.Warn("heartbeat ack error", "misses", misses, "err", err)
				} else {
					misses = 0
				}
			case <-time.After(heartbeatTimeout):
				misses++
				slog.Warn("heartbeat ack timeout", "misses", misses)
			}

			if misses >= maxHeartbeatMisses {
				return fmt.Errorf("connection dead: %d consecutive missed heartbeats", misses)
			}
		}
	}
}

// extractSubdomainFromURL parses the first DNS label from a URL like
// "https://abc123.example.com" → "abc123".
func extractSubdomainFromURL(rawURL string) string {
	// Strip scheme.
	s := rawURL
	if idx := len("https://"); len(s) > idx {
		s = s[idx:]
	}
	// Take up to first dot.
	for i, ch := range s {
		if ch == '.' {
			return s[:i]
		}
	}
	return s
}

// minDuration returns the smaller of two durations.
func minDuration(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}
