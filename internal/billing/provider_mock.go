package billing

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
)

// MockProvider is a development gateway: it creates a payment that is confirmed
// via the dev endpoint (POST /billing/dev/confirm). It must never be enabled in
// production.
type MockProvider struct {
	baseURL string
}

// NewMockProvider builds the mock gateway. baseURL is used to build a human-readable
// payment URL only.
func NewMockProvider(baseURL string) *MockProvider {
	return &MockProvider{baseURL: baseURL}
}

// Name returns the gateway identifier.
func (m *MockProvider) Name() string { return "mock" }

// CreatePayment returns a deterministic external id derived from the order.
func (m *MockProvider) CreatePayment(_ context.Context, in CreatePaymentInput) (CreatePaymentResult, error) {
	return CreatePaymentResult{
		ExternalID: "mock-" + in.OrderID.String(),
		PaymentURL: m.baseURL + "/billing/dev/pay/" + in.OrderID.String(),
		Status:     PaymentCreated,
	}, nil
}

// ParseWebhook reads {"external_id","order_id","status"} with no signature.
func (m *MockProvider) ParseWebhook(_ http.Header, body []byte) (WebhookResult, error) {
	var p struct {
		ExternalID string `json:"external_id"`
		OrderID    string `json:"order_id"`
		Status     string `json:"status"`
	}
	if err := json.Unmarshal(body, &p); err != nil {
		return WebhookResult{}, err
	}
	res := WebhookResult{ExternalID: p.ExternalID, Status: mapMockStatus(p.Status)}
	if p.OrderID != "" {
		if id, err := uuid.Parse(p.OrderID); err == nil {
			res.OrderID = id
		}
	}
	return res, nil
}

func mapMockStatus(s string) PaymentStatus {
	switch s {
	case "success", "paid":
		return PaymentSuccess
	case "failed", "fail", "cancel":
		return PaymentFailed
	default:
		return PaymentPending
	}
}
