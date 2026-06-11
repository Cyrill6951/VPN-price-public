package user

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/google/uuid"

	"github.com/vpnsaas/platform/internal/auth"
)

// Handler exposes profile and device endpoints for authenticated users.
type Handler struct {
	repo *Repository
	mw   *auth.Middleware
}

// NewHandler builds the user handler.
func NewHandler(repo *Repository, mw *auth.Middleware) *Handler {
	return &Handler{repo: repo, mw: mw}
}

// RegisterRoutes mounts /api/v1/users endpoints, all requiring authentication.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	protect := h.mw.Authenticate
	mux.Handle("GET /api/v1/users/me", protect(http.HandlerFunc(h.getMe)))
	mux.Handle("PUT /api/v1/users/me", protect(http.HandlerFunc(h.updateMe)))
	mux.Handle("GET /api/v1/users/me/devices", protect(http.HandlerFunc(h.listDevices)))
	mux.Handle("POST /api/v1/users/me/devices", protect(http.HandlerFunc(h.upsertDevice)))
	mux.Handle("DELETE /api/v1/users/me/devices/{id}", protect(http.HandlerFunc(h.deleteDevice)))
}

func (h *Handler) getMe(w http.ResponseWriter, r *http.Request) {
	p, _ := auth.PrincipalFromContext(r.Context())
	me, err := h.repo.GetMe(r.Context(), p.UserID)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, me)
}

func (h *Handler) updateMe(w http.ResponseWriter, r *http.Request) {
	p, _ := auth.PrincipalFromContext(r.Context())
	var req struct {
		FirstName *string `json:"first_name"`
		LastName  *string `json:"last_name"`
		City      *string `json:"city"`
		Country   *string `json:"country"`
		Avatar    *string `json:"avatar"`
		Language  *string `json:"language"`
		Timezone  *string `json:"timezone"`
	}
	if !decode(w, r, &req) {
		return
	}
	err := h.repo.UpdateProfile(r.Context(), p.UserID, ProfileUpdate(req))
	if err != nil {
		writeRepoError(w, err)
		return
	}
	me, err := h.repo.GetMe(r.Context(), p.UserID)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, me)
}

func (h *Handler) listDevices(w http.ResponseWriter, r *http.Request) {
	p, _ := auth.PrincipalFromContext(r.Context())
	devices, err := h.repo.ListDevices(r.Context(), p.UserID)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"devices": devices})
}

func (h *Handler) upsertDevice(w http.ResponseWriter, r *http.Request) {
	p, _ := auth.PrincipalFromContext(r.Context())
	var req struct {
		DeviceUUID string  `json:"device_uuid"`
		Name       *string `json:"name"`
		Platform   *string `json:"platform"`
		OS         *string `json:"os"`
		Version    *string `json:"version"`
	}
	if !decode(w, r, &req) {
		return
	}
	if req.DeviceUUID == "" {
		writeError(w, http.StatusBadRequest, "device_uuid is required")
		return
	}
	d, err := h.repo.UpsertDevice(r.Context(), p.UserID, DeviceUpsert{
		DeviceUUID: req.DeviceUUID, Name: req.Name,
		Platform: req.Platform, OS: req.OS, Version: req.Version,
	})
	if err != nil {
		writeRepoError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, d)
}

func (h *Handler) deleteDevice(w http.ResponseWriter, r *http.Request) {
	p, _ := auth.PrincipalFromContext(r.Context())
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid device id")
		return
	}
	if err := h.repo.DeleteDevice(r.Context(), p.UserID, id); err != nil {
		writeRepoError(w, err)
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

func writeRepoError(w http.ResponseWriter, err error) {
	if errors.Is(err, ErrNotFound) {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeError(w, http.StatusInternalServerError, "internal error")
}

func writeJSON(w http.ResponseWriter, code int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}
