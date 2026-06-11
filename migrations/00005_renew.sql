-- +goose Up
-- +goose StatementBegin

-- A renewal order targets an existing subscription instead of provisioning a new one.
ALTER TABLE orders ADD COLUMN renew_subscription_id UUID REFERENCES subscriptions (id);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE orders DROP COLUMN IF EXISTS renew_subscription_id;
-- +goose StatementEnd
