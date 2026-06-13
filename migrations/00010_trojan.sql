-- +goose NO TRANSACTION
-- +goose Up
-- +goose StatementBegin
ALTER TYPE vpn_protocol ADD VALUE IF NOT EXISTS 'trojan';
-- +goose StatementEnd

-- +goose StatementBegin
-- Trojan reuses the server's Reality keypair on a separate port.
ALTER TABLE servers ADD COLUMN IF NOT EXISTS trojan_port INT;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE servers DROP COLUMN IF EXISTS trojan_port;
-- enum value 'trojan' cannot be dropped.
-- +goose StatementEnd
