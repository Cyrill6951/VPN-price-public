package auth

import (
	"context"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// Service contains the authentication business logic.
type Service struct {
	repo     *Repository
	tokens   *TokenManager
	redis    *redis.Client
	botToken string
}

// NewService wires the auth service.
func NewService(repo *Repository, tokens *TokenManager, rdb *redis.Client, botToken string) *Service {
	return &Service{repo: repo, tokens: tokens, redis: rdb, botToken: botToken}
}

// LoginMeta carries request context recorded on the session.
type LoginMeta struct {
	UserAgent string
	IP        net.IP
}

// Register validates the password, creates the account and issues tokens.
func (s *Service) Register(ctx context.Context, email, password, language string, country *string, meta LoginMeta) (User, TokenPair, error) {
	if err := ValidatePassword(password); err != nil {
		return User{}, TokenPair{}, err
	}
	hash, err := HashPassword(password)
	if err != nil {
		return User{}, TokenPair{}, err
	}
	u, err := s.repo.CreateUserWithPassword(ctx, email, hash, language, country)
	if err != nil {
		return User{}, TokenPair{}, err
	}
	pair, err := s.issueTokens(ctx, u, meta)
	if err != nil {
		return User{}, TokenPair{}, err
	}
	return u, pair, nil
}

// Login verifies credentials and issues tokens.
func (s *Service) Login(ctx context.Context, email, password string, meta LoginMeta) (User, TokenPair, error) {
	u, hash, err := s.repo.GetAuthByEmail(ctx, email)
	if err != nil {
		return User{}, TokenPair{}, err
	}
	if hash == "" || !CheckPassword(hash, password) {
		return User{}, TokenPair{}, ErrInvalidCredentials
	}
	if u.Status == StatusBlocked {
		return User{}, TokenPair{}, ErrUserBlocked
	}
	pair, err := s.issueTokens(ctx, u, meta)
	if err != nil {
		return User{}, TokenPair{}, err
	}
	return u, pair, nil
}

// TelegramLogin verifies the Telegram signature and logs in or provisions a user.
func (s *Service) TelegramLogin(ctx context.Context, a TelegramAuth, meta LoginMeta) (User, TokenPair, error) {
	if err := VerifyTelegramAuth(s.botToken, a); err != nil {
		return User{}, TokenPair{}, err
	}
	u, err := s.repo.GetUserByTelegramID(ctx, a.ID)
	if errors.Is(err, ErrNotFound) {
		u, err = s.repo.CreateTelegramUser(ctx, a.ID, a.FirstName, a.LastName, a.Username)
	}
	if err != nil {
		return User{}, TokenPair{}, err
	}
	if u.Status == StatusBlocked {
		return User{}, TokenPair{}, ErrUserBlocked
	}
	pair, err := s.issueTokens(ctx, u, meta)
	if err != nil {
		return User{}, TokenPair{}, err
	}
	return u, pair, nil
}

// Refresh rotates a refresh token: the presented session is revoked and a new
// token pair is issued. Reusing a revoked token revokes the whole user chain.
func (s *Service) Refresh(ctx context.Context, refreshToken string, meta LoginMeta) (TokenPair, error) {
	hash := HashRefreshToken(refreshToken)
	sess, err := s.repo.GetSessionByHash(ctx, hash)
	if err != nil {
		return TokenPair{}, err
	}
	if sess.RevokedAt != nil {
		// Token reuse detected — revoke everything for safety.
		_ = s.repo.RevokeAllUserSessions(ctx, sess.UserID)
		return TokenPair{}, ErrSessionRevoked
	}
	if time.Now().After(sess.ExpiresAt) {
		return TokenPair{}, ErrSessionRevoked
	}

	if err := s.repo.RevokeSession(ctx, sess.ID); err != nil {
		return TokenPair{}, err
	}
	u, err := s.repo.GetUserByID(ctx, sess.UserID)
	if err != nil {
		return TokenPair{}, err
	}
	if u.Status == StatusBlocked {
		return TokenPair{}, ErrUserBlocked
	}
	return s.issueTokens(ctx, u, meta)
}

// Logout blacklists the current access token until its expiry and revokes the
// refresh session if one is supplied.
func (s *Service) Logout(ctx context.Context, jti string, accessExpiry time.Time, refreshToken string) error {
	ttl := time.Until(accessExpiry)
	if ttl > 0 {
		if err := s.redis.Set(ctx, blacklistKey(jti), "1", ttl).Err(); err != nil {
			return fmt.Errorf("blacklist token: %w", err)
		}
	}
	if refreshToken != "" {
		sess, err := s.repo.GetSessionByHash(ctx, HashRefreshToken(refreshToken))
		if err == nil {
			_ = s.repo.RevokeSession(ctx, sess.ID)
		}
	}
	return nil
}

// IsBlacklisted reports whether an access token jti was revoked via logout.
func (s *Service) IsBlacklisted(ctx context.Context, jti string) (bool, error) {
	n, err := s.redis.Exists(ctx, blacklistKey(jti)).Result()
	if err != nil {
		return false, fmt.Errorf("check blacklist: %w", err)
	}
	return n > 0, nil
}

// UserByID returns a user (used by the auth middleware / user module).
func (s *Service) UserByID(ctx context.Context, id uuid.UUID) (User, error) {
	return s.repo.GetUserByID(ctx, id)
}

// EnsureTelegramUser returns the user linked to a Telegram id, creating one on
// first contact. Used by the bot, where the Telegram id is inherently trusted.
func (s *Service) EnsureTelegramUser(ctx context.Context, tgID int64, firstName, lastName, username string) (User, error) {
	u, err := s.repo.GetUserByTelegramID(ctx, tgID)
	if errors.Is(err, ErrNotFound) {
		return s.repo.CreateTelegramUser(ctx, tgID, firstName, lastName, username)
	}
	return u, err
}

func (s *Service) issueTokens(ctx context.Context, u User, meta LoginMeta) (TokenPair, error) {
	access, _, expiresAt, err := s.tokens.IssueAccess(u.ID, u.Role)
	if err != nil {
		return TokenPair{}, err
	}
	refresh, refreshHash, err := s.tokens.NewRefreshToken()
	if err != nil {
		return TokenPair{}, err
	}
	refreshExp := time.Now().Add(s.tokens.RefreshTTL())
	if _, err := s.repo.CreateSession(ctx, u.ID, refreshHash, meta.UserAgent, meta.IP, refreshExp); err != nil {
		return TokenPair{}, err
	}
	return TokenPair{
		AccessToken:  access,
		RefreshToken: refresh,
		TokenType:    "Bearer",
		ExpiresAt:    expiresAt,
	}, nil
}

func blacklistKey(jti string) string { return "auth:bl:" + jti }
