-- +goose Up
-- +goose StatementBegin

ALTER TABLE servers
    ADD COLUMN IF NOT EXISTS health_score INT NOT NULL DEFAULT 100,
    ADD COLUMN IF NOT EXISTS last_seen    TIMESTAMPTZ;

-- Append-only health snapshots from the monitoring poller.
CREATE TABLE server_health (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    server_id  UUID NOT NULL REFERENCES servers (id) ON DELETE CASCADE,
    score      INT NOT NULL,
    status     TEXT NOT NULL,
    reason     TEXT,
    payload    JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_server_health_server ON server_health (server_id, created_at DESC);

-- Record of client migrations between servers.
CREATE TABLE migrations (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    subscription_id UUID NOT NULL REFERENCES subscriptions (id) ON DELETE CASCADE,
    user_id         UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    from_server     UUID REFERENCES servers (id),
    to_server       UUID REFERENCES servers (id),
    reason          TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_migrations_user ON migrations (user_id);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS migrations;
DROP TABLE IF EXISTS server_health;
ALTER TABLE servers DROP COLUMN IF EXISTS health_score, DROP COLUMN IF EXISTS last_seen;
-- +goose StatementEnd
