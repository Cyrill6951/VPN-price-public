package auth

import (
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"strings"
)

// Handler exposes the auth HTTP endpoints.
type Handler struct {
	svc *Service
	mw  *Middleware
}

// NewHandler builds the auth handler.
func NewHandler(svc *Service, mw *Middleware) *Handler {
	return &Handler{svc: svc, mw: mw}
}

// RegisterRoutes mounts auth endpoints under /api/v1/auth on the given mux.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/auth/register", h.register)
	mux.HandleFunc("POST /api/v1/auth/login", h.login)
	mux.HandleFunc("POST /api/v1/auth/refresh", h.refresh)
	mux.HandleFunc("POST /api/v1/auth/telegram", h.telegram)
	// Logout requires a valid access token.
	mux.Handle("POST /api/v1/auth/logout", h.mw.Authenticate(http.HandlerFunc(h.logout)))
}

type authResponse struct {
	User   User      `json:"user"`
	Tokens TokenPair `json:"tokens"`
}

func (h *Handler) register(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string  `json:"email"`
		Password string  `json:"password"`
		Language string  `json:"language"`
		Country  *string `json:"country"`
	}
	if !decode(w, r, &req) {
		return
	}
	if !validEmail(req.Email) {
		writeError(w, http.StatusBadRequest, "invalid email")
		return
	}
	u, tokens, err := h.svc.Register(r.Context(), strings.TrimSpace(req.Email), req.Password, req.Language, req.Country, metaFromRequest(r))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, authResponse{User: u, Tokens: tokens})
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if !decode(w, r, &req) {
		return
	}
	u, tokens, err := h.svc.Login(r.Context(), strings.TrimSpace(req.Email), req.Password, metaFromRequest(r))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, authResponse{User: u, Tokens: tokens})
}

func (h *Handler) refresh(w http.ResponseWriter, r *http.Request) {
	var req struct {
		RefreshToken string `json:"refresh_token"`
	}
	if !decode(w, r, &req) {
		return
	}
	if req.RefreshToken == "" {
		writeError(w, http.StatusBadRequest, "refresh_token is required")
		return
	}
	tokens, err := h.svc.Refresh(r.Context(), req.RefreshToken, metaFromRequest(r))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"tokens": tokens})
}

func (h *Handler) telegram(w http.ResponseWriter, r *http.Request) {
	var req TelegramAuth
	if !decode(w, r, &req) {
		return
	}
	u, tokens, err := h.svc.TelegramLogin(r.Context(), req, metaFromRequest(r))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, authResponse{User: u, Tokens: tokens})
}

func (h *Handler) logout(w http.ResponseWriter, r *http.Request) {
	p, ok := PrincipalFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthenticated")
		return
	}
	var req struct {
		RefreshToken string `json:"refresh_token"`
	}
	// Body is optional for logout; ignore decode errors on empty body.
	_ = json.NewDecoder(r.Body).Decode(&req)

	if err := h.svc.Logout(r.Context(), p.JTI, p.ExpiresAt, req.RefreshToken); err != nil {
		writeError(w, http.StatusInternalServerError, "logout failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "logged_out"})
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

func writeDomainError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrEmailTaken):
		writeError(w, http.StatusConflict, err.Error())
	case errors.Is(err, ErrInvalidCredentials):
		writeError(w, http.StatusUnauthorized, err.Error())
	case errors.Is(err, ErrUserBlocked):
		writeError(w, http.StatusForbidden, err.Error())
	case errors.Is(err, ErrSessionRevoked), errors.Is(err, ErrInvalidToken):
		writeError(w, http.StatusUnauthorized, err.Error())
	case errors.Is(err, ErrTelegramSignature):
		writeError(w, http.StatusUnauthorized, err.Error())
	case errors.Is(err, ErrNotFound):
		writeError(w, http.StatusNotFound, err.Error())
	default:
		// Validation messages (password policy, etc.) surface as 400.
		writeError(w, http.StatusBadRequest, err.Error())
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

func metaFromRequest(r *http.Request) LoginMeta {
	return LoginMeta{UserAgent: r.UserAgent(), IP: clientIP(r)}
}

func clientIP(r *http.Request) net.IP {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if first, _, ok := strings.Cut(xff, ","); ok {
			return net.ParseIP(strings.TrimSpace(first))
		}
		return net.ParseIP(strings.TrimSpace(xff))
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return net.ParseIP(r.RemoteAddr)
	}
	return net.ParseIP(host)
}

func validEmail(email string) bool {
	email = strings.TrimSpace(email)
	at := strings.IndexByte(email, '@')
	if at <= 0 || at == len(email)-1 {
		return false
	}
	return strings.IndexByte(email[at+1:], '.') > 0
}
