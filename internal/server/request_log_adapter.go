// Package server — adapter that bridges server.RequestLog to dashboard.RequestLogProvider.
// Keeps internal/dashboard free of internal/server imports (circular-import prevention).
package server

import (
	"sync"

	"github.com/nhh0718/htn-tunnel/internal/dashboard"
)

// RequestLogAdapter wraps RequestLog and satisfies dashboard.RequestLogProvider,
// converting between server-internal types and the mirrored dashboard types.
// It tracks the mapping from dashboard channels to the underlying server channels
// so Unsubscribe can properly clean up.
type RequestLogAdapter struct {
	rl  *RequestLog
	mu  sync.Mutex
	// dst → src mapping for clean unsubscription
	srcOf map[chan dashboard.RequestLogEntry]chan LogEntry
}

// NewRequestLogAdapter creates an adapter around rl.
func NewRequestLogAdapter(rl *RequestLog) *RequestLogAdapter {
	return &RequestLogAdapter{
		rl:    rl,
		srcOf: make(map[chan dashboard.RequestLogEntry]chan LogEntry),
	}
}

func (a *RequestLogAdapter) Recent(limit int, token string) []dashboard.RequestLogEntry {
	src := a.rl.Recent(limit, token)
	out := make([]dashboard.RequestLogEntry, len(src))
	for i, e := range src {
		out[i] = toDBEntry(e)
	}
	return out
}

func (a *RequestLogAdapter) TrafficStats(minutes int, token string) []dashboard.RequestLogBucket {
	src := a.rl.TrafficStats(minutes, token)
	out := make([]dashboard.RequestLogBucket, len(src))
	for i, b := range src {
		out[i] = dashboard.RequestLogBucket{
			Timestamp:  b.Timestamp,
			Requests:   b.Requests,
			BytesIn:    b.BytesIn,
			BytesOut:   b.BytesOut,
			Status2xx:  b.Status2xx,
			Status3xx:  b.Status3xx,
			Status4xx:  b.Status4xx,
			Status5xx:  b.Status5xx,
			AvgLatency: b.AvgLatency,
		}
	}
	return out
}

func (a *RequestLogAdapter) TopPaths(n int, token string) []dashboard.RequestLogPathCount {
	src := a.rl.TopPaths(n, token)
	out := make([]dashboard.RequestLogPathCount, len(src))
	for i, p := range src {
		out[i] = dashboard.RequestLogPathCount{Path: p.Path, Count: p.Count}
	}
	return out
}

// Subscribe creates a dashboard-typed channel backed by the underlying RequestLog
// subscription. A forwarding goroutine converts LogEntry → RequestLogEntry.
// Call Unsubscribe(ch) to stop the goroutine and clean up resources.
func (a *RequestLogAdapter) Subscribe() chan dashboard.RequestLogEntry {
	src := a.rl.Subscribe()
	dst := make(chan dashboard.RequestLogEntry, 64)

	a.mu.Lock()
	a.srcOf[dst] = src
	a.mu.Unlock()

	go func() {
		for e := range src {
			select {
			case dst <- toDBEntry(e):
			default: // slow consumer; drop
			}
		}
		// src was closed by Unsubscribe → close dst to signal the SSE handler.
		close(dst)
	}()

	return dst
}

// Unsubscribe removes the subscription from the underlying RequestLog and lets
// the forwarding goroutine exit naturally (src is closed by rl.Unsubscribe).
func (a *RequestLogAdapter) Unsubscribe(ch chan dashboard.RequestLogEntry) {
	a.mu.Lock()
	src, ok := a.srcOf[ch]
	if ok {
		delete(a.srcOf, ch)
	}
	a.mu.Unlock()

	if ok {
		// Closing src via rl.Unsubscribe causes the forwarding goroutine to exit,
		// which then closes dst (ch). Do not double-close ch here.
		a.rl.Unsubscribe(src)
	}
}

// toDBEntry converts a server-internal LogEntry to a dashboard.RequestLogEntry.
func toDBEntry(e LogEntry) dashboard.RequestLogEntry {
	return dashboard.RequestLogEntry{
		Timestamp:  e.Timestamp,
		TunnelID:   e.TunnelID,
		Subdomain:  e.Subdomain,
		Token:      e.Token,
		Method:     e.Method,
		Path:       e.Path,
		Status:     e.Status,
		DurationMs: e.DurationMs,
		Size:       e.Size,
	}
}
