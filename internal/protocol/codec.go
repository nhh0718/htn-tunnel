package protocol

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"sync"
)

// Encoder writes length-prefixed control messages to an io.Writer.
// Format: [4-byte length BE][1-byte version][1-byte type][N-byte JSON payload]
// Length covers version + type + payload (does NOT include the 4-byte length field).
// Thread-safe: multiple goroutines may call Encode concurrently.
type Encoder struct {
	w  io.Writer
	mu sync.Mutex
}

// NewEncoder returns an Encoder that writes to w.
func NewEncoder(w io.Writer) *Encoder {
	return &Encoder{w: w}
}

// Encode marshals payload as JSON (nil payload → zero-length payload for heartbeats)
// then writes the framed message atomically.
func (e *Encoder) Encode(msgType MsgType, payload interface{}) error {
	var payloadBytes []byte
	if payload != nil {
		var err error
		payloadBytes, err = json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("marshal payload: %w", err)
		}
	}

	// frame = version(1) + type(1) + payload(N)
	frameLen := uint32(2 + len(payloadBytes))
	header := make([]byte, 6) // 4-byte length + 1-byte version + 1-byte type
	binary.BigEndian.PutUint32(header[0:4], frameLen)
	header[4] = ProtocolVersion
	header[5] = byte(msgType)

	e.mu.Lock()
	defer e.mu.Unlock()

	if _, err := e.w.Write(header); err != nil {
		return fmt.Errorf("write header: %w", err)
	}
	if len(payloadBytes) > 0 {
		if _, err := e.w.Write(payloadBytes); err != nil {
			return fmt.Errorf("write payload: %w", err)
		}
	}
	return nil
}

// Decoder reads length-prefixed control messages from an io.Reader.
// Not thread-safe; use a single goroutine per Decoder.
type Decoder struct {
	r io.Reader
}

// NewDecoder returns a Decoder that reads from r.
func NewDecoder(r io.Reader) *Decoder {
	return &Decoder{r: r}
}

// Decode reads the next message. Returns (msgType, rawJSONPayload, error).
// Caller is responsible for unmarshaling the payload into the appropriate struct.
// Returns an error if:
//   - protocol version is unknown
//   - payload exceeds MaxMessageSize
//   - underlying read fails
func (d *Decoder) Decode() (MsgType, []byte, error) {
	// read 4-byte length prefix
	var lenBuf [4]byte
	if _, err := io.ReadFull(d.r, lenBuf[:]); err != nil {
		return 0, nil, fmt.Errorf("read length: %w", err)
	}
	frameLen := binary.BigEndian.Uint32(lenBuf[:])

	// frame must contain at least version(1) + type(1)
	if frameLen < 2 {
		return 0, nil, fmt.Errorf("frame too short: %d bytes", frameLen)
	}

	// guard against malicious large messages
	payloadLen := int(frameLen) - 2
	if payloadLen > MaxMessageSize {
		return 0, nil, fmt.Errorf("message too large: %d bytes (max %d)", payloadLen, MaxMessageSize)
	}

	// read version + type
	var meta [2]byte
	if _, err := io.ReadFull(d.r, meta[:]); err != nil {
		return 0, nil, fmt.Errorf("read version/type: %w", err)
	}
	version := meta[0]
	msgType := MsgType(meta[1])

	if version != ProtocolVersion {
		return 0, nil, fmt.Errorf("unsupported protocol version: 0x%02x", version)
	}

	// read payload (may be zero-length for heartbeats)
	var payload []byte
	if payloadLen > 0 {
		payload = make([]byte, payloadLen)
		if _, err := io.ReadFull(d.r, payload); err != nil {
			return 0, nil, fmt.Errorf("read payload: %w", err)
		}
	}

	return msgType, payload, nil
}

// DecodeInto decodes the next message and unmarshals the payload into v.
// Returns the message type so callers can verify they got the expected type.
func DecodeInto(d *Decoder, v interface{}) (MsgType, error) {
	msgType, raw, err := d.Decode()
	if err != nil {
		return 0, err
	}
	if len(raw) > 0 && v != nil {
		if err := json.Unmarshal(raw, v); err != nil {
			return msgType, fmt.Errorf("unmarshal %T: %w", v, err)
		}
	}
	return msgType, nil
}
