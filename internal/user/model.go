// Package user implements profile and device management for authenticated users.
package user

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// Profile is the extended, user-editable part of an account.
type Profile struct {
	FirstName *string         `json:"first_name"`
	LastName  *string         `json:"last_name"`
	Avatar    *string         `json:"avatar"`
	City      *string         `json:"city"`
	Country   *string         `json:"country"`
	Settings  json.RawMessage `json:"settings"`
}

// Me is the aggregate returned by GET /users/me.
type Me struct {
	ID        uuid.UUID `json:"id"`
	Email     *string   `json:"email,omitempty"`
	Phone     *string   `json:"phone,omitempty"`
	Role      string    `json:"role"`
	Status    string    `json:"status"`
	Language  string    `json:"language"`
	Timezone  string    `json:"timezone"`
	Currency  string    `json:"currency"`
	Profile   Profile   `json:"profile"`
	CreatedAt time.Time `json:"created_at"`
}

// Device is a client device linked to a user.
type Device struct {
	ID         uuid.UUID  `json:"id"`
	DeviceUUID string     `json:"device_uuid"`
	Name       *string    `json:"name"`
	Platform   *string    `json:"platform"`
	OS         *string    `json:"os"`
	Version    *string    `json:"version"`
	LastOnline *time.Time `json:"last_online"`
	CreatedAt  time.Time  `json:"created_at"`
}
