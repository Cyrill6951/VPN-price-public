// Package billing implements orders, payments, promo codes, an immutable ledger
// and webhook-driven automatic VPN issuance.
package billing

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/vpnsaas/platform/internal/vpn"
)

// OrderStatus mirrors the order_status enum.
type OrderStatus string

const (
	OrderNew         OrderStatus = "new"
	OrderWaitPayment OrderStatus = "wait_payment"
	OrderProcessing  OrderStatus = "processing"
	OrderPaid        OrderStatus = "paid"
	OrderFailed      OrderStatus = "failed"
	OrderExpired     OrderStatus = "expired"
	OrderCancelled   OrderStatus = "cancelled"
	OrderRefunded    OrderStatus = "refunded"
)

// PaymentStatus mirrors the payment_status enum.
type PaymentStatus string

const (
	PaymentCreated PaymentStatus = "created"
	PaymentPending PaymentStatus = "pending"
	PaymentSuccess PaymentStatus = "success"
	PaymentFailed  PaymentStatus = "failed"
	PaymentExpired PaymentStatus = "expired"
)

// Order is a purchase intent.
type Order struct {
	ID             uuid.UUID    `json:"id"`
	UserID         uuid.UUID    `json:"user_id"`
	PlanID         uuid.UUID    `json:"plan_id"`
	CountryID      uuid.UUID    `json:"country_id"`
	Protocol       vpn.Protocol `json:"protocol"`
	Amount         float64      `json:"amount"`
	Discount       float64      `json:"discount"`
	Currency       string       `json:"currency"`
	Gateway        string       `json:"gateway"`
	Status         OrderStatus  `json:"status"`
	SubscriptionID *uuid.UUID   `json:"subscription_id,omitempty"`
	CreatedAt      time.Time    `json:"created_at"`
	PaidAt         *time.Time   `json:"paid_at,omitempty"`
}

// Payment is a gateway payment attached to an order.
type Payment struct {
	ID         uuid.UUID     `json:"id"`
	OrderID    uuid.UUID     `json:"order_id"`
	Gateway    string        `json:"gateway"`
	ExternalID string        `json:"external_id"`
	Status     PaymentStatus `json:"status"`
	Amount     float64       `json:"amount"`
	Currency   string        `json:"currency"`
	PaymentURL string        `json:"payment_url"`
	CreatedAt  time.Time     `json:"created_at"`
}

// Provider integrates a payment gateway.
type Provider interface {
	Name() string
	CreatePayment(ctx context.Context, in CreatePaymentInput) (CreatePaymentResult, error)
	ParseWebhook(headers http.Header, body []byte) (WebhookResult, error)
}

// CreatePaymentInput is passed to a provider to register a payment.
type CreatePaymentInput struct {
	OrderID     uuid.UUID
	Amount      float64
	Currency    string
	Description string
	CallbackURL string
	ReturnURL   string
}

// CreatePaymentResult is returned by a provider after registering a payment.
type CreatePaymentResult struct {
	ExternalID string
	PaymentURL string
	Status     PaymentStatus
}

// WebhookResult is the normalized outcome of a gateway webhook.
type WebhookResult struct {
	ExternalID string
	OrderID    uuid.UUID // zero if the provider does not echo it
	Status     PaymentStatus
}

// VPNIssuer provisions a VPN once an order is paid (implemented by vpn.Service).
type VPNIssuer interface {
	Create(ctx context.Context, in vpn.CreateInput) (vpn.VPNView, error)
}

// Domain errors.
var (
	ErrPlanNotFound     = errors.New("plan not found")
	ErrUnknownGateway   = errors.New("unknown payment gateway")
	ErrOrderNotFound    = errors.New("order not found")
	ErrPromoInvalid     = errors.New("promo code is invalid or expired")
	ErrPromoUsed        = errors.New("promo code already used")
	ErrTooManyPending   = errors.New("too many pending orders, try again later")
	ErrBadProtocol      = errors.New("unsupported protocol")
	ErrWebhookConflict  = errors.New("webhook already processed")
	ErrInvalidSignature = errors.New("invalid webhook signature")
)
