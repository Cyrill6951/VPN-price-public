package user

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrNotFound is returned when a requested record does not exist.
var ErrNotFound = errors.New("not found")

// Repository persists profile and device data.
type Repository struct {
	pool *pgxpool.Pool
}

// NewRepository builds a Repository over the given pgx pool.
func NewRepository(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

// GetMe returns the account with its profile.
func (r *Repository) GetMe(ctx context.Context, userID uuid.UUID) (Me, error) {
	var m Me
	err := r.pool.QueryRow(ctx, `
		SELECT u.id, u.email, u.phone, u.role, u.status, u.language, u.timezone,
		       u.currency, u.created_at,
		       p.first_name, p.last_name, p.avatar, p.city, p.country,
		       COALESCE(p.settings, '{}'::jsonb)
		FROM users u
		LEFT JOIN user_profiles p ON p.user_id = u.id
		WHERE u.id = $1 AND u.deleted_at IS NULL`, userID).
		Scan(&m.ID, &m.Email, &m.Phone, &m.Role, &m.Status, &m.Language, &m.Timezone,
			&m.Currency, &m.CreatedAt,
			&m.Profile.FirstName, &m.Profile.LastName, &m.Profile.Avatar,
			&m.Profile.City, &m.Profile.Country, &m.Profile.Settings)
	if errors.Is(err, pgx.ErrNoRows) {
		return Me{}, ErrNotFound
	}
	if err != nil {
		return Me{}, fmt.Errorf("get me: %w", err)
	}
	return m, nil
}

// ProfileUpdate carries the editable profile fields. Nil pointers are left
// unchanged; non-nil pointers (including empty strings) overwrite.
type ProfileUpdate struct {
	FirstName *string
	LastName  *string
	City      *string
	Country   *string
	Avatar    *string
	Language  *string
	Timezone  *string
}

// UpdateProfile applies a partial update to the profile and core user fields.
func (r *Repository) UpdateProfile(ctx context.Context, userID uuid.UUID, in ProfileUpdate) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	_, err = tx.Exec(ctx, `
		UPDATE user_profiles SET
			first_name = COALESCE($2, first_name),
			last_name  = COALESCE($3, last_name),
			city       = COALESCE($4, city),
			country    = COALESCE($5, country),
			avatar     = COALESCE($6, avatar)
		WHERE user_id = $1`,
		userID, in.FirstName, in.LastName, in.City, in.Country, in.Avatar)
	if err != nil {
		return fmt.Errorf("update profile: %w", err)
	}

	if in.Language != nil || in.Timezone != nil {
		if _, err = tx.Exec(ctx, `
			UPDATE users SET
				language = COALESCE($2, language),
				timezone = COALESCE($3, timezone)
			WHERE id = $1`, userID, in.Language, in.Timezone); err != nil {
			return fmt.Errorf("update user: %w", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}

// ListDevices returns a user's registered devices, newest first.
func (r *Repository) ListDevices(ctx context.Context, userID uuid.UUID) ([]Device, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, device_uuid, name, platform, os, version, last_online, created_at
		FROM devices WHERE user_id = $1 ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, fmt.Errorf("list devices: %w", err)
	}
	defer rows.Close()

	devices := []Device{}
	for rows.Next() {
		var d Device
		if err := rows.Scan(&d.ID, &d.DeviceUUID, &d.Name, &d.Platform,
			&d.OS, &d.Version, &d.LastOnline, &d.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan device: %w", err)
		}
		devices = append(devices, d)
	}
	return devices, rows.Err()
}

// DeviceUpsert carries fields for registering or refreshing a device.
type DeviceUpsert struct {
	DeviceUUID string
	Name       *string
	Platform   *string
	OS         *string
	Version    *string
}

// UpsertDevice inserts a device or updates it on (user_id, device_uuid) conflict.
func (r *Repository) UpsertDevice(ctx context.Context, userID uuid.UUID, in DeviceUpsert) (Device, error) {
	var d Device
	err := r.pool.QueryRow(ctx, `
		INSERT INTO devices (user_id, device_uuid, name, platform, os, version, last_online)
		VALUES ($1, $2, $3, $4, $5, $6, now())
		ON CONFLICT (user_id, device_uuid) DO UPDATE SET
			name = COALESCE(EXCLUDED.name, devices.name),
			platform = COALESCE(EXCLUDED.platform, devices.platform),
			os = COALESCE(EXCLUDED.os, devices.os),
			version = COALESCE(EXCLUDED.version, devices.version),
			last_online = now()
		RETURNING id, device_uuid, name, platform, os, version, last_online, created_at`,
		userID, in.DeviceUUID, in.Name, in.Platform, in.OS, in.Version).
		Scan(&d.ID, &d.DeviceUUID, &d.Name, &d.Platform, &d.OS, &d.Version, &d.LastOnline, &d.CreatedAt)
	if err != nil {
		return Device{}, fmt.Errorf("upsert device: %w", err)
	}
	return d, nil
}

// DeleteDevice removes a device that belongs to the user.
func (r *Repository) DeleteDevice(ctx context.Context, userID, deviceID uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM devices WHERE id = $1 AND user_id = $2`, deviceID, userID)
	if err != nil {
		return fmt.Errorf("delete device: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
