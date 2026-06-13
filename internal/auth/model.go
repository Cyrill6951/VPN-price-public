// Package auth implements registration, login, JWT issuance/rotation,
// Telegram Login and the authorization (RBAC) middleware.
package auth

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

// Role is a user's authorization role. Ordering is not significant; checks are
// membership-based via RBAC middleware.
type Role string

const (
	RoleUser       Role = "user"
	RoleVIP        Role = "vip"
	RolePartner    Role = "partner"
	RoleReseller   Role = "reseller"
	RoleSupport    Role = "support"
	RoleModerator  Role = "moderator"
	RoleAdmin      Role = "admin"
	RoleSuperAdmin Role = "superadmin"
)

// Status is the account lifecycle state.
type Status string

const (
	StatusActive  Status = "active"
	StatusPending Status = "pending"
	StatusBlocked Status = "blocked"
	StatusDeleted Status = "deleted"
)

// User is the core account record.
type User struct {
	ID         uuid.UUID `json:"id"`
	Email      *string   `json:"email,omitempty"`
	Phone      *string   `json:"phone,omitempty"`
	TelegramID *int64    `json:"telegram_id,omitempty"`
	Role       Role      `json:"role"`
	Status     Status    `json:"status"`
	Language   string    `json:"language"`
	Timezone   string    `json:"timezone"`
	Currency   string    `json:"currency"`
	Country    *string   `json:"country,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// TokenPair is returned to clients on successful authentication.
type TokenPair struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	TokenType    string    `json:"token_type"`
	ExpiresAt    time.Time `json:"expires_at"`
}

// Domain errors. Handlers map these to HTTP status codes.
var (
	ErrEmailTaken         = errors.New("email already registered")
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrUserBlocked        = errors.New("account is blocked")
	ErrInvalidToken       = errors.New("invalid or expired token")
	ErrSessionRevoked     = errors.New("session revoked or expired")
	ErrTelegramSignature  = errors.New("invalid telegram signature")
	ErrNotFound           = errors.New("not found")
	Err2FARequired        = errors.New("two-factor code required")
	ErrInvalidTOTP        = errors.New("invalid two-factor code")
	ErrTOTPNotSetup       = errors.New("two-factor is not set up")
)
