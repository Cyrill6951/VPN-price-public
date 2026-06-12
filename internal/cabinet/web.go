// Package cabinet serves the end-user web cabinet (a single-page app) that talks
// to the public auth/user/vpn/billing APIs. No backend logic lives here.
package cabinet

import (
	_ "embed"
	"net/http"
)

//go:embed web/index.html
var cabinetHTML []byte

// Register mounts the user cabinet SPA at /app.
func Register(mux *http.ServeMux) {
	serve := func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(cabinetHTML)
	}
	mux.HandleFunc("GET /app", serve)
	mux.HandleFunc("GET /app/", serve)
}
