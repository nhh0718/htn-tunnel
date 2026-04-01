// Package dashboard serves the embedded web dashboard for htn-tunnel.
// Static assets are compiled into the binary via Go's embed package.
package dashboard

import "embed"

//go:embed static/*
var staticFiles embed.FS
