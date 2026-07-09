-- +goose Up
CREATE TABLE IF NOT EXISTS cors_policies (
    id                UUID CONSTRAINT cors_policies_pkey PRIMARY KEY DEFAULT gen_random_uuid(),
    name              VARCHAR(100) NOT NULL,
    allowed_origins   TEXT[] NOT NULL DEFAULT '{}',
    allowed_methods   TEXT[] NOT NULL DEFAULT '{}',
    allowed_headers   TEXT[] NOT NULL DEFAULT '{}',
    exposed_headers   TEXT[] NOT NULL DEFAULT '{}',
    allow_credentials BOOLEAN NOT NULL DEFAULT FALSE,
    max_age           INTEGER NOT NULL DEFAULT 0,
    is_active         BOOLEAN NOT NULL DEFAULT TRUE,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at        TIMESTAMPTZ
);

CREATE UNIQUE INDEX IF NOT EXISTS cors_policies_name_active_unique
ON cors_policies(name)
WHERE deleted_at IS NULL;

DROP TRIGGER IF EXISTS trg_cors_policies_updated ON cors_policies;
CREATE TRIGGER trg_cors_policies_updated
BEFORE UPDATE ON cors_policies
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

ALTER TABLE routes ADD COLUMN IF NOT EXISTS cors_policy_id UUID;
ALTER TABLE aggregation_configs ADD COLUMN IF NOT EXISTS cors_policy_id UUID;

-- +goose StatementBegin
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'routes_cors_policy_id_fkey') THEN
        ALTER TABLE routes
            ADD CONSTRAINT routes_cors_policy_id_fkey
            FOREIGN KEY (cors_policy_id) REFERENCES cors_policies(id) ON DELETE SET NULL;
    END IF;

    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'aggregation_configs_cors_policy_id_fkey') THEN
        ALTER TABLE aggregation_configs
            ADD CONSTRAINT aggregation_configs_cors_policy_id_fkey
            FOREIGN KEY (cors_policy_id) REFERENCES cors_policies(id) ON DELETE SET NULL;
    END IF;
END $$;
-- +goose StatementEnd

-- Duplicate legacy cors_configs for one route are resolved by newest-wins.
WITH ranked_legacy AS (
    SELECT
        cc.*,
        row_number() OVER (
            PARTITION BY cc.route_id
            ORDER BY cc.updated_at DESC, cc.created_at DESC, cc.id DESC
        ) AS rn
    FROM cors_configs cc
    JOIN routes r ON r.id = cc.route_id
    WHERE cc.deleted_at IS NULL
      AND cc.is_active = TRUE
      AND r.deleted_at IS NULL
), migrated AS (
    INSERT INTO cors_policies (
        name,
        allowed_origins,
        allowed_methods,
        allowed_headers,
        exposed_headers,
        allow_credentials,
        max_age,
        is_active,
        created_at,
        updated_at
    )
    SELECT
        'Migrated CORS for route ' || route_id::text,
        allowed_origins,
        allowed_methods,
        allowed_headers,
        '{}'::text[],
        allow_credentials,
        max_age,
        is_active,
        created_at,
        updated_at
    FROM ranked_legacy
    WHERE rn = 1
    ON CONFLICT (name) WHERE deleted_at IS NULL DO UPDATE SET
        allowed_origins = EXCLUDED.allowed_origins,
        allowed_methods = EXCLUDED.allowed_methods,
        allowed_headers = EXCLUDED.allowed_headers,
        exposed_headers = EXCLUDED.exposed_headers,
        allow_credentials = EXCLUDED.allow_credentials,
        max_age = EXCLUDED.max_age,
        is_active = EXCLUDED.is_active,
        updated_at = EXCLUDED.updated_at
    RETURNING id, name
)
UPDATE routes r
SET cors_policy_id = cp.id
FROM cors_policies cp
WHERE cp.name = 'Migrated CORS for route ' || r.id::text
  AND r.cors_policy_id IS NULL;

INSERT INTO cors_policies (
    id, name, allowed_origins, allowed_methods, allowed_headers,
    exposed_headers, allow_credentials, max_age, is_active
)
VALUES
    (
        'd0000000-0000-0000-0000-000000000101',
        'Public Web CORS',
        ARRAY['http://localhost:5173'],
        ARRAY['GET','POST','PUT','DELETE','OPTIONS'],
        ARRAY['Authorization','Content-Type','X-API-Key'],
        '{}'::text[],
        FALSE,
        3600,
        TRUE
    ),
    (
        'd0000000-0000-0000-0000-000000000102',
        'Admin Web CORS',
        ARRAY['http://localhost:5173'],
        ARRAY['GET','POST','PUT','PATCH','DELETE','OPTIONS'],
        ARRAY['Authorization','Content-Type'],
        '{}'::text[],
        TRUE,
        3600,
        TRUE
    )
ON CONFLICT (id) DO UPDATE SET
    name = EXCLUDED.name,
    allowed_origins = EXCLUDED.allowed_origins,
    allowed_methods = EXCLUDED.allowed_methods,
    allowed_headers = EXCLUDED.allowed_headers,
    exposed_headers = EXCLUDED.exposed_headers,
    allow_credentials = EXCLUDED.allow_credentials,
    max_age = EXCLUDED.max_age,
    is_active = EXCLUDED.is_active,
    updated_at = now(),
    deleted_at = NULL;

-- +goose Down
ALTER TABLE aggregation_configs DROP CONSTRAINT IF EXISTS aggregation_configs_cors_policy_id_fkey;
ALTER TABLE routes DROP CONSTRAINT IF EXISTS routes_cors_policy_id_fkey;
ALTER TABLE aggregation_configs DROP COLUMN IF EXISTS cors_policy_id;
ALTER TABLE routes DROP COLUMN IF EXISTS cors_policy_id;
DROP TRIGGER IF EXISTS trg_cors_policies_updated ON cors_policies;
DROP INDEX IF EXISTS cors_policies_name_active_unique;
DROP TABLE IF EXISTS cors_policies;