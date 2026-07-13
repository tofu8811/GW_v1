-- +goose Up
CREATE TABLE IF NOT EXISTS api_scopes (
    id          UUID CONSTRAINT api_scopes_pkey PRIMARY KEY DEFAULT gen_random_uuid(),
    service_id  UUID NOT NULL CONSTRAINT api_scopes_service_id_fkey REFERENCES services(id) ON DELETE RESTRICT,
    code        VARCHAR(150) NOT NULL,
    resource    VARCHAR(100) NOT NULL,
    action      VARCHAR(50) NOT NULL CONSTRAINT api_scopes_action_check CHECK (action IN ('read', 'write')),
    description TEXT,
    is_active   BOOLEAN NOT NULL DEFAULT TRUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at  TIMESTAMPTZ
);

CREATE UNIQUE INDEX IF NOT EXISTS api_scopes_code_active_unique
ON api_scopes(code)
WHERE deleted_at IS NULL;

CREATE UNIQUE INDEX IF NOT EXISTS api_scopes_service_resource_action_active_unique
ON api_scopes(service_id, resource, action)
WHERE deleted_at IS NULL;

DROP TRIGGER IF EXISTS trg_api_scopes_updated ON api_scopes;
CREATE TRIGGER trg_api_scopes_updated
BEFORE UPDATE ON api_scopes
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

ALTER TABLE routes ADD COLUMN IF NOT EXISTS required_scope_id UUID;
-- +goose StatementBegin
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'routes_required_scope_id_fkey'
    ) THEN
        ALTER TABLE routes
            ADD CONSTRAINT routes_required_scope_id_fkey
            FOREIGN KEY (required_scope_id) REFERENCES api_scopes(id) ON DELETE RESTRICT;
    END IF;
END $$;
-- +goose StatementEnd

-- Legacy api_key_scopes referenced admin permissions. There is no safe service-aware
-- mapping to api_scopes, so demo seed data recreates the pivot explicitly.
DROP TABLE IF EXISTS api_key_scopes;
CREATE TABLE api_key_scopes (
    api_key_id UUID NOT NULL CONSTRAINT api_key_scopes_api_key_id_fkey REFERENCES api_keys(id) ON DELETE CASCADE,
    scope_id   UUID NOT NULL CONSTRAINT api_key_scopes_scope_id_fkey REFERENCES api_scopes(id) ON DELETE RESTRICT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT api_key_scopes_pkey PRIMARY KEY (api_key_id, scope_id)
);

-- +goose Down
DROP TABLE IF EXISTS api_key_scopes;
CREATE TABLE api_key_scopes (
    api_key_id    UUID NOT NULL CONSTRAINT api_key_scopes_api_key_id_fkey REFERENCES api_keys(id) ON DELETE CASCADE,
    permission_id UUID NOT NULL CONSTRAINT api_key_scopes_permission_id_fkey REFERENCES permissions(id) ON DELETE RESTRICT,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT api_key_scopes_pkey PRIMARY KEY (api_key_id, permission_id)
);

ALTER TABLE routes DROP CONSTRAINT IF EXISTS routes_required_scope_id_fkey;
ALTER TABLE routes DROP COLUMN IF EXISTS required_scope_id;
DROP TRIGGER IF EXISTS trg_api_scopes_updated ON api_scopes;
DROP INDEX IF EXISTS api_scopes_service_resource_action_active_unique;
DROP INDEX IF EXISTS api_scopes_code_active_unique;
DROP TABLE IF EXISTS api_scopes;
