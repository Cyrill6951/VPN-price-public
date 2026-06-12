-- +goose Up
-- +goose StatementBegin
-- Routing mode chosen at purchase time: full | split_ru (RU resources direct).
ALTER TABLE orders ADD COLUMN routing TEXT NOT NULL DEFAULT 'full';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE orders DROP COLUMN IF EXISTS routing;
-- +goose StatementEnd
