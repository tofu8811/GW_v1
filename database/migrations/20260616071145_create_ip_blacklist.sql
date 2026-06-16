-- +goose Up
CREATE TABLE ip_blacklist (
    id          UUID PRIMARY KEY,
    ip_or_cidr  CIDR NOT NULL UNIQUE,
    reason      TEXT,
    created_by  UUID REFERENCES users(id) ON DELETE SET NULL,
    expires_at  TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_blacklist_cidr
ON ip_blacklist USING gist (ip_or_cidr inet_ops);

-- +goose Down
DROP INDEX IF EXISTS idx_blacklist_cidr;
DROP TABLE IF EXISTS ip_blacklist;
