package admin

import (
	_ "embed"
	"net/http"
)

//go:embed web/index.html
var panelHTML []byte

// RegisterPanel serves the static admin panel (single-page app) at /admin.
// The page itself authenticates via the API; the HTML is public.
func (h *Handler) RegisterPanel(mux *http.ServeMux) {
	serve := func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(panelHTML)
	}
	mux.HandleFunc("GET /admin", serve)
	mux.HandleFunc("GET /admin/", serve)
}
