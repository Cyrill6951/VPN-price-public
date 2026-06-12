package admin

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrNotFound is returned when a target record does not exist.
var ErrNotFound = errors.New("not found")

// Repository provides read/write access for the CMS.
type Repository struct {
	pool *pgxpool.Pool
}

// NewRepository builds a Repository over the given pgx pool.
func NewRepository(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

// Dashboard returns aggregate platform metrics.
func (r *Repository) Dashboard(ctx context.Context) (Dashboard, error) {
	var d Dashboard
	err := r.pool.QueryRow(ctx, `
		SELECT
			(SELECT count(*) FROM users WHERE deleted_at IS NULL),
			(SELECT count(*) FROM subscriptions WHERE status = 'active'),
			(SELECT count(*) FROM servers WHERE status = 'active'),
			(SELECT count(*) FROM servers),
			(SELECT count(*) FROM orders WHERE status = 'paid'),
			(SELECT count(*) FROM orders WHERE created_at >= date_trunc('day', now())),
			(SELECT COALESCE(sum(amount), 0) FROM orders WHERE status = 'paid')
	`).Scan(&d.Users, &d.ActiveSubscriptions, &d.ServersActive, &d.ServersTotal,
		&d.PaidOrders, &d.OrdersToday, &d.RevenueTotal)
	if err != nil {
		return Dashboard{}, fmt.Errorf("dashboard: %w", err)
	}
	return d, nil
}

// ListUsers returns users matching an optional search query.
func (r *Repository) ListUsers(ctx context.Context, q string, limit, offset int) ([]UserRow, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT u.id, u.email, u.phone, u.telegram_id, u.role, u.status, u.created_at,
		       (SELECT count(*) FROM subscriptions s WHERE s.user_id = u.id AND s.status <> 'deleted')
		FROM users u
		WHERE u.deleted_at IS NULL
		  AND ($1 = '' OR u.email ILIKE '%'||$1||'%' OR u.phone ILIKE '%'||$1||'%')
		ORDER BY u.created_at DESC
		LIMIT $2 OFFSET $3`, q, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	defer rows.Close()
	out := []UserRow{}
	for rows.Next() {
		var u UserRow
		if err := rows.Scan(&u.ID, &u.Email, &u.Phone, &u.TelegramID, &u.Role, &u.Status, &u.CreatedAt, &u.Subscriptions); err != nil {
			return nil, fmt.Errorf("scan user: %w", err)
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

// SetUserStatus updates a user's account status.
func (r *Repository) SetUserStatus(ctx context.Context, id uuid.UUID, status string) error {
	return r.execAffect(ctx, `UPDATE users SET status = $2 WHERE id = $1 AND deleted_at IS NULL`, id, status)
}

// SetUserRole updates a user's role.
func (r *Repository) SetUserRole(ctx context.Context, id uuid.UUID, role string) error {
	return r.execAffect(ctx, `UPDATE users SET role = $2 WHERE id = $1 AND deleted_at IS NULL`, id, role)
}

// ListServers returns all servers with their country name.
func (r *Repository) ListServers(ctx context.Context) ([]ServerRow, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT s.id, co.name, s.hostname, s.public_host, s.status,
		       s.client_count, s.capacity, s.priority, s.reserve
		FROM servers s JOIN countries co ON co.id = s.country_id
		ORDER BY co.name, s.hostname`)
	if err != nil {
		return nil, fmt.Errorf("list servers: %w", err)
	}
	defer rows.Close()
	out := []ServerRow{}
	for rows.Next() {
		var s ServerRow
		if err := rows.Scan(&s.ID, &s.Country, &s.Hostname, &s.PublicHost, &s.Status,
			&s.ClientCount, &s.Capacity, &s.Priority, &s.Reserve); err != nil {
			return nil, fmt.Errorf("scan server: %w", err)
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// SetServerStatus updates a server's status (active|drain|offline|archived).
func (r *Repository) SetServerStatus(ctx context.Context, id uuid.UUID, status string) error {
	return r.execAffect(ctx, `UPDATE servers SET status = $2 WHERE id = $1`, id, status)
}

// ListOrders returns recent orders with the buyer's email.
func (r *Repository) ListOrders(ctx context.Context, limit, offset int) ([]OrderRow, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT o.id, u.email, o.amount, o.currency, o.status, o.gateway, o.protocol, o.created_at, o.paid_at
		FROM orders o JOIN users u ON u.id = o.user_id
		ORDER BY o.created_at DESC
		LIMIT $1 OFFSET $2`, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list orders: %w", err)
	}
	defer rows.Close()
	out := []OrderRow{}
	for rows.Next() {
		var o OrderRow
		if err := rows.Scan(&o.ID, &o.UserEmail, &o.Amount, &o.Currency, &o.Status, &o.Gateway, &o.Protocol, &o.CreatedAt, &o.PaidAt); err != nil {
			return nil, fmt.Errorf("scan order: %w", err)
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

// ListPayments returns recent payments.
func (r *Repository) ListPayments(ctx context.Context, limit int) ([]PaymentRow, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, order_id, gateway, status, amount, currency, created_at
		FROM payments ORDER BY created_at DESC LIMIT $1`, limit)
	if err != nil {
		return nil, fmt.Errorf("list payments: %w", err)
	}
	defer rows.Close()
	out := []PaymentRow{}
	for rows.Next() {
		var p PaymentRow
		if err := rows.Scan(&p.ID, &p.OrderID, &p.Gateway, &p.Status, &p.Amount, &p.Currency, &p.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan payment: %w", err)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// ListPromos returns all promo codes.
func (r *Repository) ListPromos(ctx context.Context) ([]PromoRow, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, code, type, discount, used_count, max_uses, active, expires_at
		FROM promocodes ORDER BY created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("list promos: %w", err)
	}
	defer rows.Close()
	out := []PromoRow{}
	for rows.Next() {
		var p PromoRow
		if err := rows.Scan(&p.ID, &p.Code, &p.Type, &p.Discount, &p.UsedCount, &p.MaxUses, &p.Active, &p.ExpiresAt); err != nil {
			return nil, fmt.Errorf("scan promo: %w", err)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// CreatePromo inserts a promo code.
func (r *Repository) CreatePromo(ctx context.Context, code, ptype string, discount float64, maxUses *int) (PromoRow, error) {
	var p PromoRow
	err := r.pool.QueryRow(ctx, `
		INSERT INTO promocodes (code, type, discount, max_uses, active)
		VALUES ($1, $2, $3, $4, true)
		RETURNING id, code, type, discount, used_count, max_uses, active, expires_at`,
		code, ptype, discount, maxUses).
		Scan(&p.ID, &p.Code, &p.Type, &p.Discount, &p.UsedCount, &p.MaxUses, &p.Active, &p.ExpiresAt)
	if err != nil {
		return PromoRow{}, fmt.Errorf("create promo: %w", err)
	}
	return p, nil
}

// ListAudit returns recent audit-log entries with the actor's email.
func (r *Repository) ListAudit(ctx context.Context, limit int) ([]AuditRow, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT a.id, COALESCE(a.actor_email, u.email), a.action, a.object_type, a.object_id, a.created_at
		FROM audit_log a LEFT JOIN users u ON u.id = a.actor_id
		ORDER BY a.created_at DESC LIMIT $1`, limit)
	if err != nil {
		return nil, fmt.Errorf("list audit: %w", err)
	}
	defer rows.Close()
	out := []AuditRow{}
	for rows.Next() {
		var a AuditRow
		if err := rows.Scan(&a.ID, &a.ActorEmail, &a.Action, &a.ObjectType, &a.ObjectID, &a.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan audit: %w", err)
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// InsertAudit records an administrative action.
func (r *Repository) InsertAudit(ctx context.Context, actorID uuid.UUID, action, objType, objID string) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO audit_log (actor_id, action, object_type, object_id)
		VALUES ($1, $2, NULLIF($3,''), NULLIF($4,''))`, actorID, action, objType, objID)
	if err != nil {
		return fmt.Errorf("insert audit: %w", err)
	}
	return nil
}

func (r *Repository) execAffect(ctx context.Context, q string, args ...any) error {
	tag, err := r.pool.Exec(ctx, q, args...)
	if err != nil {
		return fmt.Errorf("exec: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
