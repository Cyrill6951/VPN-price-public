package billing

import (
	"context"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Expirer periodically expires subscriptions whose grace period has elapsed.
type Expirer struct {
	pool     *pgxpool.Pool
	grace    time.Duration
	interval time.Duration
	log      *slog.Logger
}

// NewExpirer builds the background expirer.
func NewExpirer(pool *pgxpool.Pool, grace, interval time.Duration, log *slog.Logger) *Expirer {
	return &Expirer{pool: pool, grace: grace, interval: interval, log: log}
}

// Run ticks until the context is cancelled, expiring overdue subscriptions.
func (e *Expirer) Run(ctx context.Context) {
	ticker := time.NewTicker(e.interval)
	defer ticker.Stop()
	e.sweep(ctx) // run once on start
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			e.sweep(ctx)
		}
	}
}

func (e *Expirer) sweep(ctx context.Context) {
	tag, err := e.pool.Exec(ctx, `
		UPDATE subscriptions SET status = 'expired'
		WHERE status = 'active' AND expires_at < now() - make_interval(secs => $1)`,
		e.grace.Seconds())
	if err != nil {
		e.log.Warn("subscription sweep failed", "error", err)
		return
	}
	if n := tag.RowsAffected(); n > 0 {
		e.log.Info("subscriptions expired", "count", n)
	}
}
