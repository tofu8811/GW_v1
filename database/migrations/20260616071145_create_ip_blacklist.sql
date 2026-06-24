-- +goose Up
CREATE TABLE ip_blacklist (
    id          UUID CONSTRAINT ip_blacklist_pkey PRIMARY KEY,
    ip_or_cidr  CIDR NOT NULL CONSTRAINT ip_blacklist_ip_or_cidr_unique UNIQUE,
    reason      TEXT,
    created_by  UUID CONSTRAINT ip_blacklist_created_by_fkey REFERENCES users(id) ON DELETE SET NULL,
    expires_at  TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_blacklist_cidr
ON ip_blacklist USING gist (ip_or_cidr inet_ops);

-- +goose Down
DROP INDEX IF EXISTS idx_blacklist_cidr;
DROP TABLE IF EXISTS ip_blacklist;
