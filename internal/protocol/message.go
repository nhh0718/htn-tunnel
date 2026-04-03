// Package protocol defines the wire protocol for htn-tunnel control connections.
// Actual tunnel data flows through yamux streams directly; only control messages
// (auth, tunnel requests, heartbeat) use this protocol.
package protocol

// MsgType identifies the type of control message.
type MsgType uint8

const (
	// ProtocolVersion is the current wire protocol version byte.
	ProtocolVersion byte = 0x01

	// MaxMessageSize is the hard cap on control message payload (1 MB).
	// Reject and close connection if exceeded to prevent memory exhaustion.
	MaxMessageSize = 1 << 20 // 1 MB

	MsgAuth            MsgType = 0x01
	MsgAuthResponse    MsgType = 0x02
	MsgTunnelReq       MsgType = 0x03
	MsgTunnelResp      MsgType = 0x04
	MsgHeartbeat       MsgType = 0x05
	MsgHeartbeatAck    MsgType = 0x06
	MsgRegister        MsgType = 0x0A
	MsgRegisterResp    MsgType = 0x0B
	MsgAccountInfo     MsgType = 0x0E
	MsgAccountInfoResp MsgType = 0x0F
	MsgRequestLog      MsgType = 0x10
)

// AuthMsg is sent by the client as the first message on a control connection.
type AuthMsg struct {
	Token string `json:"token"`
}

// AuthResponseMsg is the server's reply to AuthMsg.
type AuthResponseMsg struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// TunnelType distinguishes HTTP subdomain tunnels from raw TCP port tunnels.
type TunnelType string

const (
	TunnelHTTP TunnelType = "http"
	TunnelTCP  TunnelType = "tcp"
)

// TunnelRequestMsg is sent by the client to open a new tunnel.
// LocalPort must be in [1, 65535]. Subdomain is optional for HTTP tunnels;
// server assigns a random 8-char name when omitted.
type TunnelRequestMsg struct {
	Type      TunnelType `json:"type"`
	Subdomain string     `json:"subdomain,omitempty"`
	LocalPort int        `json:"local_port"`
}

// TunnelResponseMsg is the server's reply to TunnelRequestMsg.
type TunnelResponseMsg struct {
	Success    bool   `json:"success"`
	URL        string `json:"url,omitempty"`         // for HTTP tunnels
	RemotePort int    `json:"remote_port,omitempty"` // for TCP tunnels
	Message    string `json:"message,omitempty"`
}

// RegisterMsg is sent by a new user to create an API key (no auth required).
type RegisterMsg struct {
	Name      string `json:"name"`
	Subdomain string `json:"subdomain,omitempty"`
}

// RegisterResponseMsg is the server's reply to RegisterMsg.
type RegisterResponseMsg struct {
	Success    bool     `json:"success"`
	Key        string   `json:"key,omitempty"`
	Subdomains []string `json:"subdomains,omitempty"`
	Message    string   `json:"message,omitempty"`
}

// AccountInfoRespMsg is returned for MsgAccountInfo requests.
type AccountInfoRespMsg struct {
	Name       string   `json:"name"`
	Subdomains []string `json:"subdomains"`
	MaxTunnels int      `json:"max_tunnels"`
	Domain     string   `json:"domain,omitempty"`
}

// RequestLogMsg is sent from server to client for each proxied request.
type RequestLogMsg struct {
	Method   string `json:"m"`
	Path     string `json:"p"`
	Status   int    `json:"s"`
	Duration int    `json:"d"` // milliseconds
	Size     int64  `json:"z"` // response bytes
}
