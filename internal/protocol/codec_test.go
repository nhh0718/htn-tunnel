package protocol

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"
	"sync"
	"testing"
)

// roundTrip encodes a message then decodes it, returning the type and raw payload.
func roundTrip(t *testing.T, msgType MsgType, payload interface{}) (MsgType, []byte) {
	t.Helper()
	var buf bytes.Buffer
	enc := NewEncoder(&buf)
	if err := enc.Encode(msgType, payload); err != nil {
		t.Fatalf("encode: %v", err)
	}
	dec := NewDecoder(&buf)
	gotType, gotPayload, err := dec.Decode()
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	return gotType, gotPayload
}

func TestRoundTrip_Auth(t *testing.T) {
	orig := AuthMsg{Token: "tok_test123"}
	gotType, raw := roundTrip(t, MsgAuth, orig)
	if gotType != MsgAuth {
		t.Fatalf("type: got %d, want %d", gotType, MsgAuth)
	}
	var got AuthMsg
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	if got.Token != orig.Token {
		t.Errorf("token: got %q, want %q", got.Token, orig.Token)
	}
}

func TestRoundTrip_AuthResponse(t *testing.T) {
	orig := AuthResponseMsg{Success: true, Message: "ok"}
	gotType, raw := roundTrip(t, MsgAuthResponse, orig)
	if gotType != MsgAuthResponse {
		t.Fatalf("type: got %d, want %d", gotType, MsgAuthResponse)
	}
	var got AuthResponseMsg
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	if !got.Success || got.Message != "ok" {
		t.Errorf("got %+v", got)
	}
}

func TestRoundTrip_TunnelRequest(t *testing.T) {
	orig := TunnelRequestMsg{Type: TunnelHTTP, Subdomain: "myapp", LocalPort: 3000}
	gotType, raw := roundTrip(t, MsgTunnelReq, orig)
	if gotType != MsgTunnelReq {
		t.Fatalf("type mismatch: got %d, want %d", gotType, MsgTunnelReq)
	}
	var got TunnelRequestMsg
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	if got.Type != TunnelHTTP || got.Subdomain != "myapp" || got.LocalPort != 3000 {
		t.Errorf("got %+v", got)
	}
}

func TestRoundTrip_TunnelResponse(t *testing.T) {
	orig := TunnelResponseMsg{Success: true, URL: "https://myapp.example.com"}
	gotType, raw := roundTrip(t, MsgTunnelResp, orig)
	if gotType != MsgTunnelResp {
		t.Fatalf("type mismatch: got %d, want %d", gotType, MsgTunnelResp)
	}
	var got TunnelResponseMsg
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	if !got.Success || got.URL != orig.URL {
		t.Errorf("got %+v", got)
	}
}

func TestRoundTrip_Heartbeat(t *testing.T) {
	var buf bytes.Buffer
	enc := NewEncoder(&buf)
	if err := enc.Encode(MsgHeartbeat, nil); err != nil {
		t.Fatal(err)
	}
	dec := NewDecoder(&buf)
	gotType, payload, err := dec.Decode()
	if err != nil {
		t.Fatal(err)
	}
	if gotType != MsgHeartbeat {
		t.Fatalf("type: got %d, want %d", gotType, MsgHeartbeat)
	}
	if len(payload) != 0 {
		t.Errorf("heartbeat should have empty payload, got %d bytes", len(payload))
	}
}

func TestRoundTrip_HeartbeatAck(t *testing.T) {
	var buf bytes.Buffer
	enc := NewEncoder(&buf)
	if err := enc.Encode(MsgHeartbeatAck, nil); err != nil {
		t.Fatal(err)
	}
	dec := NewDecoder(&buf)
	gotType, payload, err := dec.Decode()
	if err != nil {
		t.Fatal(err)
	}
	if gotType != MsgHeartbeatAck {
		t.Fatalf("type: got %d, want %d", gotType, MsgHeartbeatAck)
	}
	if len(payload) != 0 {
		t.Errorf("heartbeat ack should have empty payload")
	}
}

func TestDecoder_RejectsOversizedMessage(t *testing.T) {
	// Craft a frame claiming a 2 MB payload (exceeds 1 MB limit).
	payloadLen := 2 * MaxMessageSize
	frameLen := uint32(2 + payloadLen)
	var buf bytes.Buffer
	buf.Write([]byte{
		byte(frameLen >> 24), byte(frameLen >> 16), byte(frameLen >> 8), byte(frameLen),
		ProtocolVersion,
		byte(MsgAuth),
	})
	dec := NewDecoder(&buf)
	_, _, err := dec.Decode()
	if err == nil {
		t.Fatal("expected error for oversized message")
	}
	if !strings.Contains(err.Error(), "too large") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestDecoder_RejectsUnknownVersion(t *testing.T) {
	frameLen := uint32(2) // version + type, no payload
	var buf bytes.Buffer
	buf.Write([]byte{
		byte(frameLen >> 24), byte(frameLen >> 16), byte(frameLen >> 8), byte(frameLen),
		0x99, // unknown version
		byte(MsgAuth),
	})
	dec := NewDecoder(&buf)
	_, _, err := dec.Decode()
	if err == nil {
		t.Fatal("expected error for unknown version")
	}
	if !strings.Contains(err.Error(), "unsupported protocol version") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestDecoder_HandlesTruncatedStream(t *testing.T) {
	// Write only 2 of the 4 length bytes then EOF.
	r := io.LimitReader(strings.NewReader("\x00\x00"), 2)
	dec := NewDecoder(r)
	_, _, err := dec.Decode()
	if err == nil {
		t.Fatal("expected error on truncated stream")
	}
}

func TestEncoder_ConcurrentWrites(t *testing.T) {
	var buf syncBuffer
	enc := NewEncoder(&buf)
	const workers = 20
	var wg sync.WaitGroup
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			if err := enc.Encode(MsgHeartbeat, nil); err != nil {
				t.Errorf("concurrent encode: %v", err)
			}
		}()
	}
	wg.Wait()

	// Each heartbeat frame = 4 (length) + 1 (version) + 1 (type) = 6 bytes, no payload.
	if got := buf.Len(); got != workers*6 {
		t.Errorf("buffer len: got %d, want %d", got, workers*6)
	}
}

func BenchmarkHeartbeatEncode(b *testing.B) {
	var buf bytes.Buffer
	enc := NewEncoder(&buf)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf.Reset()
		_ = enc.Encode(MsgHeartbeat, nil)
	}
}

func BenchmarkHeartbeatDecode(b *testing.B) {
	var buf bytes.Buffer
	enc := NewEncoder(&buf)
	_ = enc.Encode(MsgHeartbeat, nil)
	frame := buf.Bytes()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		dec := NewDecoder(bytes.NewReader(frame))
		_, _, _ = dec.Decode()
	}
}

// syncBuffer is a thread-safe bytes.Buffer for concurrent write tests.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.Write(p)
}

func (s *syncBuffer) Len() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.Len()
}
