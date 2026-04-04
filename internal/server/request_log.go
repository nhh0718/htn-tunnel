// Package server — in-memory circular request log with pub/sub and analytics helpers.
package server

import (
	"sort"
	"sync"
	"time"
)

const maxLogEntries = 10000

// LogEntry is a single proxied-request record stored in RequestLog.
type LogEntry struct {
	Timestamp  time.Time `json:"ts"`
	TunnelID   string    `json:"tid"`
	Subdomain  string    `json:"sub"`
	Token      string    `json:"tok"`
	Method     string    `json:"m"`
	Path       string    `json:"p"`
	Status     int       `json:"s"`
	DurationMs int       `json:"d"`
	Size       int64     `json:"z"`
}

// TrafficBucket aggregates traffic statistics for one time bucket (per-minute).
type TrafficBucket struct {
	Timestamp  time.Time `json:"ts"`
	Requests   int       `json:"reqs"`
	BytesIn    int64     `json:"bytes_in"`
	BytesOut   int64     `json:"bytes_out"`
	Status2xx  int       `json:"s2xx"`
	Status3xx  int       `json:"s3xx"`
	Status4xx  int       `json:"s4xx"`
	Status5xx  int       `json:"s5xx"`
	AvgLatency int       `json:"avg_ms"`
}

// PathCount holds a path and its hit count, for top-paths analytics.
type PathCount struct {
	Path  string `json:"path"`
	Count int    `json:"count"`
}

// RequestLog is a thread-safe, bounded circular buffer of LogEntry values.
// It supports real-time subscribers and basic analytics queries.
type RequestLog struct {
	mu      sync.RWMutex
	entries []LogEntry
	pos     int  // next write index
	full    bool // true once buffer has wrapped at least once

	subMu sync.RWMutex
	subs  map[chan LogEntry]struct{}
}

// NewRequestLog allocates a RequestLog with capacity maxLogEntries.
func NewRequestLog() *RequestLog {
	return &RequestLog{
		entries: make([]LogEntry, maxLogEntries),
		subs:    make(map[chan LogEntry]struct{}),
	}
}

// Add stores e in the circular buffer and fans it out to all subscribers.
// Non-blocking: slow subscribers silently drop entries.
func (rl *RequestLog) Add(e LogEntry) {
	rl.mu.Lock()
	rl.entries[rl.pos] = e
	rl.pos++
	if rl.pos >= maxLogEntries {
		rl.pos = 0
		rl.full = true
	}
	rl.mu.Unlock()

	rl.subMu.RLock()
	for ch := range rl.subs {
		select {
		case ch <- e:
		default: // subscriber too slow; drop
		}
	}
	rl.subMu.RUnlock()
}

// Subscribe returns a buffered channel that receives each new LogEntry as it arrives.
// Call Unsubscribe when done to avoid goroutine/channel leaks.
func (rl *RequestLog) Subscribe() chan LogEntry {
	ch := make(chan LogEntry, 64)
	rl.subMu.Lock()
	rl.subs[ch] = struct{}{}
	rl.subMu.Unlock()
	return ch
}

// Unsubscribe removes ch from the subscriber set and closes it.
func (rl *RequestLog) Unsubscribe(ch chan LogEntry) {
	rl.subMu.Lock()
	delete(rl.subs, ch)
	rl.subMu.Unlock()
	close(ch)
}

// snapshot returns a read-consistent copy of all stored entries, newest first.
// If token != "", only entries whose Token matches are included.
func (rl *RequestLog) snapshot(token string) []LogEntry {
	rl.mu.RLock()
	size := maxLogEntries
	if !rl.full {
		size = rl.pos
	}
	// Copy to avoid holding the lock during filtering.
	raw := make([]LogEntry, size)
	if rl.full {
		// entries[rl.pos:] are older; entries[:rl.pos] are newer.
		n := copy(raw, rl.entries[rl.pos:])
		copy(raw[n:], rl.entries[:rl.pos])
	} else {
		copy(raw, rl.entries[:rl.pos])
	}
	rl.mu.RUnlock()

	// Reverse so index 0 is the newest entry.
	for i, j := 0, len(raw)-1; i < j; i, j = i+1, j-1 {
		raw[i], raw[j] = raw[j], raw[i]
	}

	if token == "" {
		return raw
	}
	filtered := raw[:0:0]
	for _, e := range raw {
		if e.Token == token {
			filtered = append(filtered, e)
		}
	}
	return filtered
}

// Recent returns up to limit entries, newest first.
// If token != "", only entries for that token are returned.
func (rl *RequestLog) Recent(limit int, token string) []LogEntry {
	all := rl.snapshot(token)
	if limit <= 0 || limit >= len(all) {
		return all
	}
	return all[:limit]
}

// TrafficStats returns per-minute traffic buckets for the last `minutes` minutes.
// If token != "", only entries for that token are counted.
func (rl *RequestLog) TrafficStats(minutes int, token string) []TrafficBucket {
	if minutes <= 0 {
		minutes = 30
	}
	now := time.Now().UTC()
	cutoff := now.Add(-time.Duration(minutes) * time.Minute)

	// Build bucket map keyed by minute offset from cutoff.
	type accumulator struct {
		reqs      int
		bytesOut  int64
		status2xx int
		status3xx int
		status4xx int
		status5xx int
		totalMs   int64
	}
	buckets := make(map[int]*accumulator, minutes)

	all := rl.snapshot(token)
	for _, e := range all {
		if e.Timestamp.Before(cutoff) {
			continue
		}
		// Which minute bucket? 0 = most recent complete minute.
		minuteIdx := int(now.Sub(e.Timestamp).Minutes())
		if minuteIdx < 0 {
			minuteIdx = 0
		}
		if minuteIdx >= minutes {
			continue
		}
		b := buckets[minuteIdx]
		if b == nil {
			b = &accumulator{}
			buckets[minuteIdx] = b
		}
		b.reqs++
		b.bytesOut += e.Size
		b.totalMs += int64(e.DurationMs)
		switch {
		case e.Status >= 500:
			b.status5xx++
		case e.Status >= 400:
			b.status4xx++
		case e.Status >= 300:
			b.status3xx++
		default:
			b.status2xx++
		}
	}

	// Convert to sorted slice (oldest → newest).
	result := make([]TrafficBucket, minutes)
	for i := 0; i < minutes; i++ {
		// minuteIdx 0 = most recent; (minutes-1) = oldest.
		ts := now.Add(-time.Duration(minutes-1-i) * time.Minute).Truncate(time.Minute)
		b := buckets[minutes-1-i]
		tb := TrafficBucket{Timestamp: ts}
		if b != nil {
			tb.Requests = b.reqs
			tb.BytesOut = b.bytesOut
			tb.Status2xx = b.status2xx
			tb.Status3xx = b.status3xx
			tb.Status4xx = b.status4xx
			tb.Status5xx = b.status5xx
			if b.reqs > 0 {
				tb.AvgLatency = int(b.totalMs / int64(b.reqs))
			}
		}
		result[i] = tb
	}
	return result
}

// TopPaths returns the top n most-hit paths, descending by hit count.
// If token != "", only entries for that token are counted.
func (rl *RequestLog) TopPaths(n int, token string) []PathCount {
	counts := make(map[string]int)
	all := rl.snapshot(token)
	for _, e := range all {
		counts[e.Path]++
	}
	out := make([]PathCount, 0, len(counts))
	for p, c := range counts {
		out = append(out, PathCount{Path: p, Count: c})
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Count > out[j].Count
	})
	if n > 0 && n < len(out) {
		return out[:n]
	}
	return out
}
