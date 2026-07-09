-- +goose Up
ALTER TABLE aggregation_configs ADD COLUMN IF NOT EXISTS auth_required BOOLEAN NOT NULL DEFAULT TRUE;
ALTER TABLE aggregation_configs ADD COLUMN IF NOT EXISTS required_scope_id UUID;
ALTER TABLE aggregation_configs ADD COLUMN IF NOT EXISTS rate_limit_id UUID;

-- +goose StatementBegin
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'aggregation_configs_required_scope_id_fkey') THEN
        ALTER TABLE aggregation_configs
            ADD CONSTRAINT aggregation_configs_required_scope_id_fkey
            FOREIGN KEY (required_scope_id) REFERENCES api_scopes(id) ON DELETE RESTRICT;
    END IF;

    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'aggregation_configs_rate_limit_id_fkey') THEN
        ALTER TABLE aggregation_configs
            ADD CONSTRAINT aggregation_configs_rate_limit_id_fkey
            FOREIGN KEY (rate_limit_id) REFERENCES rate_limit_policies(id) ON DELETE SET NULL;
    END IF;
END $$;
-- +goose StatementEnd

-- Existing aggregations become protected by default. Admins must explicitly set auth_required=false for intentionally public aggregations.
UPDATE aggregation_configs
SET auth_required = TRUE
WHERE auth_required IS DISTINCT FROM TRUE;

-- +goose Down
ALTER TABLE aggregation_configs DROP CONSTRAINT IF EXISTS aggregation_configs_rate_limit_id_fkey;
ALTER TABLE aggregation_configs DROP CONSTRAINT IF EXISTS aggregation_configs_required_scope_id_fkey;
ALTER TABLE aggregation_configs DROP COLUMN IF EXISTS rate_limit_id;
ALTER TABLE aggregation_configs DROP COLUMN IF EXISTS required_scope_id;
ALTER TABLE aggregation_configs DROP COLUMN IF EXISTS auth_required;