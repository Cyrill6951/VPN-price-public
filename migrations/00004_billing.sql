-- +goose Up
-- +goose StatementBegin

CREATE TYPE order_status AS ENUM (
    'new', 'wait_payment', 'processing', 'paid', 'failed', 'expired', 'cancelled', 'refunded'
);
CREATE TYPE payment_status AS ENUM (
    'created', 'pending', 'success', 'failed', 'expired', 'refunded'
);
CREATE TYPE promo_type AS ENUM ('percent', 'fixed');
CREATE TYPE ledger_type AS ENUM ('charge', 'refund', 'bonus', 'adjustment');

CREATE TABLE promocodes (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    code       CITEXT UNIQUE NOT NULL,
    type       promo_type NOT NULL,
    discount   NUMERIC(10,2) NOT NULL,          -- percent (0-100) or fixed amount
    currency   TEXT,                            -- for fixed discounts
    max_uses   INT,                             -- NULL = unlimited
    used_count INT NOT NULL DEFAULT 0,
    expires_at TIMESTAMPTZ,
    active     BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE orders (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    plan_id         UUID NOT NULL REFERENCES plans (id),
    country_id      UUID NOT NULL REFERENCES countries (id),
    protocol        vpn_protocol NOT NULL,
    amount          NUMERIC(10,2) NOT NULL,
    discount        NUMERIC(10,2) NOT NULL DEFAULT 0,
    currency        TEXT NOT NULL DEFAULT 'USD',
    promo_id        UUID REFERENCES promocodes (id),
    gateway         TEXT NOT NULL,
    status          order_status NOT NULL DEFAULT 'new',
    subscription_id UUID REFERENCES subscriptions (id),
    label           TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    paid_at         TIMESTAMPTZ
);

CREATE INDEX idx_orders_user ON orders (user_id);
CREATE INDEX idx_orders_status ON orders (status);

CREATE TRIGGER trg_orders_updated_at
    BEFORE UPDATE ON orders
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE payments (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id    UUID NOT NULL REFERENCES orders (id) ON DELETE CASCADE,
    gateway     TEXT NOT NULL,
    external_id TEXT,                            -- provider payment id
    status      payment_status NOT NULL DEFAULT 'created',
    amount      NUMERIC(10,2) NOT NULL,
    currency    TEXT NOT NULL DEFAULT 'USD',
    fee         NUMERIC(10,2) NOT NULL DEFAULT 0,
    payment_url TEXT,
    payload     JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_payments_order ON payments (order_id);
CREATE UNIQUE INDEX idx_payments_gateway_external
    ON payments (gateway, external_id) WHERE external_id IS NOT NULL;

CREATE TRIGGER trg_payments_updated_at
    BEFORE UPDATE ON payments
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE promo_redemptions (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    promo_id   UUID NOT NULL REFERENCES promocodes (id),
    user_id    UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    order_id   UUID NOT NULL REFERENCES orders (id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (promo_id, user_id)
);

-- Immutable financial ledger with a hash chain for tamper-evidence.
CREATE TABLE ledger (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    UUID NOT NULL REFERENCES users (id),
    type       ledger_type NOT NULL,
    amount     NUMERIC(14,2) NOT NULL,
    currency   TEXT NOT NULL DEFAULT 'USD',
    ref_type   TEXT,
    ref_id     UUID,
    prev_hash  TEXT,
    hash       TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_ledger_user ON ledger (user_id);

-- Webhook idempotency / audit.
CREATE TABLE webhook_events (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    gateway      TEXT NOT NULL,
    external_id  TEXT NOT NULL,
    status       TEXT,
    payload      JSONB NOT NULL DEFAULT '{}'::jsonb,
    received_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    processed_at TIMESTAMPTZ,
    UNIQUE (gateway, external_id)
);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS webhook_events;
DROP TABLE IF EXISTS ledger;
DROP TABLE IF EXISTS promo_redemptions;
DROP TABLE IF EXISTS payments;
DROP TABLE IF EXISTS orders;
DROP TABLE IF EXISTS promocodes;
DROP TYPE IF EXISTS ledger_type;
DROP TYPE IF EXISTS promo_type;
DROP TYPE IF EXISTS payment_status;
DROP TYPE IF EXISTS order_status;
-- +goose StatementEnd
