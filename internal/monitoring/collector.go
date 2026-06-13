package monitoring

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
)

// Collector exposes platform business metrics to Prometheus by querying the
// database on each scrape (pull model).
type Collector struct {
	pool *pgxpool.Pool

	serversTotal  *prometheus.Desc
	serversActive *prometheus.Desc
	activeSubs    *prometheus.Desc
	paidOrders    *prometheus.Desc
	revenueTotal  *prometheus.Desc
	serverHealth  *prometheus.Desc
}

// NewCollector builds the DB-backed Prometheus collector.
func NewCollector(pool *pgxpool.Pool) *Collector {
	return &Collector{
		pool:          pool,
		serversTotal:  prometheus.NewDesc("vpn_servers_total", "Total servers", nil, nil),
		serversActive: prometheus.NewDesc("vpn_servers_active", "Active servers", nil, nil),
		activeSubs:    prometheus.NewDesc("vpn_active_subscriptions", "Active subscriptions", nil, nil),
		paidOrders:    prometheus.NewDesc("vpn_orders_paid_total", "Paid orders", nil, nil),
		revenueTotal:  prometheus.NewDesc("vpn_revenue_total", "Total revenue from paid orders", nil, nil),
		serverHealth:  prometheus.NewDesc("vpn_server_health_score", "Per-server health score (0-100)", []string{"hostname", "country"}, nil),
	}
}

// Describe implements prometheus.Collector.
func (c *Collector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.serversTotal
	ch <- c.serversActive
	ch <- c.activeSubs
	ch <- c.paidOrders
	ch <- c.revenueTotal
	ch <- c.serverHealth
}

// Collect implements prometheus.Collector.
func (c *Collector) Collect(ch chan<- prometheus.Metric) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	var total, active, subs, paid int
	var revenue float64
	err := c.pool.QueryRow(ctx, `
		SELECT
			(SELECT count(*) FROM servers),
			(SELECT count(*) FROM servers WHERE status='active'),
			(SELECT count(*) FROM subscriptions WHERE status='active'),
			(SELECT count(*) FROM orders WHERE status='paid'),
			(SELECT COALESCE(sum(amount),0) FROM orders WHERE status='paid')
	`).Scan(&total, &active, &subs, &paid, &revenue)
	if err == nil {
		ch <- prometheus.MustNewConstMetric(c.serversTotal, prometheus.GaugeValue, float64(total))
		ch <- prometheus.MustNewConstMetric(c.serversActive, prometheus.GaugeValue, float64(active))
		ch <- prometheus.MustNewConstMetric(c.activeSubs, prometheus.GaugeValue, float64(subs))
		ch <- prometheus.MustNewConstMetric(c.paidOrders, prometheus.GaugeValue, float64(paid))
		ch <- prometheus.MustNewConstMetric(c.revenueTotal, prometheus.GaugeValue, revenue)
	}

	rows, err := c.pool.Query(ctx, `
		SELECT s.hostname, co.name, s.health_score
		FROM servers s JOIN countries co ON co.id = s.country_id
		WHERE s.status <> 'archived'`)
	if err != nil {
		return
	}
	defer rows.Close()
	for rows.Next() {
		var host, country string
		var score int
		if err := rows.Scan(&host, &country, &score); err == nil {
			ch <- prometheus.MustNewConstMetric(c.serverHealth, prometheus.GaugeValue, float64(score), host, country)
		}
	}
}
