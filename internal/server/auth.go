package server

import (
	"fmt"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
	"golang.org/x/time/rate"
)

// tokenInfo holds the bcrypt hash and live tunnel count for one token.
type tokenInfo struct {
	hash       []byte // bcrypt hash of the raw token
	tunnelCount int
	maxTunnels int
}

// TokenStore validates auth tokens and tracks per-token tunnel counts.
// Raw tokens are bcrypt-hashed on load; only hashes are kept in memory.
type TokenStore struct {
	mu     sync.RWMutex
	tokens map[string]*tokenInfo // keyed by raw token for O(1) lookup
}

// NewTokenStore hashes each raw token and builds the store.
func NewTokenStore(rawTokens []string, maxTunnelsPerToken int) (*TokenStore, error) {
	ts := &TokenStore{
		tokens: make(map[string]*tokenInfo, len(rawTokens)),
	}
	for _, raw := range rawTokens {
		if raw == "" {
			continue
		}
		hash, err := bcrypt.GenerateFromPassword([]byte(raw), bcrypt.DefaultCost)
		if err != nil {
			return nil, fmt.Errorf("hash token: %w", err)
		}
		ts.tokens[raw] = &tokenInfo{
			hash:       hash,
			maxTunnels: maxTunnelsPerToken,
		}
	}
	return ts, nil
}

// Validate returns true if token is known and its bcrypt hash matches.
func (ts *TokenStore) Validate(token string) bool {
	ts.mu.RLock()
	info, ok := ts.tokens[token]
	ts.mu.RUnlock()
	if !ok {
		return false
	}
	return bcrypt.CompareHashAndPassword(info.hash, []byte(token)) == nil
}

// IncrementTunnels atomically increments the tunnel count for token.
// Returns an error if the token has reached its tunnel limit.
func (ts *TokenStore) IncrementTunnels(token string) error {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	info, ok := ts.tokens[token]
	if !ok {
		return fmt.Errorf("unknown token")
	}
	if info.tunnelCount >= info.maxTunnels {
		return fmt.Errorf("tunnel limit reached (%d/%d)", info.tunnelCount, info.maxTunnels)
	}
	info.tunnelCount++
	return nil
}

// DecrementTunnels decrements the tunnel count for token (floor 0).
func (ts *TokenStore) DecrementTunnels(token string) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	if info, ok := ts.tokens[token]; ok && info.tunnelCount > 0 {
		info.tunnelCount--
	}
}

// ipRateLimiter holds per-IP rate limiters for the pre-auth connection phase.
// Limits unauthenticated connection attempts to prevent DoS floods.
type ipRateLimiter struct {
	mu       sync.Mutex
	limiters map[string]*rateLimiterEntry
	rate     rate.Limit // connections per minute
	burst    int
}

type rateLimiterEntry struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// newIPRateLimiter creates an ipRateLimiter allowing r connections/min per IP.
func newIPRateLimiter(connPerMin int) *ipRateLimiter {
	r := &ipRateLimiter{
		limiters: make(map[string]*rateLimiterEntry),
		rate:     rate.Limit(float64(connPerMin) / 60.0),
		burst:    connPerMin, // burst = full minute's allowance
	}
	go r.cleanup()
	return r
}

// Allow returns true if the given IP is within the pre-auth rate limit.
func (r *ipRateLimiter) Allow(ip string) bool {
	r.mu.Lock()
	entry, ok := r.limiters[ip]
	if !ok {
		entry = &rateLimiterEntry{
			limiter: rate.NewLimiter(r.rate, r.burst),
		}
		r.limiters[ip] = entry
	}
	entry.lastSeen = time.Now()
	allowed := entry.limiter.Allow()
	r.mu.Unlock()
	return allowed
}

// cleanup removes stale entries every 5 minutes.
func (r *ipRateLimiter) cleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		cutoff := time.Now().Add(-10 * time.Minute)
		r.mu.Lock()
		for ip, entry := range r.limiters {
			if entry.lastSeen.Before(cutoff) {
				delete(r.limiters, ip)
			}
		}
		r.mu.Unlock()
	}
}

// RateLimiter enforces per-token and global rate limits on tunnel requests.
type RateLimiter struct {
	mu     sync.Mutex
	perTok map[string]*rate.Limiter
	global *rate.Limiter
	tokR   rate.Limit
	tokB   int
}

// NewRateLimiter creates a RateLimiter with per-token and global limits (req/min).
func NewRateLimiter(perTokenPerMin, globalPerMin int) *RateLimiter {
	return &RateLimiter{
		perTok: make(map[string]*rate.Limiter),
		global: rate.NewLimiter(rate.Limit(float64(globalPerMin)/60.0), globalPerMin),
		tokR:   rate.Limit(float64(perTokenPerMin) / 60.0),
		tokB:   perTokenPerMin,
	}
}

// Allow returns true if both the per-token and global limits permit the request.
func (rl *RateLimiter) Allow(token string) bool {
	if !rl.global.Allow() {
		return false
	}
	rl.mu.Lock()
	lim, ok := rl.perTok[token]
	if !ok {
		lim = rate.NewLimiter(rl.tokR, rl.tokB)
		rl.perTok[token] = lim
	}
	rl.mu.Unlock()
	return lim.Allow()
}
