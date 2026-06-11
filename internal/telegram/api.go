// Package telegram implements the Telegram bot: a long-polling client and the
// purchase / management flows (buy, renew, configs, Stars payments).
package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"time"
)

// Client is a minimal Telegram Bot API client.
type Client struct {
	token   string
	baseURL string
	http    *http.Client
}

// NewClient builds a bot API client.
func NewClient(token string) *Client {
	return &Client{
		token:   token,
		baseURL: "https://api.telegram.org/bot" + token,
		http:    &http.Client{Timeout: 70 * time.Second},
	}
}

// --- API types (subset) ---

// Update is a single incoming event.
type Update struct {
	UpdateID         int64             `json:"update_id"`
	Message          *Message          `json:"message"`
	CallbackQuery    *CallbackQuery    `json:"callback_query"`
	PreCheckoutQuery *PreCheckoutQuery `json:"pre_checkout_query"`
}

// Message is a chat message.
type Message struct {
	MessageID         int64              `json:"message_id"`
	From              *User              `json:"from"`
	Chat              Chat               `json:"chat"`
	Text              string             `json:"text"`
	SuccessfulPayment *SuccessfulPayment `json:"successful_payment"`
}

// Chat identifies a conversation.
type Chat struct {
	ID int64 `json:"id"`
}

// User is a Telegram user.
type User struct {
	ID        int64  `json:"id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Username  string `json:"username"`
}

// CallbackQuery is an inline-button press.
type CallbackQuery struct {
	ID      string   `json:"id"`
	From    User     `json:"from"`
	Message *Message `json:"message"`
	Data    string   `json:"data"`
}

// PreCheckoutQuery precedes a successful payment and must be answered.
type PreCheckoutQuery struct {
	ID             string `json:"id"`
	From           User   `json:"from"`
	Currency       string `json:"currency"`
	TotalAmount    int    `json:"total_amount"`
	InvoicePayload string `json:"invoice_payload"`
}

// SuccessfulPayment confirms a completed payment.
type SuccessfulPayment struct {
	Currency                string `json:"currency"`
	TotalAmount             int    `json:"total_amount"`
	InvoicePayload          string `json:"invoice_payload"`
	TelegramPaymentChargeID string `json:"telegram_payment_charge_id"`
}

// InlineKeyboardMarkup is a grid of inline buttons.
type InlineKeyboardMarkup struct {
	InlineKeyboard [][]InlineKeyboardButton `json:"inline_keyboard"`
}

// InlineKeyboardButton is a single inline button.
type InlineKeyboardButton struct {
	Text         string `json:"text"`
	CallbackData string `json:"callback_data,omitempty"`
	Pay          bool   `json:"pay,omitempty"`
}

// LabeledPrice is a line item of an invoice.
type LabeledPrice struct {
	Label  string `json:"label"`
	Amount int    `json:"amount"`
}

// --- methods ---

func (c *Client) call(ctx context.Context, method string, payload any, out any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal %s: %w", method, err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/"+method, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("call %s: %w", method, err)
	}
	defer func() { _ = resp.Body.Close() }()

	var envelope struct {
		OK          bool            `json:"ok"`
		Description string          `json:"description"`
		Result      json.RawMessage `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return fmt.Errorf("decode %s: %w", method, err)
	}
	if !envelope.OK {
		return fmt.Errorf("telegram %s: %s", method, envelope.Description)
	}
	if out != nil {
		return json.Unmarshal(envelope.Result, out)
	}
	return nil
}

// GetUpdates long-polls for new updates starting at offset.
func (c *Client) GetUpdates(ctx context.Context, offset int64, timeout int) ([]Update, error) {
	var updates []Update
	err := c.call(ctx, "getUpdates", map[string]any{
		"offset":          offset,
		"timeout":         timeout,
		"allowed_updates": []string{"message", "callback_query", "pre_checkout_query"},
	}, &updates)
	return updates, err
}

// SendMessage sends a text message with an optional inline keyboard.
func (c *Client) SendMessage(ctx context.Context, chatID int64, text string, kb *InlineKeyboardMarkup) error {
	payload := map[string]any{"chat_id": chatID, "text": text, "parse_mode": "HTML", "disable_web_page_preview": true}
	if kb != nil {
		payload["reply_markup"] = kb
	}
	return c.call(ctx, "sendMessage", payload, nil)
}

// EditMessageText replaces the text and keyboard of an existing message.
func (c *Client) EditMessageText(ctx context.Context, chatID, messageID int64, text string, kb *InlineKeyboardMarkup) error {
	payload := map[string]any{"chat_id": chatID, "message_id": messageID, "text": text, "parse_mode": "HTML", "disable_web_page_preview": true}
	if kb != nil {
		payload["reply_markup"] = kb
	}
	return c.call(ctx, "editMessageText", payload, nil)
}

// AnswerCallbackQuery acknowledges a button press.
func (c *Client) AnswerCallbackQuery(ctx context.Context, id, text string) error {
	return c.call(ctx, "answerCallbackQuery", map[string]any{"callback_query_id": id, "text": text}, nil)
}

// SendInvoice sends a Telegram Stars invoice (currency XTR, empty provider token).
func (c *Client) SendInvoice(ctx context.Context, chatID int64, title, description, payload string, prices []LabeledPrice) error {
	return c.call(ctx, "sendInvoice", map[string]any{
		"chat_id":        chatID,
		"title":          title,
		"description":    description,
		"payload":        payload,
		"provider_token": "",
		"currency":       "XTR",
		"prices":         prices,
	}, nil)
}

// AnswerPreCheckoutQuery confirms (or rejects) a pending payment.
func (c *Client) AnswerPreCheckoutQuery(ctx context.Context, id string, ok bool, errMsg string) error {
	payload := map[string]any{"pre_checkout_query_id": id, "ok": ok}
	if !ok {
		payload["error_message"] = errMsg
	}
	return c.call(ctx, "answerPreCheckoutQuery", payload, nil)
}

// SendPhoto uploads a photo (e.g. a QR code).
func (c *Client) SendPhoto(ctx context.Context, chatID int64, name string, data []byte, caption string) error {
	return c.upload(ctx, "sendPhoto", chatID, "photo", name, data, caption)
}

// SendDocument uploads a document (e.g. a WireGuard .conf).
func (c *Client) SendDocument(ctx context.Context, chatID int64, name string, data []byte, caption string) error {
	return c.upload(ctx, "sendDocument", chatID, "document", name, data, caption)
}

func (c *Client) upload(ctx context.Context, method string, chatID int64, field, name string, data []byte, caption string) error {
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	_ = mw.WriteField("chat_id", fmt.Sprintf("%d", chatID))
	if caption != "" {
		_ = mw.WriteField("caption", caption)
		_ = mw.WriteField("parse_mode", "HTML")
	}
	fw, err := mw.CreateFormFile(field, name)
	if err != nil {
		return err
	}
	if _, err := fw.Write(data); err != nil {
		return err
	}
	_ = mw.Close()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/"+method, &buf)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("upload %s: %w", method, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("upload %s: status %d: %s", method, resp.StatusCode, string(b))
	}
	return nil
}
