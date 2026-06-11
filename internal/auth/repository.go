package auth

import (
	"context"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository provides persistence for users and sessions.
type Repository struct {
	pool *pgxpool.Pool
}

// NewRepository builds a Repository over the given pgx pool.
func NewRepository(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

// Session is a stored refresh-token record.
type Session struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	ExpiresAt time.Time
	RevokedAt *time.Time
}

const userColumns = `id, email, phone, telegram_id, role, status,
	language, timezone, currency, country, created_at, updated_at`

func scanUser(row pgx.Row) (User, error) {
	var u User
	err := row.Scan(
		&u.ID, &u.Email, &u.Phone, &u.TelegramID, &u.Role, &u.Status,
		&u.Language, &u.Timezone, &u.Currency, &u.Country, &u.CreatedAt, &u.UpdatedAt,
	)
	return u, err
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

// CreateUserWithPassword inserts a user and an empty profile in one transaction.
func (r *Repository) CreateUserWithPassword(ctx context.Context, email, passwordHash, language string, country *string) (User, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return User{}, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	row := tx.QueryRow(ctx, `
		INSERT INTO users (email, password_hash, language, country)
		VALUES ($1, $2, COALESCE(NULLIF($3, ''), 'en'), $4)
		RETURNING `+userColumns, email, passwordHash, language, country)

	u, err := scanUser(row)
	if err != nil {
		if isUniqueViolation(err) {
			return User{}, ErrEmailTaken
		}
		return User{}, fmt.Errorf("insert user: %w", err)
	}

	if _, err := tx.Exec(ctx, `INSERT INTO user_profiles (user_id) VALUES ($1)`, u.ID); err != nil {
		return User{}, fmt.Errorf("insert profile: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return User{}, fmt.Errorf("commit: %w", err)
	}
	return u, nil
}

// GetAuthByEmail returns the user and its password hash for login.
func (r *Repository) GetAuthByEmail(ctx context.Context, email string) (User, string, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT `+userColumns+`, COALESCE(password_hash, '')
		FROM users
		WHERE email = $1 AND deleted_at IS NULL`, email)

	var u User
	var hash string
	err := row.Scan(
		&u.ID, &u.Email, &u.Phone, &u.TelegramID, &u.Role, &u.Status,
		&u.Language, &u.Timezone, &u.Currency, &u.Country, &u.CreatedAt, &u.UpdatedAt,
		&hash,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, "", ErrInvalidCredentials
	}
	if err != nil {
		return User{}, "", fmt.Errorf("get user by email: %w", err)
	}
	return u, hash, nil
}

// GetUserByID returns a user by id.
func (r *Repository) GetUserByID(ctx context.Context, id uuid.UUID) (User, error) {
	row := r.pool.QueryRow(ctx, `SELECT `+userColumns+`
		FROM users WHERE id = $1 AND deleted_at IS NULL`, id)
	u, err := scanUser(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrNotFound
	}
	if err != nil {
		return User{}, fmt.Errorf("get user by id: %w", err)
	}
	return u, nil
}

// GetUserByTelegramID returns a user by Telegram id, or ErrNotFound.
func (r *Repository) GetUserByTelegramID(ctx context.Context, tgID int64) (User, error) {
	row := r.pool.QueryRow(ctx, `SELECT `+userColumns+`
		FROM users WHERE telegram_id = $1 AND deleted_at IS NULL`, tgID)
	u, err := scanUser(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrNotFound
	}
	if err != nil {
		return User{}, fmt.Errorf("get user by telegram id: %w", err)
	}
	return u, nil
}

// CreateTelegramUser provisions a new account linked to a Telegram id.
func (r *Repository) CreateTelegramUser(ctx context.Context, tgID int64, firstName, lastName, username string) (User, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return User{}, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	row := tx.QueryRow(ctx, `
		INSERT INTO users (telegram_id) VALUES ($1)
		RETURNING `+userColumns, tgID)
	u, err := scanUser(row)
	if err != nil {
		return User{}, fmt.Errorf("insert telegram user: %w", err)
	}

	if _, err := tx.Exec(ctx,
		`INSERT INTO user_profiles (user_id, first_name, last_name) VALUES ($1, NULLIF($2,''), NULLIF($3,''))`,
		u.ID, firstName, lastName); err != nil {
		return User{}, fmt.Errorf("insert profile: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return User{}, fmt.Errorf("commit: %w", err)
	}
	return u, nil
}

// CreateSession stores a refresh-token session and returns its id.
func (r *Repository) CreateSession(ctx context.Context, userID uuid.UUID, tokenHash []byte, userAgent string, ip net.IP, expiresAt time.Time) (uuid.UUID, error) {
	var ipText *string
	if ip != nil {
		s := ip.String()
		ipText = &s
	}
	var id uuid.UUID
	err := r.pool.QueryRow(ctx, `
		INSERT INTO user_sessions (user_id, token_hash, user_agent, ip, expires_at)
		VALUES ($1, $2, NULLIF($3,''), $4, $5)
		RETURNING id`, userID, tokenHash, userAgent, ipText, expiresAt).Scan(&id)
	if err != nil {
		return uuid.Nil, fmt.Errorf("create session: %w", err)
	}
	return id, nil
}

// GetSessionByHash looks up an active-or-not session by its token hash.
func (r *Repository) GetSessionByHash(ctx context.Context, tokenHash []byte) (Session, error) {
	var s Session
	err := r.pool.QueryRow(ctx, `
		SELECT id, user_id, expires_at, revoked_at
		FROM user_sessions WHERE token_hash = $1`, tokenHash).
		Scan(&s.ID, &s.UserID, &s.ExpiresAt, &s.RevokedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Session{}, ErrSessionRevoked
	}
	if err != nil {
		return Session{}, fmt.Errorf("get session: %w", err)
	}
	return s, nil
}

// RevokeSession marks a single session revoked.
func (r *Repository) RevokeSession(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE user_sessions SET revoked_at = now() WHERE id = $1 AND revoked_at IS NULL`, id)
	if err != nil {
		return fmt.Errorf("revoke session: %w", err)
	}
	return nil
}

// RevokeAllUserSessions revokes every active session of a user (full logout).
func (r *Repository) RevokeAllUserSessions(ctx context.Context, userID uuid.UUID) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE user_sessions SET revoked_at = now() WHERE user_id = $1 AND revoked_at IS NULL`, userID)
	if err != nil {
		return fmt.Errorf("revoke user sessions: %w", err)
	}
	return nil
}
