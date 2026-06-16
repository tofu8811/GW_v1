-- +goose Up
CREATE TABLE api_keys (
    id            UUID PRIMARY KEY,
    key_hash      VARCHAR(255) NOT NULL UNIQUE,
    key_prefix    VARCHAR(12)  NOT NULL,
    label         VARCHAR(100),
    user_id       UUID REFERENCES users(id) ON DELETE CASCADE,
    scopes        TEXT[] NOT NULL DEFAULT '{}',
    rate_limit_id UUID REFERENCES rate_limit_policies(id) ON DELETE SET NULL,
    expires_at    TIMESTAMPTZ,
    is_active     BOOLEAN NOT NULL DEFAULT TRUE,
    last_used_at  TIMESTAMPTZ,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_apikeys_hash
ON api_keys(key_hash)
WHERE is_active;

CREATE INDEX idx_apikeys_prefix
ON api_keys(key_prefix);

CREATE TRIGGER trg_api_keys_updated
BEFORE UPDATE ON api_keys
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

-- +goose Down
DROP TRIGGER IF EXISTS trg_api_keys_updated ON api_keys;
DROP INDEX IF EXISTS idx_apikeys_prefix;
DROP INDEX IF EXISTS idx_apikeys_hash;
DROP TABLE IF EXISTS api_keys;
