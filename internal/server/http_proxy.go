package server

import (
	"bufio"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httputil"
	"path/filepath"
	"strings"
	"time"

	"github.com/caddyserver/certmagic"
	"github.com/libdns/cloudflare"
	"github.com/nhh0718/htn-tunnel/internal/config"
	"github.com/nhh0718/htn-tunnel/internal/protocol"
)

// HTTPProxy handles TLS termination, SNI-based subdomain routing, and HTTP
// reverse proxying through yamux streams to the client.
type HTTPProxy struct {
	cfg           *config.ServerConfig
	tunnelManager *TunnelManager
	requestLog    *RequestLog
	tlsListener   net.Listener
	httpListener  net.Listener
}

// NewHTTPProxy creates an HTTPProxy backed by the given TunnelManager.
// rl may be nil; if provided, every proxied request is recorded in it.
func NewHTTPProxy(cfg *config.ServerConfig, tm *TunnelManager, rl *RequestLog) *HTTPProxy {
	return &HTTPProxy{cfg: cfg, tunnelManager: tm, requestLog: rl}
}

// Start opens the TLS listener on :443 and the plain HTTP listener on :80.
// Blocks until ctx is cancelled.
func (p *HTTPProxy) Start(ctx context.Context) error {
	tlsCfg, err := p.buildTLSConfig()
	if err != nil {
		return fmt.Errorf("HTTPProxy TLS config: %w", err)
	}

	p.tlsListener, err = tls.Listen("tcp", p.cfg.HTTPProxyAddr, tlsCfg)
	if err != nil {
		return fmt.Errorf("HTTPProxy TLS listen %s: %w", p.cfg.HTTPProxyAddr, err)
	}

	p.httpListener, err = net.Listen("tcp", p.cfg.HTTPRedirectAddr)
	if err != nil {
		p.tlsListener.Close()
		return fmt.Errorf("HTTPProxy HTTP listen %s: %w", p.cfg.HTTPRedirectAddr, err)
	}

	slog.Info("HTTP proxy listening", "tls", p.cfg.HTTPProxyAddr, "plain", p.cfg.HTTPRedirectAddr, "domain", p.cfg.Domain)

	// HTTP/80: redirect to HTTPS
	go p.serveHTTPRedirect(ctx)

	// HTTPS/443: SNI routing + reverse proxy
	go p.serveTLS(ctx)

	<-ctx.Done()
	p.tlsListener.Close()
	p.httpListener.Close()
	return nil
}

// serveHTTPRedirect sends 301 redirects from port 80 to HTTPS.
func (p *HTTPProxy) serveHTTPRedirect(ctx context.Context) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		target := "https://" + r.Host + r.URL.RequestURI()
		http.Redirect(w, r, target, http.StatusMovedPermanently)
	})
	srv := &http.Server{Handler: mux}
	go func() { <-ctx.Done(); srv.Close() }()
	_ = srv.Serve(p.httpListener)
}

// serveTLS accepts HTTPS connections and proxies them to the correct tunnel.
func (p *HTTPProxy) serveTLS(ctx context.Context) {
	for {
		conn, err := p.tlsListener.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				return
			default:
				slog.Error("HTTPProxy accept error", "err", err)
				continue
			}
		}
		go p.handleTLSConn(conn)
	}
}

// handleTLSConn serves one HTTPS connection: extracts subdomain, looks up
// tunnel, and reverse-proxies the request.
func (p *HTTPProxy) handleTLSConn(conn net.Conn) {
	defer conn.Close()

	tlsConn, ok := conn.(*tls.Conn)
	if !ok {
		return
	}

	// Complete TLS handshake so we can read the HTTP request.
	if err := tlsConn.SetDeadline(time.Now().Add(10 * time.Second)); err != nil {
		return
	}
	if err := tlsConn.Handshake(); err != nil {
		return
	}
	tlsConn.SetDeadline(time.Time{})

	// SNI provides the subdomain before HTTP request parsing.
	subdomain := p.extractSubdomain(tlsConn.ConnectionState().ServerName)

	// Serve the connection as HTTP with our custom handler.
	httpSrv := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Fallback: derive subdomain from Host header if SNI was empty.
			sub := subdomain
			if sub == "" {
				sub = p.extractSubdomain(r.Host)
			}
			p.serveRequest(w, r, sub)
		}),
	}
	_ = httpSrv.Serve(newSingleConnListener(tlsConn))
}

// serveRequest proxies one HTTP request to the tunnel for subdomain.
func (p *HTTPProxy) serveRequest(w http.ResponseWriter, r *http.Request, subdomain string) {
	ts := p.tunnelManager.LookupHTTP(subdomain)
	if ts == nil {
		http.Error(w, "tunnel not found for subdomain: "+subdomain, http.StatusBadGateway)
		return
	}

	slog.Debug("request", "subdomain", subdomain, "path", r.URL.Path, "upgrade", r.Header.Get("Upgrade"), "connection", r.Header.Get("Connection"))

	// Detect WebSocket upgrade — must bypass httputil.ReverseProxy.
	if isWebSocketUpgrade(r) {
		slog.Info("WebSocket upgrade detected", "subdomain", subdomain, "path", r.URL.Path)
		p.proxyWebSocket(w, r, ts)
		return
	}

	// Wrap response writer to capture status code and size for request logging.
	rw := &loggingResponseWriter{ResponseWriter: w}
	start := time.Now()

	// Standard HTTP reverse proxy via yamux stream.
	proxy := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL.Scheme = "http"
			req.URL.Host = fmt.Sprintf("localhost:%d", ts.LocalPort)
			addForwardingHeaders(req, r)
		},
		Transport: &yamuxTransport{session: ts},
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			slog.Warn("proxy error", "subdomain", subdomain, "err", err)
			http.Error(w, "tunnel error: "+err.Error(), http.StatusBadGateway)
		},
	}
	proxy.ServeHTTP(rw, r)

	dur := time.Since(start)
	// Send request log to client via control stream.
	sendRequestLog(ts, r.Method, r.URL.Path, rw.status, dur, rw.size)

	// Store in in-memory log for dashboard analytics.
	if p.requestLog != nil {
		p.requestLog.Add(LogEntry{
			Timestamp:  start,
			TunnelID:   ts.ID,
			Subdomain:  ts.Subdomain,
			Token:      maskToken(ts.Token),
			Method:     r.Method,
			Path:       r.URL.Path,
			Status:     rw.status,
			DurationMs: int(dur.Milliseconds()),
			Size:       rw.size,
		})
	}
}

// proxyWebSocket hijacks the HTTP connection and forwards raw bytes through
// a yamux stream for WebSocket and other upgrade protocols (e.g. HMR).
func (p *HTTPProxy) proxyWebSocket(w http.ResponseWriter, r *http.Request, ts *TunnelSession) {
	hj, ok := w.(http.Hijacker)
	if !ok {
		slog.Warn("WebSocket: hijack not supported")
		http.Error(w, "WebSocket proxy not supported", http.StatusInternalServerError)
		return
	}

	clientConn, bufrw, err := hj.Hijack()
	if err != nil {
		slog.Warn("WebSocket: hijack failed", "err", err)
		return
	}
	defer clientConn.Close()

	// Open a yamux stream to the client.
	stream, err := ts.Session.Open()
	if err != nil {
		slog.Warn("WebSocket: yamux open failed", "err", err)
		return
	}
	defer stream.Close()

	// Rewrite Origin header to match local server — dev servers (e.g. Next.js)
	// reject WebSocket upgrades when Origin doesn't match the local host.
	if origin := r.Header.Get("Origin"); origin != "" {
		r.Header.Set("Origin", fmt.Sprintf("http://localhost:%d", ts.LocalPort))
	}

	// Forward the original HTTP Upgrade request over the yamux stream.
	if err := r.Write(stream); err != nil {
		slog.Warn("WebSocket: write request to stream failed", "err", err)
		return
	}

	// Flush any buffered data from the hijacked connection.
	if bufrw.Reader.Buffered() > 0 {
		buffered := make([]byte, bufrw.Reader.Buffered())
		_, _ = bufrw.Read(buffered)
		_, _ = stream.Write(buffered)
	}

	// Read the server's upgrade response from the stream and forward to client.
	resp, err := http.ReadResponse(bufio.NewReader(stream), r)
	if err != nil {
		slog.Warn("WebSocket: read upgrade response failed", "err", err)
		return
	}
	slog.Info("WebSocket: upgrade response", "status", resp.StatusCode)
	if err := resp.Write(clientConn); err != nil {
		slog.Warn("WebSocket: write response to client failed", "err", err)
		return
	}

	slog.Info("WebSocket: proxying started", "path", r.URL.Path)
	_, _, _ = proxyBidirectional(clientConn, stream)
	slog.Info("WebSocket: proxying ended", "path", r.URL.Path)
}

// yamuxTransport implements http.RoundTripper by opening a yamux stream for
// each HTTP request and writing the request/reading the response directly.
type yamuxTransport struct {
	session *TunnelSession
}

func (t *yamuxTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	stream, err := t.session.Session.Open()
	if err != nil {
		return nil, fmt.Errorf("open yamux stream: %w", err)
	}

	// Write HTTP request to the stream.
	if err := req.Write(stream); err != nil {
		stream.Close()
		return nil, fmt.Errorf("write request: %w", err)
	}

	// Read HTTP response from the stream.
	resp, err := http.ReadResponse(bufio.NewReader(stream), req)
	if err != nil {
		stream.Close()
		return nil, fmt.Errorf("read response: %w", err)
	}

	// Wrap the response body so the stream closes after the body is consumed.
	resp.Body = &streamCloser{
		ReadCloser: resp.Body,
		stream:     stream,
		session:    t.session,
	}
	return resp, nil
}

// streamCloser closes the yamux stream and tracks bandwidth after the response body is read.
type streamCloser struct {
	io.ReadCloser
	stream  io.ReadCloser
	session *TunnelSession
	read    int64
}

func (sc *streamCloser) Read(p []byte) (int, error) {
	n, err := sc.ReadCloser.Read(p)
	sc.read += int64(n)
	return n, err
}

func (sc *streamCloser) Close() error {
	err := sc.ReadCloser.Close()
	sc.stream.Close()
	sc.session.BytesOut += sc.read
	return err
}

// addForwardingHeaders sets X-Forwarded-* headers on the outgoing request.
func addForwardingHeaders(out, orig *http.Request) {
	clientIP := extractIP(orig.RemoteAddr)
	if prior := orig.Header.Get("X-Forwarded-For"); prior != "" {
		out.Header.Set("X-Forwarded-For", prior+", "+clientIP)
	} else {
		out.Header.Set("X-Forwarded-For", clientIP)
	}
	out.Header.Set("X-Forwarded-Proto", "https")
	out.Header.Set("X-Forwarded-Host", orig.Host)

	// Remove hop-by-hop headers.
	for _, h := range hopByHopHeaders {
		out.Header.Del(h)
	}
}

var hopByHopHeaders = []string{
	"Connection", "Keep-Alive", "Proxy-Authenticate",
	"Proxy-Authorization", "TE", "Trailers", "Transfer-Encoding", "Upgrade",
}

// isWebSocketUpgrade returns true for HTTP Upgrade: websocket requests.
func isWebSocketUpgrade(r *http.Request) bool {
	return strings.EqualFold(r.Header.Get("Upgrade"), "websocket") &&
		strings.Contains(strings.ToLower(r.Header.Get("Connection")), "upgrade")
}

// extractSubdomain strips the base domain and returns the leftmost label.
// e.g. "abc.tunnel.example.com" with domain "tunnel.example.com" → "abc".
func (p *HTTPProxy) extractSubdomain(host string) string {
	// Strip port if present.
	h := host
	if idx := strings.LastIndex(h, ":"); idx >= 0 {
		h = h[:idx]
	}
	suffix := "." + p.cfg.Domain
	if !strings.HasSuffix(h, suffix) {
		// Might be just the first label (e.g. in tests).
		parts := strings.SplitN(h, ".", 2)
		if len(parts) >= 1 {
			return strings.ToLower(parts[0])
		}
		return ""
	}
	sub := strings.TrimSuffix(h, suffix)
	return strings.ToLower(sub)
}

// buildTLSConfig returns a certmagic TLS config (production) or self-signed (DevMode).
func (p *HTTPProxy) buildTLSConfig() (*tls.Config, error) {
	if p.cfg.DevMode || p.cfg.Domain == "" {
		return selfSignedTLSConfig()
	}
	return p.certmagicTLSConfig()
}

// certmagicTLSConfig configures certmagic with DNS-01 wildcard cert issuance.
// Uses certmagic for cert management (issue/renew) and falls back to loading
// the wildcard cert directly from disk if certmagic's GetCertificate fails.
func (p *HTTPProxy) certmagicTLSConfig() (*tls.Config, error) {
	if p.cfg.DNSAPIToken == "" {
		return nil, fmt.Errorf("dns_api_token is required for wildcard cert issuance (DNS-01 challenge)")
	}

	// Configure DefaultACME BEFORE calling NewDefault() so the DNS-01 solver
	// is included in the copy that NewDefault() creates.
	certmagic.DefaultACME.Email = p.cfg.Email
	certmagic.DefaultACME.Agreed = true

	switch strings.ToLower(p.cfg.DNSProvider) {
	case "cloudflare":
		certmagic.DefaultACME.DNS01Solver = &certmagic.DNS01Solver{
			DNSManager: certmagic.DNSManager{
				DNSProvider: &cloudflare.Provider{
					APIToken: p.cfg.DNSAPIToken,
				},
			},
		}
	default:
		return nil, fmt.Errorf("unsupported dns_provider %q; supported: cloudflare", p.cfg.DNSProvider)
	}

	magic := certmagic.NewDefault()
	magic.Storage = &certmagic.FileStorage{Path: p.cfg.CertStorage}

	domains := []string{p.cfg.Domain, "*." + p.cfg.Domain}
	if err := magic.ManageAsync(context.Background(), domains); err != nil {
		return nil, fmt.Errorf("certmagic manage domains: %w", err)
	}

	tlsCfg := magic.TLSConfig()
	// HTTP/1.1 only — h2 breaks WebSocket upgrade which tunnels rely on.
	tlsCfg.NextProtos = []string{"http/1.1"}

	// Load wildcard cert directly from disk as fallback for certmagic's GetCertificate.
	wildcardName := "wildcard_." + p.cfg.Domain
	certDir := filepath.Join(p.cfg.CertStorage, "certificates",
		"acme-v02.api.letsencrypt.org-directory", wildcardName)
	certPath := filepath.Join(certDir, wildcardName+".crt")
	keyPath := filepath.Join(certDir, wildcardName+".key")

	if cert, err := tls.LoadX509KeyPair(certPath, keyPath); err == nil {
		slog.Info("loaded wildcard cert from disk", "domain", p.cfg.Domain)
		origGetCert := tlsCfg.GetCertificate
		tlsCfg.GetCertificate = func(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
			// Try certmagic first.
			if origGetCert != nil {
				if c, e := origGetCert(hello); e == nil && c != nil {
					return c, nil
				}
			}
			// Fallback to disk-loaded wildcard cert.
			return &cert, nil
		}
	} else {
		slog.Warn("wildcard cert not on disk yet, relying on certmagic only", "err", err)
	}

	return tlsCfg, nil
}

// singleConnListener implements net.Listener for a single pre-accepted connection.
// Used to feed a *tls.Conn into http.Server.Serve().
type singleConnListener struct {
	conn net.Conn
	ch   chan net.Conn
}

func newSingleConnListener(conn net.Conn) *singleConnListener {
	ch := make(chan net.Conn, 1)
	ch <- conn
	return &singleConnListener{conn: conn, ch: ch}
}

func (l *singleConnListener) Accept() (net.Conn, error) {
	c, ok := <-l.ch
	if !ok {
		return nil, fmt.Errorf("listener closed")
	}
	return c, nil
}

func (l *singleConnListener) Close() error {
	select {
	case <-l.ch:
	default:
	}
	close(l.ch)
	return nil
}

func (l *singleConnListener) Addr() net.Addr { return l.conn.LocalAddr() }

// loggingResponseWriter wraps http.ResponseWriter to capture status code and response size.
type loggingResponseWriter struct {
	http.ResponseWriter
	status int
	size   int64
}

func (w *loggingResponseWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *loggingResponseWriter) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = 200
	}
	n, err := w.ResponseWriter.Write(b)
	w.size += int64(n)
	return n, err
}

func (w *loggingResponseWriter) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// sendRequestLog sends a request log message to the client's control stream.
func sendRequestLog(ts *TunnelSession, method, path string, status int, dur time.Duration, size int64) {
	if ts == nil || ts.ControlEnc == nil {
		return
	}
	if status == 0 {
		status = 502
	}
	// Truncate path for log message efficiency.
	if len(path) > 100 {
		path = path[:97] + "..."
	}
	_ = ts.ControlEnc.Encode(protocol.MsgRequestLog, protocol.RequestLogMsg{
		Method:   method,
		Path:     path,
		Status:   status,
		Duration: int(dur.Milliseconds()),
		Size:     size,
	})
}

