package billing

import (
	"bytes"
	"context"
	"crypto/md5" //nolint:gosec // Cryptomus mandates MD5 for its signature scheme
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"
)

// CryptomusProvider integrates the Cryptomus crypto payment gateway.
//
// Signature scheme (per Cryptomus docs): sign = md5(base64(body) + API_KEY).
// The code compiles and follows the documented API; validate against a live
// merchant account before enabling in production.
type CryptomusProvider struct {
	merchant string
	apiKey   string
	client   *http.Client
}

// NewCryptomusProvider builds the provider from merchant credentials.
func NewCryptomusProvider(merchant, apiKey string) *CryptomusProvider {
	return &CryptomusProvider{
		merchant: merchant,
		apiKey:   apiKey,
		client:   &http.Client{Timeout: 15 * time.Second},
	}
}

// Name returns the gateway identifier.
func (c *CryptomusProvider) Name() string { return "cryptomus" }

func (c *CryptomusProvider) sign(body []byte) string {
	enc := base64.StdEncoding.EncodeToString(body)
	sum := md5.Sum([]byte(enc + c.apiKey)) //nolint:gosec // required by Cryptomus
	return hex.EncodeToString(sum[:])
}

// CreatePayment registers an invoice and returns the hosted payment URL.
func (c *CryptomusProvider) CreatePayment(ctx context.Context, in CreatePaymentInput) (CreatePaymentResult, error) {
	reqBody, err := json.Marshal(map[string]any{
		"amount":       strconv.FormatFloat(in.Amount, 'f', 2, 64),
		"currency":     in.Currency,
		"order_id":     in.OrderID.String(),
		"url_callback": in.CallbackURL,
		"url_return":   in.ReturnURL,
	})
	if err != nil {
		return CreatePaymentResult{}, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://api.cryptomus.com/v1/payment", bytes.NewReader(reqBody))
	if err != nil {
		return CreatePaymentResult{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("merchant", c.merchant)
	req.Header.Set("sign", c.sign(reqBody))

	resp, err := c.client.Do(req)
	if err != nil {
		return CreatePaymentResult{}, fmt.Errorf("cryptomus request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	var out struct {
		Result struct {
			UUID string `json:"uuid"`
			URL  string `json:"url"`
		} `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return CreatePaymentResult{}, fmt.Errorf("cryptomus decode: %w", err)
	}
	if out.Result.UUID == "" {
		return CreatePaymentResult{}, fmt.Errorf("cryptomus: empty payment id (status %d)", resp.StatusCode)
	}
	return CreatePaymentResult{
		ExternalID: out.Result.UUID,
		PaymentURL: out.Result.URL,
		Status:     PaymentPending,
	}, nil
}

// ParseWebhook verifies the signature and normalizes the payment status.
func (c *CryptomusProvider) ParseWebhook(_ http.Header, body []byte) (WebhookResult, error) {
	var raw map[string]any
	if err := json.Unmarshal(body, &raw); err != nil {
		return WebhookResult{}, err
	}
	sign, _ := raw["sign"].(string)
	delete(raw, "sign")
	reencoded, err := json.Marshal(raw)
	if err != nil {
		return WebhookResult{}, err
	}
	if sign == "" || sign != c.sign(reencoded) {
		return WebhookResult{}, ErrInvalidSignature
	}

	res := WebhookResult{Status: PaymentPending}
	if v, ok := raw["uuid"].(string); ok {
		res.ExternalID = v
	}
	if v, ok := raw["status"].(string); ok {
		res.Status = mapCryptomusStatus(v)
	}
	return res, nil
}

func mapCryptomusStatus(s string) PaymentStatus {
	switch s {
	case "paid", "paid_over":
		return PaymentSuccess
	case "fail", "cancel", "system_fail", "wrong_amount":
		return PaymentFailed
	default:
		return PaymentPending
	}
}
