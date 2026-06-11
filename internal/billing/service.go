package billing

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/vpnsaas/platform/internal/vpn"
)

const (
	pendingWindow   = 10 * time.Minute
	maxPendingOrder = 5
)

// Service contains the billing business logic.
type Service struct {
	repo        *Repository
	providers   map[string]Provider
	issuer      VPNIssuer
	callbackURL string
	log         *slog.Logger
}

// NewService wires the billing service with the configured providers.
func NewService(repo *Repository, issuer VPNIssuer, providers map[string]Provider, callbackBaseURL string, log *slog.Logger) *Service {
	return &Service{
		repo:        repo,
		providers:   providers,
		issuer:      issuer,
		callbackURL: strings.TrimRight(callbackBaseURL, "/"),
		log:         log,
	}
}

// BuyInput is a purchase request.
type BuyInput struct {
	UserID    uuid.UUID
	PlanID    uuid.UUID
	CountryID uuid.UUID
	Protocol  vpn.Protocol
	Gateway   string
	PromoCode string
	Label     *string
}

// Buy creates an order and a gateway payment, returning where to pay.
func (s *Service) Buy(ctx context.Context, in BuyInput) (Order, Payment, error) {
	if !in.Protocol.Valid() {
		return Order{}, Payment{}, ErrBadProtocol
	}
	provider, ok := s.providers[in.Gateway]
	if !ok {
		return Order{}, Payment{}, ErrUnknownGateway
	}

	plan, err := s.repo.GetPlanPricing(ctx, in.PlanID)
	if err != nil {
		return Order{}, Payment{}, err
	}

	discount, promoID, err := s.resolvePromo(ctx, in.PromoCode, plan)
	if err != nil {
		return Order{}, Payment{}, err
	}
	amount := round2(math.Max(0, plan.Price-discount))

	pending, err := s.repo.CountRecentPendingOrders(ctx, in.UserID, time.Now().Add(-pendingWindow))
	if err != nil {
		return Order{}, Payment{}, err
	}
	if pending >= maxPendingOrder {
		return Order{}, Payment{}, ErrTooManyPending
	}

	order, err := s.repo.CreateOrder(ctx, NewOrder{
		UserID: in.UserID, PlanID: in.PlanID, CountryID: in.CountryID, Protocol: in.Protocol,
		Amount: amount, Discount: round2(discount), Currency: plan.Currency,
		Gateway: in.Gateway, PromoID: promoID, Label: in.Label,
	})
	if err != nil {
		return Order{}, Payment{}, err
	}

	res, err := provider.CreatePayment(ctx, CreatePaymentInput{
		OrderID:     order.ID,
		Amount:      amount,
		Currency:    plan.Currency,
		Description: "VPN subscription",
		CallbackURL: s.callbackURL + "/api/v1/billing/webhook/" + in.Gateway,
		ReturnURL:   s.callbackURL,
	})
	if err != nil {
		_ = s.repo.MarkFailed(ctx, order.ID)
		return Order{}, Payment{}, fmt.Errorf("create payment: %w", err)
	}

	payment, err := s.repo.CreatePayment(ctx, order.ID, in.Gateway, res.ExternalID, res.Status, amount, plan.Currency, res.PaymentURL)
	if err != nil {
		return Order{}, Payment{}, err
	}
	return order, payment, nil
}

func (s *Service) resolvePromo(ctx context.Context, code string, plan PlanPricing) (discount float64, promoID *uuid.UUID, err error) {
	code = strings.TrimSpace(code)
	if code == "" {
		return 0, nil, nil
	}
	promo, err := s.repo.GetPromoByCode(ctx, code)
	if err != nil {
		return 0, nil, err
	}
	if !promo.Active || (promo.ExpiresAt != nil && promo.ExpiresAt.Before(time.Now())) {
		return 0, nil, ErrPromoInvalid
	}
	switch promo.Type {
	case "percent":
		discount = plan.Price * promo.Discount / 100
	case "fixed":
		discount = promo.Discount
	}
	id := promo.ID
	return round2(discount), &id, nil
}

// HandleWebhook processes a gateway callback: on success it provisions the VPN
// (exactly once) and finalizes the order.
func (s *Service) HandleWebhook(ctx context.Context, gateway string, headers http.Header, body []byte) error {
	provider, ok := s.providers[gateway]
	if !ok {
		return ErrUnknownGateway
	}
	res, err := provider.ParseWebhook(headers, body)
	if err != nil {
		return err
	}
	if err := s.repo.RecordWebhook(ctx, gateway, res.ExternalID, string(res.Status), body); err != nil {
		return err
	}

	order, err := s.resolveOrder(ctx, gateway, res)
	if err != nil {
		return err
	}

	switch res.Status {
	case PaymentSuccess:
		if err := s.fulfill(ctx, order); err != nil {
			return err
		}
	case PaymentFailed:
		if err := s.repo.MarkFailed(ctx, order.ID); err != nil {
			return err
		}
	default:
		return nil // pending — wait for a terminal callback
	}
	_ = s.repo.MarkWebhookProcessed(ctx, gateway, res.ExternalID)
	return nil
}

func (s *Service) resolveOrder(ctx context.Context, gateway string, res WebhookResult) (OrderForProvision, error) {
	if res.OrderID != uuid.Nil {
		return s.repo.GetOrderForProvision(ctx, res.OrderID)
	}
	return s.repo.GetOrderByExternalPayment(ctx, gateway, res.ExternalID)
}

// fulfill provisions the VPN once and finalizes the order.
func (s *Service) fulfill(ctx context.Context, order OrderForProvision) error {
	claimed, err := s.repo.ClaimForProvisioning(ctx, order.ID)
	if err != nil {
		return err
	}
	if !claimed {
		s.log.Info("order already fulfilled, skipping", "order", order.ID)
		return nil
	}

	view, err := s.issuer.Create(ctx, vpn.CreateInput{
		UserID:    order.UserID,
		CountryID: order.CountryID,
		Protocol:  order.Protocol,
		PlanID:    &order.PlanID,
		Label:     order.Label,
	})
	if err != nil {
		return fmt.Errorf("provision vpn: %w", err)
	}
	if err := s.repo.FinalizePaid(ctx, order.ID, view.Subscription.ID); err != nil {
		return fmt.Errorf("finalize order: %w", err)
	}
	s.log.Info("order fulfilled", "order", order.ID, "subscription", view.Subscription.ID)
	return nil
}

// ConfirmMock simulates a successful gateway callback for the mock provider.
// Intended for development/testing only.
func (s *Service) ConfirmMock(ctx context.Context, orderID uuid.UUID) error {
	if _, ok := s.providers["mock"]; !ok {
		return ErrUnknownGateway
	}
	body := []byte(fmt.Sprintf(`{"external_id":"mock-%s","order_id":"%s","status":"success"}`,
		orderID.String(), orderID.String()))
	return s.HandleWebhook(ctx, "mock", http.Header{}, body)
}

// ListOrders returns a user's orders.
func (s *Service) ListOrders(ctx context.Context, userID uuid.UUID) ([]Order, error) {
	return s.repo.ListOrders(ctx, userID)
}

// PromoPreview validates a promo code against a plan and returns the discount.
func (s *Service) PromoPreview(ctx context.Context, code string, planID uuid.UUID) (float64, float64, error) {
	plan, err := s.repo.GetPlanPricing(ctx, planID)
	if err != nil {
		return 0, 0, err
	}
	discount, _, err := s.resolvePromo(ctx, code, plan)
	if err != nil {
		return 0, 0, err
	}
	return round2(discount), round2(math.Max(0, plan.Price-discount)), nil
}

// Gateways returns the list of enabled gateway names.
func (s *Service) Gateways() []string {
	names := make([]string, 0, len(s.providers))
	for n := range s.providers {
		names = append(names, n)
	}
	return names
}

func round2(v float64) float64 { return math.Round(v*100) / 100 }
