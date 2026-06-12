package admin

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/google/uuid"

	"github.com/vpnsaas/platform/internal/auth"
)

// Handler exposes the CMS admin API, restricted to admin roles.
type Handler struct {
	repo *Repository
	mw   *auth.Middleware
}

// NewHandler builds the admin handler.
func NewHandler(repo *Repository, mw *auth.Middleware) *Handler {
	return &Handler{repo: repo, mw: mw}
}

var allowedUserStatus = map[string]bool{"active": true, "blocked": true, "pending": true}
var allowedUserRole = map[string]bool{
	"user": true, "vip": true, "partner": true, "reseller": true,
	"support": true, "moderator": true, "admin": true, "superadmin": true,
}
var allowedServerStatus = map[string]bool{
	"active": true, "drain": true, "offline": true, "archived": true, "provisioning": true,
}

// RegisterRoutes mounts /api/v1/admin endpoints behind auth + RBAC.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.Handle("GET /api/v1/admin/dashboard", h.guard(h.dashboard))
	mux.Handle("GET /api/v1/admin/users", h.guard(h.users))
	mux.Handle("POST /api/v1/admin/users/{id}/status", h.guard(h.userStatus))
	mux.Handle("POST /api/v1/admin/users/{id}/role", h.guard(h.userRole))
	mux.Handle("GET /api/v1/admin/servers", h.guard(h.servers))
	mux.Handle("POST /api/v1/admin/servers/{id}/status", h.guard(h.serverStatus))
	mux.Handle("GET /api/v1/admin/orders", h.guard(h.orders))
	mux.Handle("GET /api/v1/admin/payments", h.guard(h.payments))
	mux.Handle("GET /api/v1/admin/promocodes", h.guard(h.promos))
	mux.Handle("POST /api/v1/admin/promocodes", h.guard(h.createPromo))
	mux.Handle("GET /api/v1/admin/audit", h.guard(h.audit))
}

// guard wraps a handler with authentication and the admin/superadmin role check.
func (h *Handler) guard(fn http.HandlerFunc) http.Handler {
	return h.mw.Authenticate(auth.RequireRole(auth.RoleAdmin, auth.RoleSuperAdmin)(fn))
}

func (h *Handler) dashboard(w http.ResponseWriter, r *http.Request) {
	d, err := h.repo.Dashboard(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, d)
}

func (h *Handler) users(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	limit, offset := pagination(r)
	list, err := h.repo.ListUsers(r.Context(), q, limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"users": list})
}

func (h *Handler) userStatus(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	var req struct {
		Status string `json:"status"`
	}
	if !decode(w, r, &req) {
		return
	}
	if !allowedUserStatus[req.Status] {
		writeError(w, http.StatusBadRequest, "invalid status")
		return
	}
	if err := h.repo.SetUserStatus(r.Context(), id, req.Status); err != nil {
		writeRepoError(w, err)
		return
	}
	h.audited(r, "user.status", "user", id.String())
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) userRole(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	var req struct {
		Role string `json:"role"`
	}
	if !decode(w, r, &req) {
		return
	}
	if !allowedUserRole[req.Role] {
		writeError(w, http.StatusBadRequest, "invalid role")
		return
	}
	if err := h.repo.SetUserRole(r.Context(), id, req.Role); err != nil {
		writeRepoError(w, err)
		return
	}
	h.audited(r, "user.role", "user", id.String())
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) servers(w http.ResponseWriter, r *http.Request) {
	list, err := h.repo.ListServers(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"servers": list})
}

func (h *Handler) serverStatus(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	var req struct {
		Status string `json:"status"`
	}
	if !decode(w, r, &req) {
		return
	}
	if !allowedServerStatus[req.Status] {
		writeError(w, http.StatusBadRequest, "invalid status")
		return
	}
	if err := h.repo.SetServerStatus(r.Context(), id, req.Status); err != nil {
		writeRepoError(w, err)
		return
	}
	h.audited(r, "server.status", "server", id.String())
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) orders(w http.ResponseWriter, r *http.Request) {
	limit, offset := pagination(r)
	list, err := h.repo.ListOrders(r.Context(), limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"orders": list})
}

func (h *Handler) payments(w http.ResponseWriter, r *http.Request) {
	limit, _ := pagination(r)
	list, err := h.repo.ListPayments(r.Context(), limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"payments": list})
}

func (h *Handler) promos(w http.ResponseWriter, r *http.Request) {
	list, err := h.repo.ListPromos(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"promocodes": list})
}

func (h *Handler) createPromo(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Code     string  `json:"code"`
		Type     string  `json:"type"`
		Discount float64 `json:"discount"`
		MaxUses  *int    `json:"max_uses"`
	}
	if !decode(w, r, &req) {
		return
	}
	if req.Code == "" || (req.Type != "percent" && req.Type != "fixed") {
		writeError(w, http.StatusBadRequest, "code and type (percent|fixed) are required")
		return
	}
	p, err := h.repo.CreatePromo(r.Context(), req.Code, req.Type, req.Discount, req.MaxUses)
	if err != nil {
		writeError(w, http.StatusConflict, "could not create promo (duplicate code?)")
		return
	}
	h.audited(r, "promo.create", "promo", p.ID.String())
	writeJSON(w, http.StatusCreated, p)
}

func (h *Handler) audit(w http.ResponseWriter, r *http.Request) {
	limit, _ := pagination(r)
	list, err := h.repo.ListAudit(r.Context(), limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"audit": list})
}

// --- helpers ---

func (h *Handler) audited(r *http.Request, action, objType, objID string) {
	if p, ok := auth.PrincipalFromContext(r.Context()); ok {
		_ = h.repo.InsertAudit(r.Context(), p.UserID, action, objType, objID)
	}
}

func parseID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return uuid.Nil, false
	}
	return id, true
}

func pagination(r *http.Request) (limit, offset int) {
	limit, offset = 50, 0
	if v, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil && v > 0 && v <= 500 {
		limit = v
	}
	if v, err := strconv.Atoi(r.URL.Query().Get("offset")); err == nil && v >= 0 {
		offset = v
	}
	return limit, offset
}

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
		writeError(w, http.StatusNotFound, "not found")
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
