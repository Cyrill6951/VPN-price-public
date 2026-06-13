package vpn

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository persists VPN servers, subscriptions, configs and keys.
type Repository struct {
	pool *pgxpool.Pool
}

// NewRepository builds a Repository over the given pgx pool.
func NewRepository(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

const serverColumns = `id, country_id, public_host, agent_url, status,
	wg_port, wg_public_key, wg_subnet::text, wg_dns,
	reality_port, reality_public_key, reality_sni, reality_short_id, reality_dest,
	ss_port, ss_method, ss_server_key, trojan_port`

func scanServer(row pgx.Row) (Server, error) {
	var s Server
	var subnet *string
	err := row.Scan(
		&s.ID, &s.CountryID, &s.PublicHost, &s.AgentURL, &s.Status,
		&s.WGPort, &s.WGPublicKey, &subnet, &s.WGDNS,
		&s.RealityPort, &s.RealityPublicKey, &s.RealitySNI, &s.RealityShortID, &s.RealityDest,
		&s.SSPort, &s.SSMethod, &s.SSServerKey, &s.TrojanPort,
	)
	s.WGSubnet = subnet
	return s, err
}

// SelectServer picks the best active server for a country that supports the
// requested protocol (lowest load first).
func (r *Repository) SelectServer(ctx context.Context, countryID uuid.UUID, protocol Protocol) (Server, error) {
	cond := protocolAvailabilityClause(protocol)
	row := r.pool.QueryRow(ctx, `
		SELECT `+serverColumns+`
		FROM servers
		WHERE country_id = $1 AND status = 'active' AND `+cond+`
		ORDER BY reserve ASC, priority ASC, client_count ASC
		LIMIT 1`, countryID)
	s, err := scanServer(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return Server{}, ErrNoServerAvailable
	}
	if err != nil {
		return Server{}, fmt.Errorf("select server: %w", err)
	}
	return s, nil
}

func protocolAvailabilityClause(p Protocol) string {
	switch p {
	case ProtocolVLESSReality:
		return "reality_port IS NOT NULL AND reality_public_key IS NOT NULL"
	case ProtocolShadowsocks:
		return "ss_port IS NOT NULL AND ss_server_key IS NOT NULL"
	case ProtocolTrojan:
		return "trojan_port IS NOT NULL AND reality_public_key IS NOT NULL"
	default:
		return "wg_port IS NOT NULL AND wg_public_key IS NOT NULL AND wg_subnet IS NOT NULL"
	}
}

// CreateParams carries everything needed to persist a generated VPN.
type CreateParams struct {
	UserID     uuid.UUID
	Server     Server
	CountryID  uuid.UUID
	PlanID     *uuid.UUID
	Protocol   Protocol
	Label      *string
	MaxDevices int
	ExpiresAt  time.Time

	URI             string
	ConfigObjectKey string
	QRObjectKey     string
	Hash            string

	PrivateKeyEnc *string
	PublicKey     *string
	PSKEnc        *string
	ClientUUID    *string
	AssignedIP    *string
}

// Persist inserts the subscription, config and key, and bumps server load —
// atomically. The server row is locked to serialise IP allocation.
func (r *Repository) Persist(ctx context.Context, p CreateParams, configID uuid.UUID) (Subscription, Config, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return Subscription{}, Config{}, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var sub Subscription
	err = tx.QueryRow(ctx, `
		INSERT INTO subscriptions (user_id, plan_id, server_id, country_id, protocol, label, max_devices, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, user_id, server_id, country_id, protocol, status, label, expires_at, created_at`,
		p.UserID, p.PlanID, p.Server.ID, p.CountryID, string(p.Protocol), p.Label, p.MaxDevices, p.ExpiresAt).
		Scan(&sub.ID, &sub.UserID, &sub.ServerID, &sub.CountryID, &sub.Protocol, &sub.Status, &sub.Label, &sub.ExpiresAt, &sub.CreatedAt)
	if err != nil {
		return Subscription{}, Config{}, fmt.Errorf("insert subscription: %w", err)
	}

	var cfg Config
	err = tx.QueryRow(ctx, `
		INSERT INTO vpn_configs (id, subscription_id, protocol, uri, config_object_key, qr_object_key, hash)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, protocol, uri, hash, version, created_at`,
		configID, sub.ID, string(p.Protocol), p.URI, p.ConfigObjectKey, p.QRObjectKey, p.Hash).
		Scan(&cfg.ID, &cfg.Protocol, &cfg.URI, &cfg.Hash, &cfg.Version, &cfg.CreatedAt)
	if err != nil {
		return Subscription{}, Config{}, fmt.Errorf("insert config: %w", err)
	}

	if _, err = tx.Exec(ctx, `
		INSERT INTO vpn_keys (vpn_config_id, server_id, private_key, public_key, psk, client_uuid, assigned_ip)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		cfg.ID, p.Server.ID, p.PrivateKeyEnc, p.PublicKey, p.PSKEnc, p.ClientUUID, p.AssignedIP); err != nil {
		return Subscription{}, Config{}, fmt.Errorf("insert key: %w", err)
	}

	if _, err = tx.Exec(ctx,
		`UPDATE servers SET client_count = client_count + 1 WHERE id = $1`, p.Server.ID); err != nil {
		return Subscription{}, Config{}, fmt.Errorf("bump server load: %w", err)
	}

	if err = tx.Commit(ctx); err != nil {
		return Subscription{}, Config{}, fmt.Errorf("commit: %w", err)
	}
	return sub, cfg, nil
}

// GetPlan returns a plan's billing-relevant fields.
func (r *Repository) GetPlan(ctx context.Context, planID uuid.UUID) (Plan, error) {
	var p Plan
	err := r.pool.QueryRow(ctx,
		`SELECT id, name, days, max_devices FROM plans WHERE id = $1 AND active = true`, planID).
		Scan(&p.ID, &p.Name, &p.Days, &p.MaxDevices)
	if errors.Is(err, pgx.ErrNoRows) {
		return Plan{}, ErrNotFound
	}
	if err != nil {
		return Plan{}, fmt.Errorf("get plan: %w", err)
	}
	return p, nil
}

// CountryRef is a lightweight country entry for the catalogue.
type CountryRef struct {
	ID      uuid.UUID `json:"id"`
	ISO     string    `json:"iso"`
	Name    string    `json:"name"`
	Flag    *string   `json:"flag,omitempty"`
	Servers int       `json:"servers"`
}

// ListCountries returns enabled countries with their active server counts.
func (r *Repository) ListCountries(ctx context.Context) ([]CountryRef, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT c.id, c.iso, c.name, c.flag,
		       (SELECT count(*) FROM servers s WHERE s.country_id = c.id AND s.status = 'active')
		FROM countries c WHERE c.enabled = true
		ORDER BY c.priority ASC, c.name ASC`)
	if err != nil {
		return nil, fmt.Errorf("list countries: %w", err)
	}
	defer rows.Close()
	out := []CountryRef{}
	for rows.Next() {
		var c CountryRef
		if err := rows.Scan(&c.ID, &c.ISO, &c.Name, &c.Flag, &c.Servers); err != nil {
			return nil, fmt.Errorf("scan country: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// PlanRef is a catalogue plan entry.
type PlanRef struct {
	ID         uuid.UUID `json:"id"`
	Name       string    `json:"name"`
	Price      float64   `json:"price"`
	Currency   string    `json:"currency"`
	Days       int       `json:"days"`
	MaxDevices int       `json:"max_devices"`
}

// ListPlans returns active plans.
func (r *Repository) ListPlans(ctx context.Context) ([]PlanRef, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, name, price, currency, days, max_devices
		FROM plans WHERE active = true ORDER BY days ASC`)
	if err != nil {
		return nil, fmt.Errorf("list plans: %w", err)
	}
	defer rows.Close()
	out := []PlanRef{}
	for rows.Next() {
		var p PlanRef
		if err := rows.Scan(&p.ID, &p.Name, &p.Price, &p.Currency, &p.Days, &p.MaxDevices); err != nil {
			return nil, fmt.Errorf("scan plan: %w", err)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// MigrationSub is an active subscription that may need migrating.
type MigrationSub struct {
	SubID      uuid.UUID
	UserID     uuid.UUID
	CountryID  uuid.UUID
	Protocol   Protocol
	Label      *string
	TelegramID *int64
	Country    string
}

// ActiveSubsOnServer lists active subscriptions hosted on a server.
func (r *Repository) ActiveSubsOnServer(ctx context.Context, serverID uuid.UUID) ([]MigrationSub, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT s.id, s.user_id, s.country_id, s.protocol, s.label, u.telegram_id, co.name
		FROM subscriptions s
		JOIN users u ON u.id = s.user_id
		JOIN countries co ON co.id = s.country_id
		WHERE s.server_id = $1 AND s.status = 'active'`, serverID)
	if err != nil {
		return nil, fmt.Errorf("active subs on server: %w", err)
	}
	defer rows.Close()
	out := []MigrationSub{}
	for rows.Next() {
		var m MigrationSub
		if err := rows.Scan(&m.SubID, &m.UserID, &m.CountryID, &m.Protocol, &m.Label, &m.TelegramID, &m.Country); err != nil {
			return nil, fmt.Errorf("scan migration sub: %w", err)
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// GetServerByID loads a server by id.
func (r *Repository) GetServerByID(ctx context.Context, id uuid.UUID) (Server, error) {
	s, err := scanServer(r.pool.QueryRow(ctx, `SELECT `+serverColumns+` FROM servers WHERE id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return Server{}, ErrNotFound
	}
	if err != nil {
		return Server{}, fmt.Errorf("get server: %w", err)
	}
	return s, nil
}

// SelectReserveServer picks an alternative active server in a country for the
// given protocol, preferring reserve servers and excluding excludeID.
func (r *Repository) SelectReserveServer(ctx context.Context, countryID uuid.UUID, protocol Protocol, excludeID uuid.UUID) (Server, error) {
	cond := protocolAvailabilityClause(protocol)
	row := r.pool.QueryRow(ctx, `
		SELECT `+serverColumns+`
		FROM servers
		WHERE country_id = $1 AND status = 'active' AND id <> $2 AND `+cond+`
		ORDER BY reserve DESC, priority ASC, client_count ASC
		LIMIT 1`, countryID, excludeID)
	s, err := scanServer(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return Server{}, ErrNoServerAvailable
	}
	if err != nil {
		return Server{}, fmt.Errorf("select reserve: %w", err)
	}
	return s, nil
}

// OldKeyForSub returns the active key material of a subscription (for deprovision).
func (r *Repository) OldKeyForSub(ctx context.Context, subID uuid.UUID) (publicKey, clientUUID *string, err error) {
	err = r.pool.QueryRow(ctx, `
		SELECT k.public_key, k.client_uuid
		FROM vpn_keys k JOIN vpn_configs c ON c.id = k.vpn_config_id
		WHERE c.subscription_id = $1 AND k.status = 'active'
		ORDER BY k.created_at DESC LIMIT 1`, subID).Scan(&publicKey, &clientUUID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, fmt.Errorf("old key: %w", err)
	}
	return publicKey, clientUUID, nil
}

// Reassign moves a subscription to a new server: revokes old keys, writes a new
// config+key, updates the subscription and server loads, and records the migration.
func (r *Repository) Reassign(ctx context.Context, p CreateParams, configID, subID, oldServerID uuid.UUID, reason string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err = tx.Exec(ctx, `UPDATE vpn_keys SET status='revoked'
		WHERE vpn_config_id IN (SELECT id FROM vpn_configs WHERE subscription_id=$1)`, subID); err != nil {
		return fmt.Errorf("revoke old keys: %w", err)
	}

	var version int
	if err = tx.QueryRow(ctx,
		`SELECT COALESCE(max(version),0)+1 FROM vpn_configs WHERE subscription_id=$1`, subID).Scan(&version); err != nil {
		return fmt.Errorf("next version: %w", err)
	}

	if _, err = tx.Exec(ctx, `
		INSERT INTO vpn_configs (id, subscription_id, protocol, uri, config_object_key, qr_object_key, hash, version)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
		configID, subID, string(p.Protocol), p.URI, p.ConfigObjectKey, p.QRObjectKey, p.Hash, version); err != nil {
		return fmt.Errorf("insert config: %w", err)
	}
	if _, err = tx.Exec(ctx, `
		INSERT INTO vpn_keys (vpn_config_id, server_id, private_key, public_key, psk, client_uuid, assigned_ip)
		VALUES ($1,$2,$3,$4,$5,$6,$7)`,
		configID, p.Server.ID, p.PrivateKeyEnc, p.PublicKey, p.PSKEnc, p.ClientUUID, p.AssignedIP); err != nil {
		return fmt.Errorf("insert key: %w", err)
	}
	if _, err = tx.Exec(ctx, `UPDATE subscriptions SET server_id=$2, status='active' WHERE id=$1`, subID, p.Server.ID); err != nil {
		return fmt.Errorf("update subscription: %w", err)
	}
	if _, err = tx.Exec(ctx, `UPDATE servers SET client_count=client_count+1 WHERE id=$1`, p.Server.ID); err != nil {
		return fmt.Errorf("inc new server: %w", err)
	}
	if _, err = tx.Exec(ctx, `UPDATE servers SET client_count=GREATEST(client_count-1,0) WHERE id=$1`, oldServerID); err != nil {
		return fmt.Errorf("dec old server: %w", err)
	}
	if _, err = tx.Exec(ctx, `
		INSERT INTO migrations (subscription_id, user_id, from_server, to_server, reason)
		VALUES ($1,$2,$3,$4,$5)`, subID, p.UserID, oldServerID, p.Server.ID, reason); err != nil {
		return fmt.Errorf("insert migration: %w", err)
	}

	return tx.Commit(ctx)
}

// ListUsedIPs returns active WireGuard addresses already assigned on a server.
func (r *Repository) ListUsedIPs(ctx context.Context, serverID uuid.UUID) ([]string, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT host(assigned_ip) FROM vpn_keys
		WHERE server_id = $1 AND assigned_ip IS NOT NULL AND status = 'active'`, serverID)
	if err != nil {
		return nil, fmt.Errorf("list used ips: %w", err)
	}
	defer rows.Close()

	ips := []string{}
	for rows.Next() {
		var ip string
		if err := rows.Scan(&ip); err != nil {
			return nil, fmt.Errorf("scan ip: %w", err)
		}
		ips = append(ips, ip)
	}
	return ips, rows.Err()
}

// ListUserVPNs returns a user's active subscriptions with their latest config.
func (r *Repository) ListUserVPNs(ctx context.Context, userID uuid.UUID) ([]VPNView, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT s.id, s.user_id, s.server_id, s.country_id, s.protocol, s.status, s.label,
		       s.expires_at, s.created_at,
		       c.id, c.protocol, c.uri, c.hash, c.version, c.created_at,
		       co.name, srv.public_host
		FROM subscriptions s
		JOIN servers srv ON srv.id = s.server_id
		JOIN countries co ON co.id = s.country_id
		LEFT JOIN LATERAL (
			SELECT * FROM vpn_configs vc WHERE vc.subscription_id = s.id
			ORDER BY vc.version DESC LIMIT 1
		) c ON true
		WHERE s.user_id = $1 AND s.status <> 'deleted'
		ORDER BY s.created_at DESC`, userID)
	if err != nil {
		return nil, fmt.Errorf("list user vpns: %w", err)
	}
	defer rows.Close()

	views := []VPNView{}
	for rows.Next() {
		var v VPNView
		if err := rows.Scan(
			&v.Subscription.ID, &v.Subscription.UserID, &v.Subscription.ServerID,
			&v.Subscription.CountryID, &v.Subscription.Protocol, &v.Subscription.Status,
			&v.Subscription.Label, &v.Subscription.ExpiresAt, &v.Subscription.CreatedAt,
			&v.Config.ID, &v.Config.Protocol, &v.Config.URI, &v.Config.Hash,
			&v.Config.Version, &v.Config.CreatedAt,
			&v.Country, &v.Host,
		); err != nil {
			return nil, fmt.Errorf("scan vpn view: %w", err)
		}
		views = append(views, v)
	}
	return views, rows.Err()
}

// ConfigArtifact identifies stored config/QR objects for a subscription.
type ConfigArtifact struct {
	ConfigObjectKey string
	QRObjectKey     string
	Protocol        Protocol
	URI             string
}

// GetArtifactForUser returns the object keys of a user's subscription config.
func (r *Repository) GetArtifactForUser(ctx context.Context, userID, subscriptionID uuid.UUID) (ConfigArtifact, error) {
	var a ConfigArtifact
	err := r.pool.QueryRow(ctx, `
		SELECT c.config_object_key, c.qr_object_key, c.protocol, c.uri
		FROM vpn_configs c
		JOIN subscriptions s ON s.id = c.subscription_id
		WHERE s.id = $1 AND s.user_id = $2 AND s.status <> 'deleted'
		ORDER BY c.version DESC LIMIT 1`, subscriptionID, userID).
		Scan(&a.ConfigObjectKey, &a.QRObjectKey, &a.Protocol, &a.URI)
	if errors.Is(err, pgx.ErrNoRows) {
		return ConfigArtifact{}, ErrNotFound
	}
	if err != nil {
		return ConfigArtifact{}, fmt.Errorf("get artifact: %w", err)
	}
	return a, nil
}

// Deletion bundles what is needed to deprovision and clean up a deleted VPN.
type Deletion struct {
	Server     Server
	Protocol   Protocol
	PublicKey  *string
	ClientUUID *string
	ObjectKeys []string
}

// SoftDelete marks the subscription deleted, revokes its keys, decrements server
// load and returns the data needed to deprovision the node and purge storage.
func (r *Repository) SoftDelete(ctx context.Context, userID, subscriptionID uuid.UUID) (Deletion, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return Deletion{}, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var serverID uuid.UUID
	var proto Protocol
	err = tx.QueryRow(ctx, `
		SELECT server_id, protocol FROM subscriptions
		WHERE id = $1 AND user_id = $2 AND status <> 'deleted' FOR UPDATE`,
		subscriptionID, userID).Scan(&serverID, &proto)
	if errors.Is(err, pgx.ErrNoRows) {
		return Deletion{}, ErrNotFound
	}
	if err != nil {
		return Deletion{}, fmt.Errorf("lock subscription: %w", err)
	}

	del := Deletion{Protocol: proto}

	rows, err := tx.Query(ctx, `
		SELECT k.public_key, k.client_uuid, c.config_object_key, c.qr_object_key
		FROM vpn_configs c
		JOIN vpn_keys k ON k.vpn_config_id = c.id
		WHERE c.subscription_id = $1`, subscriptionID)
	if err != nil {
		return Deletion{}, fmt.Errorf("load keys: %w", err)
	}
	for rows.Next() {
		var pub, cuuid, ck, qk *string
		if err := rows.Scan(&pub, &cuuid, &ck, &qk); err != nil {
			rows.Close()
			return Deletion{}, fmt.Errorf("scan key: %w", err)
		}
		if del.PublicKey == nil {
			del.PublicKey, del.ClientUUID = pub, cuuid
		}
		if ck != nil {
			del.ObjectKeys = append(del.ObjectKeys, *ck)
		}
		if qk != nil {
			del.ObjectKeys = append(del.ObjectKeys, *qk)
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return Deletion{}, fmt.Errorf("iterate keys: %w", err)
	}

	if _, err = tx.Exec(ctx, `UPDATE vpn_keys SET status = 'revoked'
		WHERE vpn_config_id IN (SELECT id FROM vpn_configs WHERE subscription_id = $1)`, subscriptionID); err != nil {
		return Deletion{}, fmt.Errorf("revoke keys: %w", err)
	}
	if _, err = tx.Exec(ctx, `UPDATE subscriptions SET status = 'deleted' WHERE id = $1`, subscriptionID); err != nil {
		return Deletion{}, fmt.Errorf("mark deleted: %w", err)
	}
	if _, err = tx.Exec(ctx, `UPDATE servers SET client_count = GREATEST(client_count - 1, 0) WHERE id = $1`, serverID); err != nil {
		return Deletion{}, fmt.Errorf("decrement load: %w", err)
	}

	srvRow := tx.QueryRow(ctx, `SELECT `+serverColumns+` FROM servers WHERE id = $1`, serverID)
	if del.Server, err = scanServer(srvRow); err != nil {
		return Deletion{}, fmt.Errorf("load server: %w", err)
	}

	if err = tx.Commit(ctx); err != nil {
		return Deletion{}, fmt.Errorf("commit: %w", err)
	}
	return del, nil
}
