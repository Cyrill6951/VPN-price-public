-- +goose Up
-- +goose StatementBegin

CREATE TABLE audit_log (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    actor_id    UUID REFERENCES users (id),
    actor_email TEXT,
    action      TEXT NOT NULL,
    object_type TEXT,
    object_id   TEXT,
    payload     JSONB NOT NULL DEFAULT '{}'::jsonb,
    ip          INET,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_audit_log_created ON audit_log (created_at DESC);
CREATE INDEX idx_audit_log_actor ON audit_log (actor_id);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS audit_log;
-- +goose StatementEnd
