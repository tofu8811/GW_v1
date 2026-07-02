-- +goose Up
ALTER TABLE ip_blacklist
ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;

ALTER TABLE ip_blacklist
DROP CONSTRAINT IF EXISTS ip_blacklist_ip_or_cidr_unique;

CREATE UNIQUE INDEX IF NOT EXISTS ip_blacklist_ip_or_cidr_active_unique
ON ip_blacklist (ip_or_cidr)
WHERE deleted_at IS NULL;

-- +goose Down
DROP INDEX IF EXISTS ip_blacklist_ip_or_cidr_active_unique;

ALTER TABLE ip_blacklist
ADD CONSTRAINT ip_blacklist_ip_or_cidr_unique UNIQUE (ip_or_cidr);

ALTER TABLE ip_blacklist
DROP COLUMN IF EXISTS deleted_at;
