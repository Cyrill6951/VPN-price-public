// Package observability wires Prometheus metrics for the HTTP layer.
package observability

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Metrics holds the registered Prometheus collectors.
type Metrics struct {
	reqTotal    *prometheus.CounterVec
	reqDuration *prometheus.HistogramVec
}

// NewMetrics registers and returns the default HTTP metrics.
func NewMetrics() *Metrics {
	return &Metrics{
		reqTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests by method, route and status.",
		}, []string{"method", "route", "status"}),
		reqDuration: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request latency in seconds by method and route.",
			Buckets: prometheus.DefBuckets,
		}, []string{"method", "route"}),
	}
}

// Observe records a single completed request.
func (m *Metrics) Observe(method, route string, status int, dur time.Duration) {
	m.reqTotal.WithLabelValues(method, route, strconv.Itoa(status)).Inc()
	m.reqDuration.WithLabelValues(method, route).Observe(dur.Seconds())
}

// Handler returns the Prometheus scrape endpoint.
func (m *Metrics) Handler() http.Handler { return promhttp.Handler() }
