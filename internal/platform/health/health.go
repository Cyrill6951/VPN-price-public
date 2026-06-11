// Package health exposes liveness and readiness HTTP handlers.
package health

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

// Checker reports liveness and readiness of the service dependencies.
type Checker struct {
	db    *pgxpool.Pool
	redis *redis.Client
}

// New builds a Checker over the given dependencies.
func New(db *pgxpool.Pool, rdb *redis.Client) *Checker {
	return &Checker{db: db, redis: rdb}
}

// Live always returns 200: the process is up and serving.
func (c *Checker) Live(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// Ready verifies that Postgres and Redis are reachable.
func (c *Checker) Ready(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	checks := map[string]string{}
	code := http.StatusOK

	if err := c.db.Ping(ctx); err != nil {
		checks["postgres"] = "down: " + err.Error()
		code = http.StatusServiceUnavailable
	} else {
		checks["postgres"] = "ok"
	}

	if err := c.redis.Ping(ctx).Err(); err != nil {
		checks["redis"] = "down: " + err.Error()
		code = http.StatusServiceUnavailable
	} else {
		checks["redis"] = "ok"
	}

	status := "ok"
	if code != http.StatusOK {
		status = "degraded"
	}
	writeJSON(w, code, map[string]any{"status": status, "checks": checks})
}

func writeJSON(w http.ResponseWriter, code int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(body)
}
