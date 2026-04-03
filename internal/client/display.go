package client

import (
	"fmt"
	"strings"
	"time"

	"github.com/nhh0718/htn-tunnel/internal/protocol"
)

// ANSI color codes.
const (
	reset  = "\033[0m"
	bold   = "\033[1m"
	green  = "\033[32m"
	yellow = "\033[33m"
	red    = "\033[31m"
	cyan   = "\033[36m"
	gray   = "\033[90m"
)

// PrintBox draws a status box with tunnel info.
func PrintBox(version, tunnel, forward, server, status string) {
	statusColor := green
	statusDot := "●"
	switch status {
	case "connecting":
		statusColor = yellow
		statusDot = "◌"
	case "disconnected":
		statusColor = red
		statusDot = "○"
	}

	lines := []string{
		fmt.Sprintf("  %shtn-tunnel%s v%s", bold, reset, version),
		"",
		fmt.Sprintf("  Tunnel:   %s%s%s", cyan, tunnel, reset),
		fmt.Sprintf("  Forward:  → %s", forward),
		fmt.Sprintf("  Server:   %s", server),
		fmt.Sprintf("  Status:   %s%s %s%s", statusColor, statusDot, status, reset),
	}

	maxLen := 0
	for _, l := range lines {
		vl := visibleLen(l)
		if vl > maxLen {
			maxLen = vl
		}
	}
	width := maxLen + 4

	fmt.Println()
	fmt.Printf("  ╭%s╮\n", strings.Repeat("─", width))
	for _, l := range lines {
		pad := width - visibleLen(l)
		fmt.Printf("  │%s%s│\n", l, strings.Repeat(" ", pad))
	}
	fmt.Printf("  ╰%s╯\n\n", strings.Repeat("─", width))
}

// PrintRequestLog formats and prints one request log entry.
func PrintRequestLog(log protocol.RequestLogMsg) {
	now := time.Now().Format("15:04:05")
	method := padRight(log.Method, 6)
	path := log.Path
	if len(path) > 40 {
		path = path[:37] + "..."
	}
	path = padRight(path, 40)

	statusColor := gray
	switch {
	case log.Status >= 500:
		statusColor = red
	case log.Status >= 400:
		statusColor = yellow
	case log.Status >= 300:
		statusColor = cyan
	case log.Status >= 200:
		statusColor = green
	}

	sizeStr := fmtBytes(log.Size)
	durStr := fmt.Sprintf("%dms", log.Duration)

	fmt.Printf("  %s%s%s  %s  %s  %s%d%s  %s%s  %s\n",
		gray, now, reset,
		method,
		path,
		statusColor, log.Status, reset,
		gray, padLeft(durStr, 6),
		padLeft(sizeStr, 8),
	)
}

func fmtBytes(b int64) string {
	switch {
	case b < 1024:
		return fmt.Sprintf("%dB", b)
	case b < 1048576:
		return fmt.Sprintf("%.1fKB", float64(b)/1024)
	default:
		return fmt.Sprintf("%.1fMB", float64(b)/1048576)
	}
}

func padRight(s string, n int) string {
	if len(s) >= n {
		return s
	}
	return s + strings.Repeat(" ", n-len(s))
}

func padLeft(s string, n int) string {
	if len(s) >= n {
		return s
	}
	return strings.Repeat(" ", n-len(s)) + s
}

// visibleLen returns the visible character count (strips ANSI escape sequences).
func visibleLen(s string) int {
	inEsc := false
	n := 0
	for _, r := range s {
		if r == '\033' {
			inEsc = true
			continue
		}
		if inEsc {
			if r == 'm' {
				inEsc = false
			}
			continue
		}
		n++
	}
	return n
}
