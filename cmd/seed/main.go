// Command seed inserts development catalogue data: countries, plans and one
// VPN server with freshly generated WireGuard + Reality keys.
//
// Idempotent: safe to run repeatedly. The server's *private* keys are printed
// once — use them when provisioning the real node (they are not stored in DB).
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/vpnsaas/platform/internal/vpn"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "seed error:", err)
		os.Exit(1)
	}
}

func run() error {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		return fmt.Errorf("DATABASE_URL is required")
	}
	host := getenv("SEED_SERVER_HOST", "203.0.113.10") // TEST-NET-3 placeholder

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return err
	}
	defer pool.Close()

	countryID, err := upsertCountry(ctx, pool, "DE", "Germany", "🇩🇪")
	if err != nil {
		return err
	}
	fmt.Println("country DE:", countryID)

	if err := ensurePlans(ctx, pool); err != nil {
		return err
	}
	fmt.Println("plans ensured")

	if err := ensureServer(ctx, pool, countryID, host); err != nil {
		return err
	}
	return nil
}

func upsertCountry(ctx context.Context, pool *pgxpool.Pool, iso, name, flag string) (uuid.UUID, error) {
	var id uuid.UUID
	err := pool.QueryRow(ctx, `
		INSERT INTO countries (iso, name, flag, enabled, priority)
		VALUES ($1, $2, $3, true, 10)
		ON CONFLICT (iso) DO UPDATE SET name = EXCLUDED.name, flag = EXCLUDED.flag
		RETURNING id`, iso, name, flag).Scan(&id)
	return id, err
}

func ensurePlans(ctx context.Context, pool *pgxpool.Pool) error {
	var count int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM plans`).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	plans := []struct {
		name       string
		price      float64
		days       int
		maxDevices int
	}{
		{"Trial", 0, 1, 1},
		{"Monthly", 4.99, 30, 3},
		{"Quarterly", 12.99, 90, 5},
		{"Yearly", 39.99, 365, 5},
	}
	for _, p := range plans {
		if _, err := pool.Exec(ctx, `
			INSERT INTO plans (name, price, currency, days, max_devices, active)
			VALUES ($1, $2, 'USD', $3, $4, true)`, p.name, p.price, p.days, p.maxDevices); err != nil {
			return err
		}
	}
	return nil
}

func ensureServer(ctx context.Context, pool *pgxpool.Pool, countryID uuid.UUID, host string) error {
	var exists bool
	if err := pool.QueryRow(ctx,
		`SELECT exists(SELECT 1 FROM servers WHERE hostname = 'de-dev-01')`).Scan(&exists); err != nil {
		return err
	}
	if exists {
		fmt.Println("server de-dev-01 already present")
		return nil
	}

	wgPriv, wgPub, err := vpn.GenerateWireGuardKeypair()
	if err != nil {
		return err
	}
	realPriv, realPub, err := vpn.GenerateRealityKeypair()
	if err != nil {
		return err
	}
	shortID, err := vpn.NewShortID()
	if err != nil {
		return err
	}
	ssKey, err := vpn.GenerateShadowsocksKey()
	if err != nil {
		return err
	}

	_, err = pool.Exec(ctx, `
		INSERT INTO servers (
			country_id, provider, hostname, public_host, status, priority, capacity,
			wg_port, wg_public_key, wg_subnet, wg_dns,
			reality_port, reality_public_key, reality_sni, reality_short_id, reality_dest,
			ss_port, ss_method, ss_server_key, trojan_port
		) VALUES (
			$1, 'dev', 'de-dev-01', $2, 'active', 10, 1000,
			51820, $3, '10.7.0.0/24', '1.1.1.1',
			443, $4, 'www.microsoft.com', $5, 'www.microsoft.com:443',
			8388, '2022-blake3-aes-128-gcm', $6, 8443
		)`, countryID, host, wgPub, realPub, shortID, ssKey)
	if err != nil {
		return err
	}

	fmt.Println("server de-dev-01 created:", host)
	fmt.Println("---- NODE SECRETS (store securely; needed to provision the real node) ----")
	fmt.Println("WireGuard server private key:", wgPriv)
	fmt.Println("Reality server private key:  ", realPriv)
	fmt.Println("Reality short id:            ", shortID)
	fmt.Println("-------------------------------------------------------------------------")
	return nil
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
