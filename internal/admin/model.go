// Package admin implements the CMS backend: dashboard, user/server/order
// management and an audit trail, all restricted to admin roles.
package admin

import (
	"time"

	"github.com/google/uuid"
)

// Dashboard holds top-level platform metrics.
type Dashboard struct {
	Users               int     `json:"users"`
	ActiveSubscriptions int     `json:"active_subscriptions"`
	ServersActive       int     `json:"servers_active"`
	ServersTotal        int     `json:"servers_total"`
	PaidOrders          int     `json:"paid_orders"`
	OrdersToday         int     `json:"orders_today"`
	RevenueTotal        float64 `json:"revenue_total"`
}

// UserRow is a user list entry.
type UserRow struct {
	ID            uuid.UUID `json:"id"`
	Email         *string   `json:"email,omitempty"`
	Phone         *string   `json:"phone,omitempty"`
	TelegramID    *int64    `json:"telegram_id,omitempty"`
	Role          string    `json:"role"`
	Status        string    `json:"status"`
	Subscriptions int       `json:"subscriptions"`
	CreatedAt     time.Time `json:"created_at"`
}

// ServerRow is a server list entry.
type ServerRow struct {
	ID          uuid.UUID `json:"id"`
	Country     string    `json:"country"`
	Hostname    string    `json:"hostname"`
	PublicHost  string    `json:"public_host"`
	Status      string    `json:"status"`
	ClientCount int       `json:"client_count"`
	Capacity    int       `json:"capacity"`
	Priority    int       `json:"priority"`
	Reserve     bool      `json:"reserve"`
}

// OrderRow is an order list entry.
type OrderRow struct {
	ID        uuid.UUID  `json:"id"`
	UserEmail *string    `json:"user_email,omitempty"`
	Amount    float64    `json:"amount"`
	Currency  string     `json:"currency"`
	Status    string     `json:"status"`
	Gateway   string     `json:"gateway"`
	Protocol  string     `json:"protocol"`
	CreatedAt time.Time  `json:"created_at"`
	PaidAt    *time.Time `json:"paid_at,omitempty"`
}

// PaymentRow is a payment list entry.
type PaymentRow struct {
	ID        uuid.UUID `json:"id"`
	OrderID   uuid.UUID `json:"order_id"`
	Gateway   string    `json:"gateway"`
	Status    string    `json:"status"`
	Amount    float64   `json:"amount"`
	Currency  string    `json:"currency"`
	CreatedAt time.Time `json:"created_at"`
}

// PromoRow is a promo code list entry.
type PromoRow struct {
	ID        uuid.UUID  `json:"id"`
	Code      string     `json:"code"`
	Type      string     `json:"type"`
	Discount  float64    `json:"discount"`
	UsedCount int        `json:"used_count"`
	MaxUses   *int       `json:"max_uses,omitempty"`
	Active    bool       `json:"active"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}

// AuditRow is an audit-log entry with the actor's email.
type AuditRow struct {
	ID         uuid.UUID `json:"id"`
	ActorEmail *string   `json:"actor_email,omitempty"`
	Action     string    `json:"action"`
	ObjectType *string   `json:"object_type,omitempty"`
	ObjectID   *string   `json:"object_id,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}
