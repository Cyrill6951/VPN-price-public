package auth

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
)

type principalKey struct{}

// Principal is the authenticated identity attached to a request context.
type Principal struct {
	UserID    uuid.UUID
	Role      Role
	JTI       string
	ExpiresAt time.Time
}

// Middleware authenticates requests using a Bearer access token.
type Middleware struct {
	svc *Service
}

// NewMiddleware builds the auth middleware over the given service.
func NewMiddleware(svc *Service) *Middleware { return &Middleware{svc: svc} }

// Authenticate verifies the access token, checks the logout blacklist and
// injects the Principal. It rejects requests without a valid token.
func (m *Middleware) Authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw := bearerToken(r)
		if raw == "" {
			writeError(w, http.StatusUnauthorized, "missing bearer token")
			return
		}
		claims, err := m.svc.tokens.ParseAccess(raw)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "invalid token")
			return
		}
		blacklisted, err := m.svc.IsBlacklisted(r.Context(), claims.ID)
		if err != nil {
			writeError(w, http.StatusServiceUnavailable, "auth backend unavailable")
			return
		}
		if blacklisted {
			writeError(w, http.StatusUnauthorized, "token revoked")
			return
		}
		uid, err := uuid.Parse(claims.Subject)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "invalid subject")
			return
		}

		p := Principal{UserID: uid, Role: claims.Role, JTI: claims.ID}
		if claims.ExpiresAt != nil {
			p.ExpiresAt = claims.ExpiresAt.Time
		}
		ctx := context.WithValue(r.Context(), principalKey{}, p)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequireRole restricts a handler to principals holding one of the given roles.
// It must be applied after Authenticate.
func RequireRole(roles ...Role) func(http.Handler) http.Handler {
	allowed := make(map[Role]struct{}, len(roles))
	for _, r := range roles {
		allowed[r] = struct{}{}
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			p, ok := PrincipalFromContext(r.Context())
			if !ok {
				writeError(w, http.StatusUnauthorized, "unauthenticated")
				return
			}
			if _, ok := allowed[p.Role]; !ok {
				writeError(w, http.StatusForbidden, "insufficient role")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// PrincipalFromContext returns the authenticated principal, if present.
func PrincipalFromContext(ctx context.Context) (Principal, bool) {
	p, ok := ctx.Value(principalKey{}).(Principal)
	return p, ok
}

func bearerToken(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if h == "" {
		return ""
	}
	const prefix = "Bearer "
	if len(h) > len(prefix) && strings.EqualFold(h[:len(prefix)], prefix) {
		return strings.TrimSpace(h[len(prefix):])
	}
	return ""
}
