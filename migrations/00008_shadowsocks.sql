-- +goose NO TRANSACTION
-- +goose Up
-- +goose StatementBegin
ALTER TYPE vpn_protocol ADD VALUE IF NOT EXISTS 'shadowsocks';
-- +goose StatementEnd

-- +goose StatementBegin
ALTER TABLE servers
    ADD COLUMN IF NOT EXISTS ss_port       INT,
    ADD COLUMN IF NOT EXISTS ss_method     TEXT,
    ADD COLUMN IF NOT EXISTS ss_server_key TEXT;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE servers
    DROP COLUMN IF EXISTS ss_port,
    DROP COLUMN IF EXISTS ss_method,
    DROP COLUMN IF EXISTS ss_server_key;
-- enum values cannot be dropped; 'shadowsocks' remains.
-- +goose StatementEnd
