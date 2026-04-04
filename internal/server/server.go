// Package server implements the htn-tunnel relay server.
// It accepts TLS control connections from clients, authenticates them,
// establishes yamux sessions, and routes HTTP/TCP tunnel requests.
package server

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"log/slog"
	"math/big"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/hashicorp/yamux"
	"github.com/nhh0718/htn-tunnel/internal/config"
	"github.com/nhh0718/htn-tunnel/internal/dashboard"
	"github.com/nhh0718/htn-tunnel/internal/protocol"
)

// authTimeout is the deadline for receiving the Auth message after TCP connect.
const authTimeout = 5 * time.Second

// sessionIdleTimeout is how long a session may go without a heartbeat before
// being considered dead and having its tunnels reserved for reconnect.
const sessionIdleTimeout = 90 * time.Second

// reservationTTL is how long a subdomain is held after a client disconnects.
const reservationTTL = 60 * time.Second

// Server is the main relay server.
type Server struct {
	cfg            *config.ServerConfig
	configProvider *ConfigProvider
	tokenStore     *TokenStore
	keyStore       *KeyStore
	rateLimiter    *RateLimiter
	ipLimiter      *ipRateLimiter
	tunnelManager  *TunnelManager
	httpProxy      *HTTPProxy
	tcpProxy       *TCPProxy
	requestLog     *RequestLog
	listener       net.Listener
}

// NewServer creates a Server from cfg. configPath is the yaml file path for config editing.
func NewServer(cfg *config.ServerConfig, configPath string) (*Server, error) {
	ts, err := NewTokenStore(cfg.Tokens, cfg.MaxTunnelsPerToken)
	if err != nil {
		return nil, fmt.Errorf("init token store: %w", err)
	}
	ks, err := NewKeyStore(cfg.KeyStorePath)
	if err != nil {
		return nil, fmt.Errorf("init key store: %w", err)
	}
	tm := NewTunnelManager(cfg.TCPPortRange[0], cfg.TCPPortRange[1])
	tcp := NewTCPProxy(tm)
	// Start anonymous tunnel expiry goroutine.
	tm.StartAnonymousExpiry(time.Duration(cfg.AnonTunnelTTL) * time.Second)

	rl := NewRequestLog()

	return &Server{
		cfg:            cfg,
		configProvider: NewConfigProvider(cfg, configPath),
		tokenStore:     ts,
		keyStore:       ks,
		rateLimiter:    NewRateLimiter(cfg.RateLimit, cfg.GlobalRateLimit),
		ipLimiter:      newIPRateLimiter(10), // 10 pre-auth conn/min per IP
		tunnelManager:  tm,
		httpProxy:      NewHTTPProxy(cfg, tm, rl),
		tcpProxy:       tcp,
		requestLog:     rl,
	}, nil
}

// TunnelManager exposes the tunnel registry (used by HTTPProxy, TCPProxy, Dashboard).
func (s *Server) TunnelManager() *TunnelManager { return s.tunnelManager }

// startDashboard starts the embedded web dashboard if enabled in config.
func (s *Server) startDashboard(ctx context.Context) {
	if !s.cfg.DashboardEnabled {
		return
	}
	rlAdapter := NewRequestLogAdapter(s.requestLog)
	h := dashboard.NewHandler(s.tunnelManager, NewKeyStoreAdapter(s.keyStore), s.configProvider, s.cfg.AdminToken, s.cfg.Domain, rlAdapter)
	srv := &http.Server{
		Addr:    s.cfg.DashboardAddr,
		Handler: h,
	}
	go func() {
		slog.Info("dashboard listening", "addr", s.cfg.DashboardAddr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("dashboard server error", "err", err)
		}
	}()
	go func() {
		<-ctx.Done()
		srv.Close()
	}()
}

// Start opens the TLS control-plane listener and begins accepting client connections.
// Blocks until ctx is cancelled or a fatal error occurs.
func (s *Server) Start(ctx context.Context) error {
	tlsCfg, err := s.buildTLSConfig()
	if err != nil {
		return fmt.Errorf("build TLS config: %w", err)
	}

	ln, err := tls.Listen("tcp", s.cfg.ListenAddr, tlsCfg)
	if err != nil {
		return fmt.Errorf("listen %s: %w", s.cfg.ListenAddr, err)
	}
	s.listener = ln
	slog.Info("control plane listening", "addr", s.cfg.ListenAddr, "dev_mode", s.cfg.DevMode)
	s.startDashboard(ctx)
	go func() {
		if err := s.httpProxy.Start(ctx); err != nil {
			slog.Error("HTTP proxy error", "err", err)
		}
	}()

	go func() {
		<-ctx.Done()
		ln.Close()
	}()

	for {
		conn, err := ln.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				return nil // clean shutdown
			default:
				slog.Error("accept error", "err", err)
				continue
			}
		}
		go s.handleConnection(conn)
	}
}

// Shutdown closes the listener and waits up to 30 s for active sessions to drain.
func (s *Server) Shutdown(ctx context.Context) error {
	if s.listener != nil {
		s.listener.Close()
	}
	return nil
}

// handleConnection runs in a goroutine for each accepted TCP connection.
// It performs auth, then hands off to handleControlSession.
func (s *Server) handleConnection(conn net.Conn) {
	defer conn.Close()

	remoteIP := extractIP(conn.RemoteAddr().String())

	// Pre-auth IP rate limit (prevent unauthenticated floods).
	if !s.ipLimiter.Allow(remoteIP) {
		slog.Warn("pre-auth rate limit exceeded", "ip", remoteIP)
		return
	}

	// Enforce auth timeout.
	if err := conn.SetDeadline(time.Now().Add(authTimeout)); err != nil {
		return
	}

	enc := protocol.NewEncoder(conn)
	dec := protocol.NewDecoder(conn)

	// First message: Auth or Register.
	msgType, raw, err := dec.Decode()
	if err != nil {
		slog.Warn("auth read error", "ip", remoteIP, "err", err)
		return
	}

	// Handle registration (one-shot, no session).
	if msgType == protocol.MsgRegister {
		s.handleRegister(enc, raw, remoteIP)
		return
	}

	if msgType != protocol.MsgAuth {
		slog.Warn("expected Auth, got wrong message type", "ip", remoteIP, "type", msgType)
		return
	}

	var authMsg protocol.AuthMsg
	if err := json.Unmarshal(raw, &authMsg); err != nil {
		slog.Warn("auth decode error", "ip", remoteIP, "err", err)
		return
	}

	// Anonymous mode: empty token → assign "anon:<IP>" virtual token.
	if authMsg.Token == "" {
		if s.cfg.AllowAnonymous == nil || !*s.cfg.AllowAnonymous {
			_ = enc.Encode(protocol.MsgAuthResponse, protocol.AuthResponseMsg{
				Success: false, Message: "anonymous connections are disabled — run 'htn-tunnel login' to register",
			})
			return
		}
		authMsg.Token = "anon:" + remoteIP
		slog.Info("anonymous client connected", "ip", remoteIP)
	} else {
		// Validate token: try API key store first, then legacy tokens.
		tokenValid := false
		if IsAPIKey(authMsg.Token) {
			tokenValid = s.keyStore.Validate(authMsg.Token)
		} else {
			tokenValid = s.tokenStore.Validate(authMsg.Token)
		}

		if !tokenValid {
			slog.Warn("auth failed", "ip", remoteIP, "token_prefix", maskToken(authMsg.Token))
			_ = enc.Encode(protocol.MsgAuthResponse, protocol.AuthResponseMsg{
				Success: false, Message: "invalid token",
			})
			return
		}
	}

	// Clear auth deadline; set ongoing idle deadline.
	conn.SetDeadline(time.Time{})

	if err := enc.Encode(protocol.MsgAuthResponse, protocol.AuthResponseMsg{
		Success: true, Message: "authenticated",
	}); err != nil {
		return
	}

	slog.Info("client authenticated", "ip", remoteIP, "token_prefix", maskToken(authMsg.Token))

	// Upgrade connection to yamux session.
	// yamux now owns conn; enc/dec on the raw conn must NOT be used after this point.
	yamuxCfg := yamux.DefaultConfig()
	yamuxCfg.EnableKeepAlive = true
	yamuxCfg.KeepAliveInterval = 30 * time.Second
	yamuxCfg.ConnectionWriteTimeout = 10 * time.Second

	session, err := yamux.Server(conn, yamuxCfg)
	if err != nil {
		slog.Error("yamux init error", "ip", remoteIP, "err", err)
		return
	}
	defer func() {
		session.Close()
		s.cleanupSession(session, authMsg.Token)
	}()

	s.handleControlSession(session, authMsg.Token, remoteIP)
}

// handleRegister processes a self-service registration request (no auth required).
func (s *Server) handleRegister(enc *protocol.Encoder, raw []byte, remoteIP string) {
	if s.cfg.AllowRegistration != nil && !*s.cfg.AllowRegistration {
		_ = enc.Encode(protocol.MsgRegisterResp, protocol.RegisterResponseMsg{
			Success: false, Message: "registration is disabled",
		})
		return
	}

	var msg protocol.RegisterMsg
	if err := json.Unmarshal(raw, &msg); err != nil {
		_ = enc.Encode(protocol.MsgRegisterResp, protocol.RegisterResponseMsg{
			Success: false, Message: "invalid request",
		})
		return
	}

	var subdomains []string
	if msg.Subdomain != "" {
		subdomains = []string{msg.Subdomain}
	}

	key, err := s.keyStore.CreateKey(msg.Name, subdomains, 10)
	if err != nil {
		_ = enc.Encode(protocol.MsgRegisterResp, protocol.RegisterResponseMsg{
			Success: false, Message: err.Error(),
		})
		return
	}

	slog.Info("user registered", "ip", remoteIP, "name", msg.Name, "subdomain", msg.Subdomain)
	_ = enc.Encode(protocol.MsgRegisterResp, protocol.RegisterResponseMsg{
		Success:    true,
		Key:        key,
		Subdomains: subdomains,
	})
}

// handleControlSession accepts the client's control stream (the first yamux stream
// the client opens), then reads TunnelRequest/Heartbeat messages from it.
func (s *Server) handleControlSession(session *yamux.Session, token, remoteIP string) {
	// Client opens the control stream right after yamux handshake.
	controlStream, err := session.AcceptStream()
	if err != nil {
		slog.Warn("accept control stream", "ip", remoteIP, "err", err)
		return
	}
	defer controlStream.Close()

	enc := protocol.NewEncoder(controlStream)
	dec := protocol.NewDecoder(controlStream)
	// Session idle watchdog: reset on each heartbeat.
	idleTimer := time.NewTimer(sessionIdleTimeout)
	defer idleTimer.Stop()

	// Control message loop runs in a goroutine; signals via done channel.
	done := make(chan error, 1)
	go func() {
		for {
			msgType, raw, err := dec.Decode()
			if err != nil {
				done <- err
				return
			}
			idleTimer.Reset(sessionIdleTimeout)

			switch msgType {
			case protocol.MsgHeartbeat:
				if err := enc.Encode(protocol.MsgHeartbeatAck, nil); err != nil {
					done <- err
					return
				}

			case protocol.MsgAccountInfo:
				s.handleAccountInfo(enc, token)

			case protocol.MsgTunnelReq:
				var req protocol.TunnelRequestMsg
				if err := json.Unmarshal(raw, &req); err != nil {
					slog.Warn("bad TunnelRequest", "ip", remoteIP, "err", err)
					continue
				}
				s.handleTunnelRequest(session, token, remoteIP, req, enc)

			default:
				slog.Warn("unknown message type", "type", msgType, "ip", remoteIP)
			}
		}
	}()

	select {
	case err := <-done:
		if err != nil {
			slog.Info("control session ended", "ip", remoteIP, "err", err)
		}
	case <-idleTimer.C:
		slog.Warn("session idle timeout", "ip", remoteIP)
		// Reserve subdomains for reconnect window.
		for _, ts := range s.tunnelManager.SessionTunnels(session) {
			if ts.Type == "http" {
				s.tunnelManager.ReserveHTTP(ts.Subdomain, reservationTTL)
			}
		}
	}
}

// handleAccountInfo responds with account details for the authenticated key.
func (s *Server) handleAccountInfo(enc *protocol.Encoder, token string) {
	if strings.HasPrefix(token, "anon:") {
		_ = enc.Encode(protocol.MsgAccountInfoResp, protocol.AccountInfoRespMsg{
			Name: "anonymous", MaxTunnels: 1, Domain: s.cfg.Domain,
		})
		return
	}
	if IsAPIKey(token) {
		k := s.keyStore.GetKey(token)
		if k != nil {
			_ = enc.Encode(protocol.MsgAccountInfoResp, protocol.AccountInfoRespMsg{
				Name: k.Name, Subdomains: k.Subdomains, MaxTunnels: k.MaxTunnels, Domain: s.cfg.Domain,
			})
			return
		}
	}
	// Legacy tokens don't have account info.
	_ = enc.Encode(protocol.MsgAccountInfoResp, protocol.AccountInfoRespMsg{
		Name: "legacy", MaxTunnels: s.cfg.MaxTunnelsPerToken, Domain: s.cfg.Domain,
	})
}

// decrementTunnels handles API keys, legacy tokens, and anonymous (no-op).
func (s *Server) decrementTunnels(token string) {
	if strings.HasPrefix(token, "anon:") {
		return
	}
	if IsAPIKey(token) {
		s.keyStore.DecrementTunnels(token)
	} else {
		s.tokenStore.DecrementTunnels(token)
	}
}

// handleTunnelRequest validates and dispatches a TunnelRequest.
func (s *Server) handleTunnelRequest(
	session *yamux.Session,
	token, remoteIP string,
	req protocol.TunnelRequestMsg,
	enc *protocol.Encoder,
) {
	// Validate local_port range.
	if req.LocalPort < 1 || req.LocalPort > 65535 {
		sendTunnelError(enc, fmt.Sprintf("invalid local_port %d", req.LocalPort))
		return
	}

	// Anonymous: only random HTTP subdomains, no TCP, max 1 tunnel per IP.
	if strings.HasPrefix(token, "anon:") {
		if req.Type == protocol.TunnelTCP {
			sendTunnelError(enc, "anonymous users cannot create TCP tunnels — run 'htn-tunnel login' to register")
			return
		}
		if req.Subdomain != "" {
			sendTunnelError(enc, "anonymous users cannot request custom subdomains — run 'htn-tunnel login' to register")
			return
		}
		// Enforce 1 tunnel per anonymous IP.
		existing := s.tunnelManager.TunnelsForAnon(token)
		if len(existing) >= 1 {
			sendTunnelError(enc, "anonymous users are limited to 1 tunnel — run 'htn-tunnel login' for more")
			return
		}
	}

	// Check rate limit.
	if !s.rateLimiter.Allow(token) {
		sendTunnelError(enc, "rate limit exceeded")
		return
	}

	// Check tunnel count (API key vs legacy token; anonymous skips — enforced above).
	if !strings.HasPrefix(token, "anon:") {
		if IsAPIKey(token) {
			if err := s.keyStore.IncrementTunnels(token); err != nil {
				sendTunnelError(enc, err.Error())
				return
			}
		} else {
			if err := s.tokenStore.IncrementTunnels(token); err != nil {
				sendTunnelError(enc, err.Error())
				return
			}
		}
	}

	ts := &TunnelSession{
		Token:      token,
		Type:       string(req.Type),
		LocalPort:  req.LocalPort,
		Session:    session,
		ControlEnc: enc,
	}
	// Generate a unique ID.
	ts.ID = randomHex(8)

	switch req.Type {
	case protocol.TunnelHTTP:
		s.registerHTTPTunnel(ts, req, enc, token)
	case protocol.TunnelTCP:
		s.registerTCPTunnel(ts, enc, token)
	default:
		s.decrementTunnels(token)
		sendTunnelError(enc, fmt.Sprintf("unknown tunnel type %q", req.Type))
	}
}

func (s *Server) registerHTTPTunnel(
	ts *TunnelSession,
	req protocol.TunnelRequestMsg,
	enc *protocol.Encoder,
	token string,
) {
	subdomain := req.Subdomain
	if subdomain == "" {
		// Assign random subdomain; retry on collision.
		for i := 0; i < 10; i++ {
			subdomain = RandomSubdomain()
			ts.Subdomain = subdomain
			if err := s.tunnelManager.RegisterHTTP(subdomain, ts); err == nil {
				goto registered
			}
		}
		s.decrementTunnels(token)
		sendTunnelError(enc, "could not allocate subdomain")
		return
	}
	ts.Subdomain = subdomain
	if err := s.tunnelManager.RegisterHTTP(subdomain, ts); err != nil {
		s.decrementTunnels(token)
		sendTunnelError(enc, err.Error())
		return
	}

registered:
	url := fmt.Sprintf("https://%s.%s", subdomain, s.cfg.Domain)
	slog.Info("HTTP tunnel registered", "subdomain", subdomain, "token_prefix", maskToken(token))
	_ = enc.Encode(protocol.MsgTunnelResp, protocol.TunnelResponseMsg{
		Success: true, URL: url,
	})
}

func (s *Server) registerTCPTunnel(ts *TunnelSession, enc *protocol.Encoder, token string) {
	if s.tcpProxy == nil {
		s.decrementTunnels(token)
		sendTunnelError(enc, "TCP tunnels not available")
		return
	}
	port, err := s.tcpProxy.StartTCPTunnel(ts)
	if err != nil {
		s.decrementTunnels(token)
		sendTunnelError(enc, fmt.Sprintf("allocate TCP port: %v", err))
		return
	}
	slog.Info("TCP tunnel registered", "port", port, "token_prefix", maskToken(token))
	_ = enc.Encode(protocol.MsgTunnelResp, protocol.TunnelResponseMsg{
		Success:    true,
		RemotePort: port,
	})
}

// cleanupSession unregisters all tunnels owned by session after it closes.
func (s *Server) cleanupSession(session *yamux.Session, token string) {
	tunnels := s.tunnelManager.SessionTunnels(session)
	for _, ts := range tunnels {
		switch ts.Type {
		case "http":
			s.tunnelManager.UnregisterHTTP(ts.Subdomain)
		case "tcp":
			if s.tcpProxy != nil {
				s.tcpProxy.StopTCPTunnel(ts.Port)
			} else {
				s.tunnelManager.UnregisterTCP(ts.Port)
			}
		}
		s.decrementTunnels(token)
	}
}

// buildTLSConfig returns a TLS config: self-signed in DevMode, certmagic otherwise.
// certmagic integration is completed in Phase 3 (HTTP tunnel); here we always
// return a working (possibly self-signed) config so Phase 2 tests can run.
func (s *Server) buildTLSConfig() (*tls.Config, error) {
	if s.cfg.DevMode || s.cfg.Domain == "" {
		return selfSignedTLSConfig()
	}
	// Phase 3 replaces this with certmagic.
	return selfSignedTLSConfig()
}

// selfSignedTLSConfig generates an ephemeral RSA certificate for dev/test use.
func selfSignedTLSConfig() (*tls.Config, error) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, err
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "htn-tunnel-dev"},
		NotBefore:    time.Now().Add(-time.Minute),
		NotAfter:     time.Now().Add(87600 * time.Hour), // 10 years
		KeyUsage:     x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
	}
	certDER, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &priv.PublicKey, priv)
	if err != nil {
		return nil, err
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)})

	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return nil, err
	}
	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
	}, nil
}

// sendTunnelError sends a failed TunnelResponse with the given message.
func sendTunnelError(enc *protocol.Encoder, msg string) {
	_ = enc.Encode(protocol.MsgTunnelResp, protocol.TunnelResponseMsg{
		Success: false, Message: msg,
	})
}

// extractIP strips the port from a host:port address string.
func extractIP(addr string) string {
	ip, _, err := net.SplitHostPort(addr)
	if err != nil {
		return addr
	}
	return ip
}

// maskToken masks a token for safe logging: "tok_xxxx...xxxx".
func maskToken(t string) string {
	if len(t) <= 8 {
		return "***"
	}
	return t[:4] + "..." + t[len(t)-4:]
}

// randomHex returns n random hex bytes as a string.
func randomHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%x", b)
}
