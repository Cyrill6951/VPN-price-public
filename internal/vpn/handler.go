package vpn

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/google/uuid"

	"github.com/vpnsaas/platform/internal/auth"
)

// Handler exposes VPN and catalogue endpoints.
type Handler struct {
	svc *Service
	mw  *auth.Middleware
}

// NewHandler builds the VPN handler.
func NewHandler(svc *Service, mw *auth.Middleware) *Handler {
	return &Handler{svc: svc, mw: mw}
}

// RegisterRoutes mounts catalogue (public) and VPN (protected) endpoints.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/countries", h.countries)
	mux.HandleFunc("GET /api/v1/plans", h.plans)

	protect := h.mw.Authenticate
	mux.Handle("POST /api/v1/vpn/create", protect(http.HandlerFunc(h.create)))
	mux.Handle("GET /api/v1/vpn/list", protect(http.HandlerFunc(h.list)))
	mux.Handle("GET /api/v1/vpn/{id}/config", protect(http.HandlerFunc(h.config)))
	mux.Handle("GET /api/v1/vpn/{id}/qr", protect(http.HandlerFunc(h.qr)))
	mux.Handle("DELETE /api/v1/vpn/{id}", protect(http.HandlerFunc(h.delete)))
}

func (h *Handler) countries(w http.ResponseWriter, r *http.Request) {
	list, err := h.svc.Countries(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"countries": list})
}

func (h *Handler) plans(w http.ResponseWriter, r *http.Request) {
	list, err := h.svc.Plans(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"plans": list})
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	p, _ := auth.PrincipalFromContext(r.Context())
	var req struct {
		CountryID string  `json:"country_id"`
		Protocol  string  `json:"protocol"`
		PlanID    *string `json:"plan_id"`
		Label     *string `json:"label"`
	}
	if !decode(w, r, &req) {
		return
	}
	countryID, err := uuid.Parse(req.CountryID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid country_id")
		return
	}
	in := CreateInput{
		UserID:    p.UserID,
		CountryID: countryID,
		Protocol:  Protocol(req.Protocol),
		Label:     req.Label,
	}
	if req.PlanID != nil {
		planID, perr := uuid.Parse(*req.PlanID)
		if perr != nil {
			writeError(w, http.StatusBadRequest, "invalid plan_id")
			return
		}
		in.PlanID = &planID
	}

	view, err := h.svc.Create(r.Context(), in)
	if err != nil {
		writeVPNError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, view)
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	p, _ := auth.PrincipalFromContext(r.Context())
	views, err := h.svc.List(r.Context(), p.UserID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"vpns": views})
}

func (h *Handler) config(w http.ResponseWriter, r *http.Request) {
	p, _ := auth.PrincipalFromContext(r.Context())
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	data, contentType, err := h.svc.FetchConfig(r.Context(), p.UserID, id)
	if err != nil {
		writeVPNError(w, err)
		return
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", `attachment; filename="vpn.conf"`)
	_, _ = w.Write(data)
}

func (h *Handler) qr(w http.ResponseWriter, r *http.Request) {
	p, _ := auth.PrincipalFromContext(r.Context())
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	png, err := h.svc.FetchQR(r.Context(), p.UserID, id)
	if err != nil {
		writeVPNError(w, err)
		return
	}
	w.Header().Set("Content-Type", "image/png")
	_, _ = w.Write(png)
}

func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	p, _ := auth.PrincipalFromContext(r.Context())
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	if err := h.svc.Delete(r.Context(), p.UserID, id); err != nil {
		writeVPNError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// --- helpers ---

func decode(w http.ResponseWriter, r *http.Request, dst any) bool {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return false
	}
	return true
}

func writeVPNError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		writeError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, ErrUnsupportedProtocol):
		writeError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, ErrNoServerAvailable), errors.Is(err, ErrServerNotConfigured), errors.Is(err, ErrSubnetExhausted):
		writeError(w, http.StatusServiceUnavailable, err.Error())
	default:
		writeError(w, http.StatusInternalServerError, "internal error")
	}
}

func writeJSON(w http.ResponseWriter, code int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}
