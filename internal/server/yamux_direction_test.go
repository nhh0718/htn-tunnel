// Package server: yamux stream direction PoC test.
// Validates the critical assumption: the SERVER calls session.Open() to push
// streams TO the client (reverse of typical yamux usage).
// Client calls session.AcceptStream() to receive them.
// All phases 2-5 are built on this assumption.
package server

import (
	"bytes"
	"io"
	"net"
	"testing"
	"time"

	"github.com/hashicorp/yamux"
)

func TestYamuxStreamDirection_ServerOpensToClient(t *testing.T) {
	// Use net.Pipe() for a synchronous in-process connection.
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	const testPayload = "hello from server-opened stream"
	// clientDone signals that the client has finished reading so the server
	// can safely close its session without racing against the client read.
	clientDone := make(chan struct{})
	errc := make(chan error, 2)

	// Server side: yamux.Server wraps the connection, then opens a stream to client.
	go func() {
		session, err := yamux.Server(serverConn, yamux.DefaultConfig())
		if err != nil {
			errc <- err
			return
		}
		defer session.Close()

		stream, err := session.Open()
		if err != nil {
			errc <- err
			return
		}

		_, err = io.WriteString(stream, testPayload)
		stream.Close() // signal EOF to client before waiting
		if err != nil {
			errc <- err
			return
		}

		// Wait for client to finish reading before closing session.
		select {
		case <-clientDone:
		case <-time.After(5 * time.Second):
		}
		errc <- nil
	}()

	// Client side: yamux.Client wraps the connection, then accepts the stream.
	go func() {
		defer close(clientDone)

		session, err := yamux.Client(clientConn, yamux.DefaultConfig())
		if err != nil {
			errc <- err
			return
		}
		defer session.Close()

		stream, err := session.AcceptStream()
		if err != nil {
			errc <- err
			return
		}
		defer stream.Close()

		var buf bytes.Buffer
		if _, err := io.Copy(&buf, stream); err != nil {
			errc <- err
			return
		}
		if buf.String() != testPayload {
			t.Errorf("payload: got %q, want %q", buf.String(), testPayload)
		}
		errc <- nil
	}()

	timeout := time.After(5 * time.Second)
	for i := 0; i < 2; i++ {
		select {
		case err := <-errc:
			if err != nil {
				t.Fatalf("yamux direction test: %v", err)
			}
		case <-timeout:
			t.Fatal("yamux direction test timed out")
		}
	}
}
