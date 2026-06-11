package auth_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/vpnsaas/platform/internal/auth"
	"github.com/vpnsaas/platform/internal/user"
)

// TestAuthFlow exercises the full register → me → refresh(rotation+reuse) →
// login → logout(blacklist) flow against a real Postgres and Redis.
//
// It is skipped unless TEST_DATABASE_URL is set, so `go test ./...` stays green
// in CI without infrastructure. Run it with the docker-compose stack up.
func TestAuthFlow(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping integration test")
	}
	redisURL := os.Getenv("TEST_REDIS_URL")
	if redisURL == "" {
		redisURL = "redis://localhost:6379/0"
	}

	ctx := context.Background()

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connect postgres: %v", err)
	}
	defer pool.Close()
	if _, err := pool.Exec(ctx, `TRUNCATE users CASCADE`); err != nil {
		t.Fatalf("truncate: %v", err)
	}

	ropts, err := redis.ParseURL(redisURL)
	if err != nil {
		t.Fatalf("parse redis url: %v", err)
	}
	rdb := redis.NewClient(ropts)
	defer func() { _ = rdb.Close() }()

	tokens := auth.NewTokenManager("test-secret", 15*time.Minute, time.Hour)
	authSvc := auth.NewService(auth.NewRepository(pool), tokens, rdb, "")
	authMW := auth.NewMiddleware(authSvc)

	mux := http.NewServeMux()
	auth.NewHandler(authSvc, authMW).RegisterRoutes(mux)
	user.NewHandler(user.NewRepository(pool), authMW).RegisterRoutes(mux)

	srv := httptest.NewServer(mux)
	defer srv.Close()
	c := &apiClient{t: t, base: srv.URL, client: srv.Client()}

	// 1. Register.
	body := map[string]any{"email": "alice@example.com", "password": "Str0ng!Passw0rd"}
	st, reg := c.do(http.MethodPost, "/api/v1/auth/register", "", body)
	if st != http.StatusCreated {
		t.Fatalf("register status = %d, want 201 (%v)", st, reg)
	}
	access1 := tokenField(t, reg, "access_token")
	refresh1 := tokenField(t, reg, "refresh_token")

	// 2. Duplicate registration is a conflict.
	if st, _ := c.do(http.MethodPost, "/api/v1/auth/register", "", body); st != http.StatusConflict {
		t.Fatalf("duplicate register status = %d, want 409", st)
	}

	// 3. Protected route with a valid token.
	if st, _ := c.do(http.MethodGet, "/api/v1/users/me", access1, nil); st != http.StatusOK {
		t.Fatalf("GET /me status = %d, want 200", st)
	}
	// 4. Protected route without a token.
	if st, _ := c.do(http.MethodGet, "/api/v1/users/me", "", nil); st != http.StatusUnauthorized {
		t.Fatalf("GET /me unauthenticated status = %d, want 401", st)
	}

	// 5. Refresh rotation.
	st, ref := c.do(http.MethodPost, "/api/v1/auth/refresh", "", map[string]any{"refresh_token": refresh1})
	if st != http.StatusOK {
		t.Fatalf("refresh status = %d, want 200 (%v)", st, ref)
	}
	refresh2 := nestedTokenField(t, ref, "refresh_token")

	// 6. Reusing the rotated (now revoked) refresh token is rejected.
	if st, _ := c.do(http.MethodPost, "/api/v1/auth/refresh", "", map[string]any{"refresh_token": refresh1}); st != http.StatusUnauthorized {
		t.Fatalf("reused refresh status = %d, want 401", st)
	}
	// 7. Reuse detection revoked the whole chain, so refresh2 also fails.
	if st, _ := c.do(http.MethodPost, "/api/v1/auth/refresh", "", map[string]any{"refresh_token": refresh2}); st != http.StatusUnauthorized {
		t.Fatalf("chain-revoked refresh status = %d, want 401", st)
	}

	// 8. Login issues a fresh pair.
	st, login := c.do(http.MethodPost, "/api/v1/auth/login", "", body)
	if st != http.StatusOK {
		t.Fatalf("login status = %d, want 200 (%v)", st, login)
	}
	access3 := tokenField(t, login, "access_token")
	refresh3 := tokenField(t, login, "refresh_token")

	// 9. Logout blacklists the access token.
	if st, _ := c.do(http.MethodPost, "/api/v1/auth/logout", access3, map[string]any{"refresh_token": refresh3}); st != http.StatusOK {
		t.Fatalf("logout status = %d, want 200", st)
	}
	// 10. The blacklisted access token no longer works.
	if st, _ := c.do(http.MethodGet, "/api/v1/users/me", access3, nil); st != http.StatusUnauthorized {
		t.Fatalf("GET /me after logout status = %d, want 401", st)
	}
}

type apiClient struct {
	t      *testing.T
	base   string
	client *http.Client
}

func (c *apiClient) do(method, path, token string, body any) (int, map[string]any) {
	c.t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			c.t.Fatalf("encode body: %v", err)
		}
	}
	req, err := http.NewRequest(method, c.base+path, &buf)
	if err != nil {
		c.t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := c.client.Do(req)
	if err != nil {
		c.t.Fatalf("do request: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	out := map[string]any{}
	_ = json.NewDecoder(resp.Body).Decode(&out)
	return resp.StatusCode, out
}

func tokenField(t *testing.T, resp map[string]any, field string) string {
	t.Helper()
	tk, ok := resp["tokens"].(map[string]any)
	if !ok {
		t.Fatalf("response has no tokens object: %v", resp)
	}
	v, _ := tk[field].(string)
	if v == "" {
		t.Fatalf("token field %q empty in %v", field, resp)
	}
	return v
}

// nestedTokenField reads tokens.<field> from the /refresh response shape.
func nestedTokenField(t *testing.T, resp map[string]any, field string) string {
	return tokenField(t, resp, field)
}
