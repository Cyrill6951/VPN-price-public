package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// Claims is the payload of an access token.
type Claims struct {
	jwt.RegisteredClaims
	Role Role `json:"role"`
}

// TokenManager issues and verifies access tokens and generates refresh tokens.
type TokenManager struct {
	secret     []byte
	accessTTL  time.Duration
	refreshTTL time.Duration
}

// NewTokenManager builds a TokenManager with the given HMAC secret and TTLs.
func NewTokenManager(secret string, accessTTL, refreshTTL time.Duration) *TokenManager {
	return &TokenManager{
		secret:     []byte(secret),
		accessTTL:  accessTTL,
		refreshTTL: refreshTTL,
	}
}

// RefreshTTL exposes the refresh token lifetime for session bookkeeping.
func (tm *TokenManager) RefreshTTL() time.Duration { return tm.refreshTTL }

// IssueAccess signs a new access token for the user. It returns the token, its
// jti (for blacklisting on logout) and its expiry.
func (tm *TokenManager) IssueAccess(userID uuid.UUID, role Role) (token, jti string, expiresAt time.Time, err error) {
	now := time.Now()
	expiresAt = now.Add(tm.accessTTL)
	jti = uuid.NewString()

	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID.String(),
			ID:        jti,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			NotBefore: jwt.NewNumericDate(now),
		},
		Role: role,
	}

	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(tm.secret)
	if err != nil {
		return "", "", time.Time{}, fmt.Errorf("sign access token: %w", err)
	}
	return signed, jti, expiresAt, nil
}

// ParseAccess verifies the signature and standard claims of an access token.
func (tm *TokenManager) ParseAccess(token string) (*Claims, error) {
	claims := &Claims{}
	_, err := jwt.ParseWithClaims(token, claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return tm.secret, nil
	}, jwt.WithValidMethods([]string{"HS256"}))
	if err != nil {
		return nil, ErrInvalidToken
	}
	return claims, nil
}

// NewRefreshToken returns a fresh opaque refresh token and its sha256 hash.
// Only the hash is stored server-side.
func (tm *TokenManager) NewRefreshToken() (token string, hash []byte, err error) {
	b := make([]byte, 32)
	if _, err = rand.Read(b); err != nil {
		return "", nil, fmt.Errorf("generate refresh token: %w", err)
	}
	token = base64.RawURLEncoding.EncodeToString(b)
	return token, HashRefreshToken(token), nil
}

// HashRefreshToken returns the sha256 hash used to look up a stored session.
func HashRefreshToken(token string) []byte {
	sum := sha256.Sum256([]byte(token))
	return sum[:]
}
