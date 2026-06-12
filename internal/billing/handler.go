package billing

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/google/uuid"

	"github.com/vpnsaas/platform/internal/auth"
	"github.com/vpnsaas/platform/internal/vpn"
)

// Handler exposes billing endpoints.
type Handler struct {
	svc     *Service
	mw      *auth.Middleware
	devMode bool
}

// NewHandler builds the billing handler. devMode enables the mock-confirm endpoint.
func NewHandler(svc *Service, mw *auth.Middleware, devMode bool) *Handler {
	return &Handler{svc: svc, mw: mw, devMode: devMode}
}

// RegisterRoutes mounts billing endpoints.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/billing/gateways", h.gateways)
	// Webhooks are public; authenticity is verified by provider signatures.
	mux.HandleFunc("POST /api/v1/billing/webhook/{gateway}", h.webhook)

	protect := h.mw.Authenticate
	mux.Handle("POST /api/v1/billing/buy", protect(http.HandlerFunc(h.buy)))
	mux.Handle("GET /api/v1/billing/orders", protect(http.HandlerFunc(h.orders)))
	mux.Handle("POST /api/v1/billing/promo/check", protect(http.HandlerFunc(h.promoCheck)))

	if h.devMode {
		mux.Handle("POST /api/v1/billing/dev/confirm", protect(http.HandlerFunc(h.devConfirm)))
	}
}

func (h *Handler) gateways(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"gateways": h.svc.Gateways()})
}

func (h *Handler) buy(w http.ResponseWriter, r *http.Request) {
	p, _ := auth.PrincipalFromContext(r.Context())
	var req struct {
		PlanID    string  `json:"plan_id"`
		CountryID string  `json:"country_id"`
		Protocol  string  `json:"protocol"`
		Gateway   string  `json:"gateway"`
		PromoCode string  `json:"promo_code"`
		Label     *string `json:"label"`
		Routing   string  `json:"routing"`
	}
	if !decode(w, r, &req) {
		return
	}
	planID, err := uuid.Parse(req.PlanID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid plan_id")
		return
	}
	countryID, err := uuid.Parse(req.CountryID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid country_id")
		return
	}

	order, payment, err := h.svc.Buy(r.Context(), BuyInput{
		UserID:    p.UserID,
		PlanID:    planID,
		CountryID: countryID,
		Protocol:  vpn.Protocol(req.Protocol),
		Gateway:   req.Gateway,
		PromoCode: req.PromoCode,
		Label:     req.Label,
		Routing:   vpn.RoutingMode(req.Routing),
	})
	if err != nil {
		writeBillingError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"order": order, "payment": payment})
}

func (h *Handler) webhook(w http.ResponseWriter, r *http.Request) {
	gateway := r.PathValue("gateway")
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
	if err != nil {
		writeError(w, http.StatusBadRequest, "cannot read body")
		return
	}
	if err := h.svc.HandleWebhook(r.Context(), gateway, r.Header, body); err != nil {
		// Acknowledge bad signatures/unknown gateways with 400; processing errors 500.
		switch {
		case errors.Is(err, ErrUnknownGateway), errors.Is(err, ErrInvalidSignature):
			writeError(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, ErrOrderNotFound):
			writeJSON(w, http.StatusOK, map[string]string{"status": "ignored"})
		default:
			writeError(w, http.StatusInternalServerError, "processing error")
		}
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) orders(w http.ResponseWriter, r *http.Request) {
	p, _ := auth.PrincipalFromContext(r.Context())
	list, err := h.svc.ListOrders(r.Context(), p.UserID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"orders": list})
}

func (h *Handler) promoCheck(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Code   string `json:"code"`
		PlanID string `json:"plan_id"`
	}
	if !decode(w, r, &req) {
		return
	}
	planID, err := uuid.Parse(req.PlanID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid plan_id")
		return
	}
	discount, total, err := h.svc.PromoPreview(r.Context(), req.Code, planID)
	if err != nil {
		writeBillingError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"discount": discount, "total": total})
}

func (h *Handler) devConfirm(w http.ResponseWriter, r *http.Request) {
	var req struct {
		OrderID string `json:"order_id"`
	}
	if !decode(w, r, &req) {
		return
	}
	orderID, err := uuid.Parse(req.OrderID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid order_id")
		return
	}
	if err := h.svc.ConfirmMock(r.Context(), orderID); err != nil {
		writeBillingError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "confirmed"})
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

func writeBillingError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrPlanNotFound), errors.Is(err, ErrOrderNotFound):
		writeError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, ErrUnknownGateway), errors.Is(err, ErrBadProtocol),
		errors.Is(err, ErrPromoInvalid), errors.Is(err, ErrPromoUsed):
		writeError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, ErrTooManyPending):
		writeError(w, http.StatusTooManyRequests, err.Error())
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
