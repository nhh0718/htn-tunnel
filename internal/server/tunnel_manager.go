package server

import (
	"fmt"
	"math/rand"
	"net"
	"regexp"
	"sync"
	"time"

	"github.com/hashicorp/yamux"
	"github.com/htn-sys/htn-tunnel/internal/dashboard"
)

// subdomainRe validates subdomain labels: lowercase alphanumeric + hyphens,
// must start and end with alphanumeric, 3-63 chars.
var subdomainRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,61}[a-z0-9]$`)

// reservedSubdomains cannot be claimed by clients.
var reservedSubdomains = map[string]bool{
	"www": true, "api": true, "admin": true, "mail": true,
	"ftp": true, "localhost": true, "tunnel": true, "dashboard": true,
}

// TunnelSession represents one active or reserved tunnel.
type TunnelSession struct {
	ID         string
	Token      string          // owning auth token
	Type       string          // "http" or "tcp"
	Subdomain  string          // HTTP tunnels only
	Port       int             // TCP tunnels: server-allocated port
	LocalPort  int             // client's local port
	Session    *yamux.Session  // yamux session to client
	Listener   net.Listener    // TCP tunnels: port listener
	CreatedAt  time.Time
	BytesIn    int64           // atomic via sync/atomic in proxy wrappers
	BytesOut   int64

	// Reservation fields: set when the client disconnects but the subdomain/port
	// is held for reconnect (60 s TTL).
	Reserved      bool
	ReservedUntil time.Time
}

// TunnelManager is the central registry of all active and reserved tunnels.
// HTTP tunnels are indexed by subdomain; TCP tunnels by allocated port.
type TunnelManager struct {
	mu          sync.RWMutex
	httpTunnels map[string]*TunnelSession // subdomain → session
	tcpTunnels  map[int]*TunnelSession    // port → session
	portMin     int
	portMax     int
}

// NewTunnelManager creates a TunnelManager with the given TCP port allocation range.
func NewTunnelManager(portMin, portMax int) *TunnelManager {
	tm := &TunnelManager{
		httpTunnels: make(map[string]*TunnelSession),
		tcpTunnels:  make(map[int]*TunnelSession),
		portMin:     portMin,
		portMax:     portMax,
	}
	go tm.reservationReaper()
	return tm
}

// ValidateSubdomain checks format and reserved-name constraints.
func ValidateSubdomain(sub string) error {
	if len(sub) < 3 || len(sub) > 63 {
		return fmt.Errorf("subdomain %q: length must be 3-63 chars", sub)
	}
	if !subdomainRe.MatchString(sub) {
		return fmt.Errorf("subdomain %q: must be lowercase alphanumeric and hyphens, cannot start/end with hyphen", sub)
	}
	if reservedSubdomains[sub] {
		return fmt.Errorf("subdomain %q is reserved", sub)
	}
	return nil
}

// RandomSubdomain generates an 8-char lowercase alphanumeric random subdomain.
func RandomSubdomain() string {
	const chars = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, 8)
	for i := range b {
		b[i] = chars[rand.Intn(len(chars))]
	}
	return string(b)
}

// RegisterHTTP registers an HTTP tunnel under subdomain.
// Atomic check-and-reserve: if the subdomain is reserved by the same token,
// the reservation is reclaimed; different-token reservation → error.
func (tm *TunnelManager) RegisterHTTP(subdomain string, session *TunnelSession) error {
	if err := ValidateSubdomain(subdomain); err != nil {
		return err
	}

	tm.mu.Lock()
	defer tm.mu.Unlock()

	existing, taken := tm.httpTunnels[subdomain]
	if taken {
		if existing.Reserved && existing.Token == session.Token &&
			time.Now().Before(existing.ReservedUntil) {
			// Reclaim own reservation.
			session.CreatedAt = time.Now()
			tm.httpTunnels[subdomain] = session
			return nil
		}
		return fmt.Errorf("subdomain %q is already in use", subdomain)
	}
	session.CreatedAt = time.Now()
	tm.httpTunnels[subdomain] = session
	return nil
}

// RegisterTCP allocates a random port, starts a TCP listener, and stores the
// tunnel. Returns the allocated port number.
func (tm *TunnelManager) RegisterTCP(session *TunnelSession) (int, error) {
	const maxAttempts = 100
	for attempt := 0; attempt < maxAttempts; attempt++ {
		port := tm.portMin + rand.Intn(tm.portMax-tm.portMin+1)

		tm.mu.Lock()
		_, taken := tm.tcpTunnels[port]
		tm.mu.Unlock()
		if taken {
			continue
		}

		ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
		if err != nil {
			continue // port busy at OS level
		}

		session.Port = port
		session.Listener = ln
		session.CreatedAt = time.Now()

		tm.mu.Lock()
		// Re-check under lock to avoid TOCTOU race.
		if _, taken = tm.tcpTunnels[port]; taken {
			tm.mu.Unlock()
			ln.Close()
			continue
		}
		tm.tcpTunnels[port] = session
		tm.mu.Unlock()
		return port, nil
	}
	return 0, fmt.Errorf("unable to allocate TCP port after %d attempts", maxAttempts)
}

// UnregisterHTTP removes the HTTP tunnel for subdomain immediately.
func (tm *TunnelManager) UnregisterHTTP(subdomain string) {
	tm.mu.Lock()
	delete(tm.httpTunnels, subdomain)
	tm.mu.Unlock()
}

// ReserveHTTP marks the subdomain as reserved (held for 60 s after disconnect).
func (tm *TunnelManager) ReserveHTTP(subdomain string, ttl time.Duration) {
	tm.mu.Lock()
	if s, ok := tm.httpTunnels[subdomain]; ok {
		s.Reserved = true
		s.ReservedUntil = time.Now().Add(ttl)
	}
	tm.mu.Unlock()
}

// UnregisterTCP removes the TCP tunnel for port and closes its listener.
func (tm *TunnelManager) UnregisterTCP(port int) {
	tm.mu.Lock()
	s, ok := tm.tcpTunnels[port]
	delete(tm.tcpTunnels, port)
	tm.mu.Unlock()
	if ok && s.Listener != nil {
		s.Listener.Close()
	}
}

// LookupHTTP returns the active (non-reserved) TunnelSession for subdomain, or nil.
func (tm *TunnelManager) LookupHTTP(subdomain string) *TunnelSession {
	tm.mu.RLock()
	s := tm.httpTunnels[subdomain]
	tm.mu.RUnlock()
	if s == nil || s.Reserved {
		return nil
	}
	return s
}

// LookupTCP returns the TunnelSession for port, or nil.
func (tm *TunnelManager) LookupTCP(port int) *TunnelSession {
	tm.mu.RLock()
	s := tm.tcpTunnels[port]
	tm.mu.RUnlock()
	return s
}

// SessionTunnels returns all tunnel sessions owned by the given yamux session.
func (tm *TunnelManager) SessionTunnels(ySession *yamux.Session) []*TunnelSession {
	var out []*TunnelSession
	tm.mu.RLock()
	for _, s := range tm.httpTunnels {
		if s.Session == ySession {
			out = append(out, s)
		}
	}
	for _, s := range tm.tcpTunnels {
		if s.Session == ySession {
			out = append(out, s)
		}
	}
	tm.mu.RUnlock()
	return out
}

// reservationReaper periodically removes expired subdomain reservations.
// Runs under the lock to prevent races with concurrent reclaim attempts.
func (tm *TunnelManager) reservationReaper() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		now := time.Now()
		tm.mu.Lock()
		for sub, s := range tm.httpTunnels {
			if s.Reserved && now.After(s.ReservedUntil) {
				delete(tm.httpTunnels, sub)
			}
		}
		tm.mu.Unlock()
	}
}

// DashboardStats holds aggregate tunnel statistics for the dashboard.
type DashboardStats struct {
	ActiveHTTP   int   `json:"active_http"`
	ActiveTCP    int   `json:"active_tcp"`
	TotalTunnels int   `json:"total_tunnels"`
	BytesIn      int64 `json:"bytes_in"`
	BytesOut     int64 `json:"bytes_out"`
}

// Stats returns aggregate statistics across all active tunnels.
func (tm *TunnelManager) Stats() DashboardStats {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	var s DashboardStats
	for _, t := range tm.httpTunnels {
		if !t.Reserved {
			s.ActiveHTTP++
			s.BytesIn += t.BytesIn
			s.BytesOut += t.BytesOut
		}
	}
	for _, t := range tm.tcpTunnels {
		s.ActiveTCP++
		s.BytesIn += t.BytesIn
		s.BytesOut += t.BytesOut
	}
	s.TotalTunnels = s.ActiveHTTP + s.ActiveTCP
	return s
}

// ListTunnels returns a snapshot of all active tunnel sessions.
func (tm *TunnelManager) ListTunnels() []*TunnelSession {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	out := make([]*TunnelSession, 0, len(tm.httpTunnels)+len(tm.tcpTunnels))
	for _, s := range tm.httpTunnels {
		if !s.Reserved {
			out = append(out, s)
		}
	}
	for _, s := range tm.tcpTunnels {
		out = append(out, s)
	}
	return out
}

// ListTunnelsForDashboard satisfies dashboard.TunnelProvider.
func (tm *TunnelManager) ListTunnelsForDashboard() []dashboard.TunnelStats {
	all := tm.ListTunnels()
	out := make([]dashboard.TunnelStats, 0, len(all))
	for _, t := range all {
		uptime := time.Since(t.CreatedAt).Truncate(time.Second).String()
		out = append(out, dashboard.TunnelStats{
			ID:        t.ID,
			Type:      t.Type,
			Subdomain: t.Subdomain,
			Port:      t.Port,
			LocalPort: t.LocalPort,
			Token:     maskToken(t.Token),
			Uptime:    uptime,
			BytesIn:   t.BytesIn,
			BytesOut:  t.BytesOut,
			CreatedAt: t.CreatedAt,
		})
	}
	return out
}

// StatsForDashboard satisfies dashboard.TunnelProvider.
func (tm *TunnelManager) StatsForDashboard() dashboard.AggregateStats {
	s := tm.Stats()
	return dashboard.AggregateStats{
		ActiveHTTP:   s.ActiveHTTP,
		ActiveTCP:    s.ActiveTCP,
		TotalTunnels: s.TotalTunnels,
		BytesIn:      s.BytesIn,
		BytesOut:     s.BytesOut,
	}
}

// KillTunnelByID closes the tunnel with the given ID. Returns false if not found.
func (tm *TunnelManager) KillTunnelByID(id string) bool {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	for sub, s := range tm.httpTunnels {
		if s.ID == id {
			if s.Session != nil {
				s.Session.Close()
			}
			delete(tm.httpTunnels, sub)
			return true
		}
	}
	for port, s := range tm.tcpTunnels {
		if s.ID == id {
			if s.Listener != nil {
				s.Listener.Close()
			}
			if s.Session != nil {
				s.Session.Close()
			}
			delete(tm.tcpTunnels, port)
			return true
		}
	}
	return false
}
