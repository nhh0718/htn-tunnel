package server

import (
	"fmt"
	"io"
	"log/slog"
	"net"
	"sync"
)

// TCPProxy manages raw TCP port forwarding. Each TCP tunnel gets its own
// listener; incoming connections are forwarded through yamux streams to the client.
type TCPProxy struct {
	tunnelManager *TunnelManager
}

// NewTCPProxy creates a TCPProxy backed by the given TunnelManager.
func NewTCPProxy(tm *TunnelManager) *TCPProxy {
	return &TCPProxy{tunnelManager: tm}
}

// StartTCPTunnel allocates a random port, starts the listener, and returns
// the allocated port. The accept loop runs in a background goroutine.
func (p *TCPProxy) StartTCPTunnel(session *TunnelSession) (int, error) {
	port, err := p.tunnelManager.RegisterTCP(session)
	if err != nil {
		return 0, err
	}

	go p.acceptLoop(session)
	return port, nil
}

// StopTCPTunnel closes the port listener and unregisters the tunnel.
func (p *TCPProxy) StopTCPTunnel(port int) {
	p.tunnelManager.UnregisterTCP(port)
}

// acceptLoop accepts remote TCP connections and proxies each one through a
// new yamux stream to the client.
func (p *TCPProxy) acceptLoop(session *TunnelSession) {
	ln := session.Listener
	slog.Info("TCP tunnel accepting", "port", session.Port)
	for {
		conn, err := ln.Accept()
		if err != nil {
			// Listener was closed (tunnel stopped); exit silently.
			return
		}
		go p.handleTCPConn(conn, session)
	}
}

// handleTCPConn proxies a single remote TCP connection through a yamux stream.
func (p *TCPProxy) handleTCPConn(conn net.Conn, session *TunnelSession) {
	defer conn.Close()

	stream, err := session.Session.Open()
	if err != nil {
		slog.Warn("TCP proxy: yamux open failed", "port", session.Port, "err", err)
		return
	}
	defer stream.Close()

	var wg sync.WaitGroup
	wg.Add(2)

	// remote → client
	go func() {
		defer wg.Done()
		n, _ := io.Copy(stream, conn)
		session.BytesIn += n
		stream.Close()
	}()

	// client → remote
	go func() {
		defer wg.Done()
		n, _ := io.Copy(conn, stream)
		session.BytesOut += n
		conn.Close()
	}()

	wg.Wait()
}

// countingWriter wraps an io.Writer and counts bytes written (for bandwidth tracking).
type countingWriter struct {
	w     io.Writer
	count int64
}

func (c *countingWriter) Write(p []byte) (int, error) {
	n, err := c.w.Write(p)
	c.count += int64(n)
	return n, err
}

// proxyBidirectional copies data in both directions between a and b,
// returning the bytes copied in each direction. Used by HTTP and TCP proxies.
func proxyBidirectional(a, b net.Conn) (int64, int64, error) {
	var (
		inBytes  int64
		outBytes int64
		wg       sync.WaitGroup
		aErr     error
	)
	wg.Add(2)

	go func() {
		defer wg.Done()
		inBytes, aErr = io.Copy(a, b)
		a.Close()
	}()

	go func() {
		defer wg.Done()
		outBytes, _ = io.Copy(b, a)
		b.Close()
	}()

	wg.Wait()
	if aErr != nil {
		return inBytes, outBytes, fmt.Errorf("proxy: %w", aErr)
	}
	return inBytes, outBytes, nil
}
