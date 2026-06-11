-- +goose Up
-- +goose StatementBegin

-- Roles and account status (see docs 02 participants, 13 RBAC).
CREATE TYPE user_role AS ENUM (
    'user', 'vip', 'partner', 'reseller',
    'support', 'moderator', 'admin', 'superadmin'
);

CREATE TYPE user_status AS ENUM ('active', 'pending', 'blocked', 'deleted');

CREATE TABLE users (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email         CITEXT UNIQUE,
    phone         TEXT UNIQUE,
    telegram_id   BIGINT UNIQUE,
    password_hash TEXT,
    role          user_role   NOT NULL DEFAULT 'user',
    status        user_status NOT NULL DEFAULT 'active',
    language      TEXT NOT NULL DEFAULT 'en',
    timezone      TEXT NOT NULL DEFAULT 'UTC',
    currency      TEXT NOT NULL DEFAULT 'USD',
    country       TEXT,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at    TIMESTAMPTZ,
    -- An account must be reachable by at least one identity.
    CONSTRAINT users_identity_present
        CHECK (email IS NOT NULL OR phone IS NOT NULL OR telegram_id IS NOT NULL)
);

CREATE INDEX idx_users_status ON users (status) WHERE deleted_at IS NULL;

CREATE TRIGGER trg_users_updated_at
    BEFORE UPDATE ON users
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE user_profiles (
    user_id     UUID PRIMARY KEY REFERENCES users (id) ON DELETE CASCADE,
    first_name  TEXT,
    last_name   TEXT,
    avatar      TEXT,
    birth_date  DATE,
    city        TEXT,
    country     TEXT,
    settings    JSONB NOT NULL DEFAULT '{}'::jsonb,
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TRIGGER trg_user_profiles_updated_at
    BEFORE UPDATE ON user_profiles
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- One row per issued refresh token (rotation: old row is revoked on refresh).
CREATE TABLE user_sessions (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    token_hash  BYTEA NOT NULL,        -- sha256 of the opaque refresh token
    user_agent  TEXT,
    ip          INET,
    expires_at  TIMESTAMPTZ NOT NULL,
    revoked_at  TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX idx_user_sessions_token_hash ON user_sessions (token_hash);
CREATE INDEX idx_user_sessions_user ON user_sessions (user_id) WHERE revoked_at IS NULL;

CREATE TABLE devices (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    device_uuid  TEXT NOT NULL,
    name         TEXT,
    platform     TEXT,     -- windows | macos | linux | android | ios | router | tv
    os           TEXT,
    version      TEXT,
    last_online  TIMESTAMPTZ,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (user_id, device_uuid)
);

CREATE INDEX idx_devices_user ON devices (user_id);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS devices;
DROP TABLE IF EXISTS user_sessions;
DROP TABLE IF EXISTS user_profiles;
DROP TABLE IF EXISTS users;
DROP TYPE IF EXISTS user_status;
DROP TYPE IF EXISTS user_role;
-- +goose StatementEnd
