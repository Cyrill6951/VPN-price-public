-- +goose Up
-- +goose StatementBegin

-- Catalogue ---------------------------------------------------------------

CREATE TABLE countries (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    iso        TEXT UNIQUE NOT NULL,          -- ISO 3166-1 alpha-2
    name       TEXT NOT NULL,
    flag       TEXT,
    enabled    BOOLEAN NOT NULL DEFAULT true,
    priority   INT NOT NULL DEFAULT 100,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE plans (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name          TEXT NOT NULL,
    price         NUMERIC(10,2) NOT NULL DEFAULT 0,
    currency      TEXT NOT NULL DEFAULT 'USD',
    days          INT NOT NULL,
    max_devices   INT NOT NULL DEFAULT 1,
    traffic_limit BIGINT,                     -- bytes; NULL = unlimited
    speed_limit   INT,                        -- mbit/s; NULL = unlimited
    active        BOOLEAN NOT NULL DEFAULT true,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Infrastructure ----------------------------------------------------------

CREATE TYPE server_status AS ENUM ('provisioning', 'active', 'drain', 'offline', 'archived');

CREATE TABLE servers (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    country_id    UUID NOT NULL REFERENCES countries (id),
    provider      TEXT,
    hostname      TEXT NOT NULL,
    public_host   TEXT NOT NULL,              -- client-facing host/IP
    ipv4          INET,
    ipv6          INET,
    ssh_port      INT NOT NULL DEFAULT 22,
    agent_url     TEXT,                        -- node agent base URL (provisioner)
    status        server_status NOT NULL DEFAULT 'provisioning',
    reserve       BOOLEAN NOT NULL DEFAULT false,
    priority      INT NOT NULL DEFAULT 100,
    capacity      INT NOT NULL DEFAULT 1000,
    client_count  INT NOT NULL DEFAULT 0,

    -- WireGuard server parameters
    wg_port       INT,
    wg_public_key TEXT,
    wg_subnet     CIDR,                        -- e.g. 10.7.0.0/24
    wg_dns        TEXT DEFAULT '1.1.1.1',

    -- VLESS + Reality server parameters
    reality_port       INT,
    reality_public_key TEXT,
    reality_sni        TEXT,                   -- spoofed SNI / serverName
    reality_short_id   TEXT,
    reality_dest       TEXT,                   -- camouflage destination

    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_servers_country ON servers (country_id);
CREATE INDEX idx_servers_status ON servers (status);

CREATE TRIGGER trg_servers_updated_at
    BEFORE UPDATE ON servers
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Subscriptions & configs -------------------------------------------------

CREATE TYPE vpn_protocol AS ENUM ('wireguard', 'vless_reality');
CREATE TYPE subscription_status AS ENUM ('active', 'paused', 'blocked', 'expired', 'migrated', 'deleted');

CREATE TABLE subscriptions (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    plan_id     UUID REFERENCES plans (id),
    server_id   UUID NOT NULL REFERENCES servers (id),
    country_id  UUID NOT NULL REFERENCES countries (id),
    protocol    vpn_protocol NOT NULL,
    status      subscription_status NOT NULL DEFAULT 'active',
    label       TEXT,
    max_devices INT NOT NULL DEFAULT 1,
    expires_at  TIMESTAMPTZ NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_subscriptions_user ON subscriptions (user_id) WHERE status <> 'deleted';
CREATE INDEX idx_subscriptions_server ON subscriptions (server_id);

CREATE TRIGGER trg_subscriptions_updated_at
    BEFORE UPDATE ON subscriptions
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Rendered, versioned client configuration (payload lives in object storage).
CREATE TABLE vpn_configs (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    subscription_id   UUID NOT NULL REFERENCES subscriptions (id) ON DELETE CASCADE,
    protocol          vpn_protocol NOT NULL,
    uri               TEXT,                    -- vless:// or wg quick-import URI
    config_object_key TEXT,                    -- MinIO key of the encrypted config
    qr_object_key     TEXT,                    -- MinIO key of the QR PNG
    hash              TEXT,                    -- sha256 of plaintext config
    version           INT NOT NULL DEFAULT 1,
    encrypted         BOOLEAN NOT NULL DEFAULT true,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_vpn_configs_subscription ON vpn_configs (subscription_id);

-- Cryptographic material per config (secrets encrypted at rest).
CREATE TABLE vpn_keys (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    vpn_config_id  UUID NOT NULL REFERENCES vpn_configs (id) ON DELETE CASCADE,
    server_id      UUID NOT NULL REFERENCES servers (id),
    private_key    TEXT,                       -- encrypted (WireGuard client priv)
    public_key     TEXT,                       -- WireGuard client public
    psk            TEXT,                       -- encrypted preshared key (optional)
    client_uuid    TEXT,                       -- VLESS client UUID
    assigned_ip    INET,                       -- WireGuard client address
    status         TEXT NOT NULL DEFAULT 'active',
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_vpn_keys_config ON vpn_keys (vpn_config_id);
CREATE INDEX idx_vpn_keys_server ON vpn_keys (server_id);
-- One WireGuard address per server (subnets repeat across servers).
CREATE UNIQUE INDEX idx_vpn_keys_server_ip
    ON vpn_keys (server_id, assigned_ip)
    WHERE assigned_ip IS NOT NULL AND status = 'active';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS vpn_keys;
DROP TABLE IF EXISTS vpn_configs;
DROP TABLE IF EXISTS subscriptions;
DROP TYPE IF EXISTS subscription_status;
DROP TYPE IF EXISTS vpn_protocol;
DROP TABLE IF EXISTS servers;
DROP TYPE IF EXISTS server_status;
DROP TABLE IF EXISTS plans;
DROP TABLE IF EXISTS countries;
-- +goose StatementEnd
