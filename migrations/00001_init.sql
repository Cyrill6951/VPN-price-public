-- +goose Up
-- +goose StatementBegin

-- Extensions used across the platform.
CREATE EXTENSION IF NOT EXISTS pgcrypto;   -- gen_random_uuid()
CREATE EXTENSION IF NOT EXISTS citext;     -- case-insensitive email/login

-- Shared trigger to keep updated_at current on row updates.
CREATE OR REPLACE FUNCTION set_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Global key/value settings (feature flags, runtime config) — see docs 03/17 SETTINGS.
CREATE TABLE settings (
    key        TEXT PRIMARY KEY,
    value      JSONB NOT NULL DEFAULT '{}'::jsonb,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TRIGGER trg_settings_updated_at
    BEFORE UPDATE ON settings
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Seed a marker row so the table is non-empty and verifiable post-migration.
INSERT INTO settings (key, value)
VALUES ('schema.version', '{"milestone": "M0"}'::jsonb);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS settings;
DROP FUNCTION IF EXISTS set_updated_at();
-- Extensions are left in place: other schemas may depend on them.
-- +goose StatementEnd
