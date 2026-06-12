package billing

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/vpnsaas/platform/internal/vpn"
)

// Repository persists billing entities.
type Repository struct {
	pool *pgxpool.Pool
}

// NewRepository builds a Repository over the given pgx pool.
func NewRepository(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

// PlanPricing holds the billing-relevant plan fields.
type PlanPricing struct {
	Price    float64
	Currency string
	Days     int
}

// GetPlanPricing returns an active plan's price.
func (r *Repository) GetPlanPricing(ctx context.Context, planID uuid.UUID) (PlanPricing, error) {
	var p PlanPricing
	err := r.pool.QueryRow(ctx,
		`SELECT price, currency, days FROM plans WHERE id = $1 AND active = true`, planID).
		Scan(&p.Price, &p.Currency, &p.Days)
	if errors.Is(err, pgx.ErrNoRows) {
		return PlanPricing{}, ErrPlanNotFound
	}
	if err != nil {
		return PlanPricing{}, fmt.Errorf("get plan pricing: %w", err)
	}
	return p, nil
}

// Promo is a validated promo code.
type Promo struct {
	ID        uuid.UUID
	Type      string
	Discount  float64
	ExpiresAt *time.Time
	Active    bool
}

// GetPromoByCode loads a promo code by its code.
func (r *Repository) GetPromoByCode(ctx context.Context, code string) (Promo, error) {
	var p Promo
	err := r.pool.QueryRow(ctx,
		`SELECT id, type, discount, expires_at, active FROM promocodes WHERE code = $1`, code).
		Scan(&p.ID, &p.Type, &p.Discount, &p.ExpiresAt, &p.Active)
	if errors.Is(err, pgx.ErrNoRows) {
		return Promo{}, ErrPromoInvalid
	}
	if err != nil {
		return Promo{}, fmt.Errorf("get promo: %w", err)
	}
	return p, nil
}

// CountRecentPendingOrders counts a user's unpaid orders since a cutoff (anti-fraud).
func (r *Repository) CountRecentPendingOrders(ctx context.Context, userID uuid.UUID, since time.Time) (int, error) {
	var n int
	err := r.pool.QueryRow(ctx, `
		SELECT count(*) FROM orders
		WHERE user_id = $1 AND status IN ('new','wait_payment') AND created_at > $2`,
		userID, since).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("count pending orders: %w", err)
	}
	return n, nil
}

// NewOrder holds the fields needed to create an order.
type NewOrder struct {
	UserID              uuid.UUID
	PlanID              uuid.UUID
	CountryID           uuid.UUID
	Protocol            vpn.Protocol
	Amount              float64
	Discount            float64
	Currency            string
	Gateway             string
	PromoID             *uuid.UUID
	Label               *string
	RenewSubscriptionID *uuid.UUID
	Routing             string
}

// CreateOrder inserts an order and redeems the promo (if any) atomically.
func (r *Repository) CreateOrder(ctx context.Context, in NewOrder) (Order, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return Order{}, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var o Order
	routing := in.Routing
	if routing == "" {
		routing = "full"
	}
	err = tx.QueryRow(ctx, `
		INSERT INTO orders (user_id, plan_id, country_id, protocol, amount, discount, currency, gateway, promo_id, label, renew_subscription_id, routing, status)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,'wait_payment')
		RETURNING id, user_id, plan_id, country_id, protocol, amount, discount, currency, gateway, status, created_at`,
		in.UserID, in.PlanID, in.CountryID, string(in.Protocol), in.Amount, in.Discount,
		in.Currency, in.Gateway, in.PromoID, in.Label, in.RenewSubscriptionID, routing).
		Scan(&o.ID, &o.UserID, &o.PlanID, &o.CountryID, &o.Protocol, &o.Amount, &o.Discount,
			&o.Currency, &o.Gateway, &o.Status, &o.CreatedAt)
	if err != nil {
		return Order{}, fmt.Errorf("insert order: %w", err)
	}

	if in.PromoID != nil {
		var pid uuid.UUID
		err = tx.QueryRow(ctx, `
			UPDATE promocodes SET used_count = used_count + 1
			WHERE id = $1 AND active = true AND (max_uses IS NULL OR used_count < max_uses)
			RETURNING id`, *in.PromoID).Scan(&pid)
		if errors.Is(err, pgx.ErrNoRows) {
			return Order{}, ErrPromoInvalid
		}
		if err != nil {
			return Order{}, fmt.Errorf("redeem promo: %w", err)
		}
		tag, rErr := tx.Exec(ctx, `
			INSERT INTO promo_redemptions (promo_id, user_id, order_id)
			VALUES ($1,$2,$3) ON CONFLICT (promo_id, user_id) DO NOTHING`,
			*in.PromoID, in.UserID, o.ID)
		if rErr != nil {
			return Order{}, fmt.Errorf("insert redemption: %w", rErr)
		}
		if tag.RowsAffected() == 0 {
			return Order{}, ErrPromoUsed
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return Order{}, fmt.Errorf("commit: %w", err)
	}
	return o, nil
}

// CreatePayment records a gateway payment for an order.
func (r *Repository) CreatePayment(ctx context.Context, orderID uuid.UUID, gateway, externalID string, status PaymentStatus, amount float64, currency, url string) (Payment, error) {
	var p Payment
	err := r.pool.QueryRow(ctx, `
		INSERT INTO payments (order_id, gateway, external_id, status, amount, currency, payment_url)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
		RETURNING id, order_id, gateway, COALESCE(external_id,''), status, amount, currency, COALESCE(payment_url,''), created_at`,
		orderID, gateway, externalID, string(status), amount, currency, url).
		Scan(&p.ID, &p.OrderID, &p.Gateway, &p.ExternalID, &p.Status, &p.Amount, &p.Currency, &p.PaymentURL, &p.CreatedAt)
	if err != nil {
		return Payment{}, fmt.Errorf("insert payment: %w", err)
	}
	return p, nil
}

// OrderForProvision carries what is needed to issue a VPN for a paid order.
type OrderForProvision struct {
	ID         uuid.UUID
	UserID     uuid.UUID
	PlanID     uuid.UUID
	CountryID  uuid.UUID
	Protocol   vpn.Protocol
	Status     OrderStatus
	Label      *string
	RenewSubID *uuid.UUID
	Routing    string
}

// GetOrderByExternalPayment resolves the order behind a gateway payment id.
func (r *Repository) GetOrderByExternalPayment(ctx context.Context, gateway, externalID string) (OrderForProvision, error) {
	return r.scanProvision(ctx, `
		SELECT o.id, o.user_id, o.plan_id, o.country_id, o.protocol, o.status, o.label, o.renew_subscription_id, o.routing
		FROM orders o JOIN payments p ON p.order_id = o.id
		WHERE p.gateway = $1 AND p.external_id = $2`, gateway, externalID)
}

// GetOrderForProvision loads an order by id.
func (r *Repository) GetOrderForProvision(ctx context.Context, orderID uuid.UUID) (OrderForProvision, error) {
	return r.scanProvision(ctx, `
		SELECT id, user_id, plan_id, country_id, protocol, status, label, renew_subscription_id, routing
		FROM orders WHERE id = $1`, orderID)
}

func (r *Repository) scanProvision(ctx context.Context, q string, args ...any) (OrderForProvision, error) {
	var o OrderForProvision
	err := r.pool.QueryRow(ctx, q, args...).
		Scan(&o.ID, &o.UserID, &o.PlanID, &o.CountryID, &o.Protocol, &o.Status, &o.Label, &o.RenewSubID, &o.Routing)
	if errors.Is(err, pgx.ErrNoRows) {
		return OrderForProvision{}, ErrOrderNotFound
	}
	if err != nil {
		return OrderForProvision{}, fmt.Errorf("get order: %w", err)
	}
	return o, nil
}

// ExtendSubscription pushes a subscription's expiry forward by the given days,
// reactivating it, and returns the subscription id.
func (r *Repository) ExtendSubscription(ctx context.Context, subID uuid.UUID, days int) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE subscriptions
		SET expires_at = GREATEST(expires_at, now()) + make_interval(days => $2),
		    status = 'active'
		WHERE id = $1 AND status <> 'deleted'`, subID, days)
	if err != nil {
		return fmt.Errorf("extend subscription: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrOrderNotFound
	}
	return nil
}

// ClaimForProvisioning atomically moves an order wait_payment -> processing so
// exactly one webhook delivery provisions the VPN. Returns false if already claimed.
func (r *Repository) ClaimForProvisioning(ctx context.Context, orderID uuid.UUID) (bool, error) {
	var id uuid.UUID
	err := r.pool.QueryRow(ctx,
		`UPDATE orders SET status='processing' WHERE id=$1 AND status='wait_payment' RETURNING id`,
		orderID).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("claim order: %w", err)
	}
	return true, nil
}

// FinalizePaid marks the order paid, the payment successful and appends a ledger
// entry — atomically. Idempotent: a non-claimed order yields ErrOrderNotFound.
func (r *Repository) FinalizePaid(ctx context.Context, orderID, subscriptionID uuid.UUID) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var userID uuid.UUID
	var amount float64
	var currency string
	err = tx.QueryRow(ctx, `
		UPDATE orders SET status = 'paid', paid_at = now(), subscription_id = $2
		WHERE id = $1 AND status IN ('wait_payment','processing')
		RETURNING user_id, amount, currency`, orderID, subscriptionID).
		Scan(&userID, &amount, &currency)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrOrderNotFound
	}
	if err != nil {
		return fmt.Errorf("mark order paid: %w", err)
	}

	if _, err = tx.Exec(ctx,
		`UPDATE payments SET status = 'success' WHERE order_id = $1`, orderID); err != nil {
		return fmt.Errorf("mark payment success: %w", err)
	}

	if err := appendLedger(ctx, tx, userID, "charge", amount, currency, "order", orderID); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}

// MarkFailed sets the order and its payment to failed.
func (r *Repository) MarkFailed(ctx context.Context, orderID uuid.UUID) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err = tx.Exec(ctx, `UPDATE orders SET status='failed' WHERE id=$1 AND status IN ('wait_payment','processing')`, orderID); err != nil {
		return fmt.Errorf("mark order failed: %w", err)
	}
	if _, err = tx.Exec(ctx, `UPDATE payments SET status='failed' WHERE order_id=$1`, orderID); err != nil {
		return fmt.Errorf("mark payment failed: %w", err)
	}
	return tx.Commit(ctx)
}

// appendLedger writes a hash-chained ledger entry within a transaction.
func appendLedger(ctx context.Context, tx pgx.Tx, userID uuid.UUID, ltype string, amount float64, currency, refType string, refID uuid.UUID) error {
	var prevHash string
	err := tx.QueryRow(ctx,
		`SELECT hash FROM ledger WHERE user_id = $1 ORDER BY created_at DESC, id DESC LIMIT 1`, userID).
		Scan(&prevHash)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("read prev ledger: %w", err)
	}
	payload := fmt.Sprintf("%s|%s|%.2f|%s|%s|%s|%s",
		prevHash, ltype, amount, currency, refType, refID, userID)
	sum := sha256.Sum256([]byte(payload))
	hash := hex.EncodeToString(sum[:])

	_, err = tx.Exec(ctx, `
		INSERT INTO ledger (user_id, type, amount, currency, ref_type, ref_id, prev_hash, hash)
		VALUES ($1,$2,$3,$4,$5,$6,NULLIF($7,''),$8)`,
		userID, ltype, amount, currency, refType, refID, prevHash, hash)
	if err != nil {
		return fmt.Errorf("insert ledger: %w", err)
	}
	return nil
}

// RecordWebhook upserts a webhook event for audit (keyed by gateway+external_id).
// Provisioning idempotency is enforced separately via ClaimForProvisioning.
func (r *Repository) RecordWebhook(ctx context.Context, gateway, externalID, status string, payload []byte) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO webhook_events (gateway, external_id, status, payload)
		VALUES ($1,$2,$3,$4)
		ON CONFLICT (gateway, external_id)
		DO UPDATE SET status = EXCLUDED.status, payload = EXCLUDED.payload, received_at = now()`,
		gateway, externalID, status, payload)
	if err != nil {
		return fmt.Errorf("record webhook: %w", err)
	}
	return nil
}

// MarkWebhookProcessed stamps a webhook event as handled.
func (r *Repository) MarkWebhookProcessed(ctx context.Context, gateway, externalID string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE webhook_events SET processed_at = now() WHERE gateway=$1 AND external_id=$2`,
		gateway, externalID)
	return err
}

// OrderSubscriptionID returns the subscription linked to an order (after fulfillment).
func (r *Repository) OrderSubscriptionID(ctx context.Context, orderID uuid.UUID) (uuid.UUID, error) {
	var sub *uuid.UUID
	err := r.pool.QueryRow(ctx, `SELECT subscription_id FROM orders WHERE id = $1`, orderID).Scan(&sub)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, ErrOrderNotFound
	}
	if err != nil {
		return uuid.Nil, fmt.Errorf("get order subscription: %w", err)
	}
	if sub == nil {
		return uuid.Nil, nil
	}
	return *sub, nil
}

// ListOrders returns a user's orders, newest first.
func (r *Repository) ListOrders(ctx context.Context, userID uuid.UUID) ([]Order, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, user_id, plan_id, country_id, protocol, amount, discount, currency,
		       gateway, status, subscription_id, created_at, paid_at
		FROM orders WHERE user_id = $1 ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, fmt.Errorf("list orders: %w", err)
	}
	defer rows.Close()
	out := []Order{}
	for rows.Next() {
		var o Order
		if err := rows.Scan(&o.ID, &o.UserID, &o.PlanID, &o.CountryID, &o.Protocol, &o.Amount,
			&o.Discount, &o.Currency, &o.Gateway, &o.Status, &o.SubscriptionID, &o.CreatedAt, &o.PaidAt); err != nil {
			return nil, fmt.Errorf("scan order: %w", err)
		}
		out = append(out, o)
	}
	return out, rows.Err()
}
