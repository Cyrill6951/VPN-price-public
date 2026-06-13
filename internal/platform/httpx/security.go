package httpx

import (
	"fmt"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

// SecurityHeaders sets conservative security headers on every response.
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "SAMEORIGIN")
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		h.Set("Cross-Origin-Opener-Policy", "same-origin")
		h.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		next.ServeHTTP(w, r)
	})
}

// RateLimiter implements a fixed-window per-IP rate limiter backed by Redis.
type RateLimiter struct {
	rdb *redis.Client
}

// NewRateLimiter builds a RateLimiter over the given Redis client.
func NewRateLimiter(rdb *redis.Client) *RateLimiter { return &RateLimiter{rdb: rdb} }

// Middleware returns a rate-limiting middleware for a scope, allowing `limit`
// requests per `window` per client IP. Fails open if Redis is unavailable.
func (rl *RateLimiter) Middleware(scope string, limit int, window time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := ClientIP(r)
			key := fmt.Sprintf("rl:%s:%s", scope, ip)

			count, err := rl.rdb.Incr(r.Context(), key).Result()
			if err != nil {
				next.ServeHTTP(w, r) // fail open
				return
			}
			if count == 1 {
				_ = rl.rdb.Expire(r.Context(), key, window).Err()
			}
			if count > int64(limit) {
				ttl, _ := rl.rdb.TTL(r.Context(), key).Result()
				if ttl <= 0 {
					ttl = window
				}
				w.Header().Set("Retry-After", strconv.Itoa(int(ttl.Seconds())))
				writeJSONError(w, http.StatusTooManyRequests, "rate limit exceeded")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// ClientIP extracts the best-effort client IP, honouring X-Forwarded-For.
func ClientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if i := indexByte(xff, ','); i >= 0 {
			return trimSpace(xff[:i])
		}
		return trimSpace(xff)
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func writeJSONError(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_, _ = w.Write([]byte(`{"error":"` + msg + `"}`))
}

func indexByte(s string, b byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == b {
			return i
		}
	}
	return -1
}

func trimSpace(s string) string {
	start, end := 0, len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t') {
		end--
	}
	return s[start:end]
}
