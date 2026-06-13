// Package monitoring polls VPN nodes for health, computes a health score, records
// it, and triggers automatic client migration when a node fails or is blocked.
package monitoring

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/vpnsaas/platform/internal/vpn"
)

// Migrator re-issues subscriptions off a failed server (implemented by vpn.Service).
type Migrator interface {
	MigrateServer(ctx context.Context, fromServerID uuid.UUID, reason string) ([]vpn.MigrationResult, error)
}

// Notifier delivers a migration notice to a user (e.g. via Telegram).
type Notifier interface {
	NotifyMigration(ctx context.Context, telegramID int64, country string)
}

// Engine is the background health/migration poller.
type Engine struct {
	pool     *pgxpool.Pool
	migrator Migrator
	notifier Notifier
	token    string
	client   *http.Client
	interval time.Duration
	log      *slog.Logger
}

// NewEngine builds the monitoring engine.
func NewEngine(pool *pgxpool.Pool, migrator Migrator, notifier Notifier, agentToken string, interval time.Duration, log *slog.Logger) *Engine {
	return &Engine{
		pool:     pool,
		migrator: migrator,
		notifier: notifier,
		token:    agentToken,
		client:   &http.Client{Timeout: 20 * time.Second},
		interval: interval,
		log:      log,
	}
}

// agentStatus mirrors the node agent's GET /status response.
type agentStatus struct {
	CPULoad    float64         `json:"cpu_load"`
	MemUsedPct float64         `json:"mem_used_pct"`
	WGPeers    int             `json:"wg_peers"`
	XrayUp     bool            `json:"xray_up"`
	Blocks     map[string]bool `json:"blocks"`
}

type serverRow struct {
	id       uuid.UUID
	agentURL string
	status   string
}

// Run polls until the context is cancelled.
func (e *Engine) Run(ctx context.Context) {
	e.log.Info("monitoring engine started", "interval", e.interval)
	ticker := time.NewTicker(e.interval)
	defer ticker.Stop()
	e.sweep(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			e.sweep(ctx)
		}
	}
}

func (e *Engine) sweep(ctx context.Context) {
	rows, err := e.pool.Query(ctx, `
		SELECT id, COALESCE(agent_url,''), status FROM servers
		WHERE status <> 'archived' AND agent_url IS NOT NULL`)
	if err != nil {
		e.log.Warn("monitoring: list servers failed", "error", err)
		return
	}
	var servers []serverRow
	for rows.Next() {
		var s serverRow
		if err := rows.Scan(&s.id, &s.agentURL, &s.status); err == nil {
			servers = append(servers, s)
		}
	}
	rows.Close()

	for _, s := range servers {
		e.check(ctx, s)
	}
}

func (e *Engine) check(ctx context.Context, s serverRow) {
	st, err := e.poll(ctx, s.agentURL)
	if err != nil {
		// Node unreachable → offline; migrate its clients to a reserve.
		e.record(ctx, s.id, 0, "offline", "unreachable: "+err.Error(), nil)
		e.markStatus(ctx, s.id, "offline")
		if s.status == "active" {
			e.migrate(ctx, s.id, "node unreachable")
		}
		return
	}

	score, blocked := scoreHealth(st)
	hstatus := "active" // health label (free text for the record)
	switch {
	case score <= 0:
		hstatus = "offline"
	case score < 60:
		hstatus = "degraded"
	}
	reason := ""
	if len(blocked) > 0 {
		reason = "blocked: " + strings.Join(blocked, ",")
	}
	e.record(ctx, s.id, score, hstatus, reason, &st)
	e.markHealth(ctx, s.id, score)

	// Egress blocking of multiple key targets → migrate clients to a clean node.
	if len(blocked) >= 2 {
		e.markStatus(ctx, s.id, "offline")
		if s.status == "active" {
			e.migrate(ctx, s.id, reason)
		}
		return
	}

	// Healthy again → recover a server that was previously auto-marked offline.
	if score >= 60 && s.status == "offline" {
		e.markStatus(ctx, s.id, "active")
	}
}

func (e *Engine) poll(ctx context.Context, agentURL string) (agentStatus, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(agentURL, "/")+"/status", nil)
	if err != nil {
		return agentStatus{}, err
	}
	req.Header.Set("Authorization", "Bearer "+e.token)
	resp, err := e.client.Do(req)
	if err != nil {
		return agentStatus{}, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 300 {
		return agentStatus{}, fmt.Errorf("agent status %d", resp.StatusCode)
	}
	var st agentStatus
	if err := json.NewDecoder(resp.Body).Decode(&st); err != nil {
		return agentStatus{}, err
	}
	return st, nil
}

func (e *Engine) migrate(ctx context.Context, serverID uuid.UUID, reason string) {
	results, err := e.migrator.MigrateServer(ctx, serverID, reason)
	if err != nil {
		e.log.Warn("migration failed", "server", serverID, "error", err)
		return
	}
	if e.notifier != nil {
		for _, r := range results {
			if r.TelegramID != nil {
				e.notifier.NotifyMigration(ctx, *r.TelegramID, r.Country)
			}
		}
	}
}

func (e *Engine) record(ctx context.Context, serverID uuid.UUID, score int, status, reason string, st *agentStatus) {
	payload := []byte("{}")
	if st != nil {
		payload, _ = json.Marshal(st)
	}
	_, err := e.pool.Exec(ctx, `
		INSERT INTO server_health (server_id, score, status, reason, payload)
		VALUES ($1,$2,$3,NULLIF($4,''),$5)`, serverID, score, status, reason, payload)
	if err != nil {
		e.log.Warn("record health failed", "error", err)
	}
}

func (e *Engine) markHealth(ctx context.Context, serverID uuid.UUID, score int) {
	_, _ = e.pool.Exec(ctx,
		`UPDATE servers SET health_score=$2, last_seen=now() WHERE id=$1`, serverID, score)
}

func (e *Engine) markStatus(ctx context.Context, serverID uuid.UUID, status string) {
	_, _ = e.pool.Exec(ctx, `UPDATE servers SET status=$2 WHERE id=$1 AND status<>$2`, serverID, status)
}

// scoreHealth computes a 0..100 score and the list of blocked targets.
func scoreHealth(st agentStatus) (int, []string) {
	score := 100
	if !st.XrayUp {
		score -= 40
	}
	var blocked []string
	for name, ok := range st.Blocks {
		if !ok {
			blocked = append(blocked, name)
			score -= 15
		}
	}
	if st.CPULoad > 0.9 {
		score -= 20
	}
	if st.MemUsedPct > 90 {
		score -= 15
	}
	if score < 0 {
		score = 0
	}
	return score, blocked
}
