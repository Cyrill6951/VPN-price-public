package billing

import (
	"context"
	"net/http"
)

// StarsProvider represents the Telegram Stars gateway. Payment is initiated and
// confirmed by the Telegram bot (sendInvoice + successful_payment), which then
// calls Service.FulfillOrder directly — so this provider only registers the
// payment intent and is never driven through the HTTP webhook route.
type StarsProvider struct{}

// NewStarsProvider builds the Telegram Stars provider.
func NewStarsProvider() *StarsProvider { return &StarsProvider{} }

// Name returns the gateway identifier.
func (p *StarsProvider) Name() string { return GatewayTelegramStars }

// CreatePayment registers the intent; the bot sends the actual Stars invoice.
func (p *StarsProvider) CreatePayment(_ context.Context, in CreatePaymentInput) (CreatePaymentResult, error) {
	return CreatePaymentResult{
		ExternalID: "stars-" + in.OrderID.String(),
		Status:     PaymentPending,
	}, nil
}

// ParseWebhook is not used: Telegram Stars are confirmed in-bot.
func (p *StarsProvider) ParseWebhook(_ http.Header, _ []byte) (WebhookResult, error) {
	return WebhookResult{}, ErrUnknownGateway
}

// GatewayTelegramStars is the gateway identifier for Telegram Stars.
const GatewayTelegramStars = "telegram_stars"
