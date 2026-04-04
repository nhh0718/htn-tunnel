// Package client: auth_callback.go implements the localhost HTTP callback server
// used by "htn-tunnel login" to receive API keys from the browser after registration.
package client

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os/exec"
	"runtime"
)

// CallbackResult holds the key and optional user name returned by the browser callback.
type CallbackResult struct {
	Key  string
	Name string
}

// StartCallbackServer starts a temporary HTTP server on a random localhost port.
// It returns the callback URL and a channel that receives the result when the
// browser redirects back with ?key=...&name=...
func StartCallbackServer(ctx context.Context) (callbackURL string, resultCh <-chan CallbackResult, err error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", nil, fmt.Errorf("listen callback: %w", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	callbackURL = fmt.Sprintf("http://127.0.0.1:%d/cb", port)

	ch := make(chan CallbackResult, 1)
	mux := http.NewServeMux()
	mux.HandleFunc("/cb", func(w http.ResponseWriter, r *http.Request) {
		key := r.URL.Query().Get("key")
		name := r.URL.Query().Get("name")
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, `<!DOCTYPE html><html><head><meta charset="utf-8"><title>htn-tunnel</title></head><body style="font-family:system-ui;text-align:center;padding:60px">
<h1 style="color:#4ade80">&#10003; Đăng ký thành công!</h1>
<p>Quay lại terminal để tiếp tục.</p>
<script>setTimeout(()=>window.close(),2000)</script>
</body></html>`)
		select {
		case ch <- CallbackResult{Key: key, Name: name}:
		default:
		}
	})

	srv := &http.Server{Handler: mux}
	go srv.Serve(ln) //nolint:errcheck
	go func() {
		select {
		case <-ctx.Done():
		case <-ch:
		}
		srv.Close()
	}()

	return callbackURL, ch, nil
}

// OpenBrowser opens url in the user's default browser.
func OpenBrowser(url string) error {
	switch runtime.GOOS {
	case "linux":
		return exec.Command("xdg-open", url).Start()
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		return exec.Command("open", url).Start()
	default:
		return fmt.Errorf("unsupported platform %s", runtime.GOOS)
	}
}
