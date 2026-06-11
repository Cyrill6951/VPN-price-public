package billing

import (
	"context"
	"net/http"
	"testing"

	"github.com/google/uuid"
)

func TestRound2(t *testing.T) {
	cases := map[float64]float64{
		4.995:  5.0,
		4.994:  4.99,
		0:      0,
		12.999: 13.0,
	}
	for in, want := range cases {
		if got := round2(in); got != want {
			t.Errorf("round2(%v) = %v, want %v", in, got, want)
		}
	}
}

func TestMockProvider(t *testing.T) {
	p := NewMockProvider("http://localhost:8080")
	if p.Name() != "mock" {
		t.Fatalf("name = %q", p.Name())
	}
	orderID := uuid.New()
	res, err := p.CreatePayment(context.Background(), CreatePaymentInput{OrderID: orderID, Amount: 4.99, Currency: "USD"})
	if err != nil {
		t.Fatalf("CreatePayment: %v", err)
	}
	if res.ExternalID != "mock-"+orderID.String() {
		t.Errorf("external id = %q", res.ExternalID)
	}

	body := []byte(`{"external_id":"mock-x","order_id":"` + orderID.String() + `","status":"success"}`)
	wr, err := p.ParseWebhook(http.Header{}, body)
	if err != nil {
		t.Fatalf("ParseWebhook: %v", err)
	}
	if wr.Status != PaymentSuccess {
		t.Errorf("status = %q, want success", wr.Status)
	}
	if wr.OrderID != orderID {
		t.Errorf("order id mismatch")
	}
}

func TestMapCryptomusStatus(t *testing.T) {
	cases := map[string]PaymentStatus{
		"paid":      PaymentSuccess,
		"paid_over": PaymentSuccess,
		"fail":      PaymentFailed,
		"cancel":    PaymentFailed,
		"check":     PaymentPending,
	}
	for in, want := range cases {
		if got := mapCryptomusStatus(in); got != want {
			t.Errorf("mapCryptomusStatus(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestCryptomusSignDeterministic(t *testing.T) {
	c := NewCryptomusProvider("merchant", "key")
	s1 := c.sign([]byte(`{"a":1}`))
	s2 := c.sign([]byte(`{"a":1}`))
	if s1 != s2 || s1 == "" {
		t.Errorf("sign must be deterministic and non-empty: %q %q", s1, s2)
	}
}
