-- +goose Up
ALTER TABLE services ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;
ALTER TABLE service_instances ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;
ALTER TABLE routes ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;
ALTER TABLE rate_limit_policies ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;
ALTER TABLE cors_configs ADD COLUMN IF NOT EXISTS is_active BOOLEAN NOT NULL DEFAULT TRUE;
ALTER TABLE cors_configs ADD COLUMN IF NOT EXISTS created_at TIMESTAMPTZ NOT NULL DEFAULT now();
ALTER TABLE cors_configs ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT now();
ALTER TABLE cors_configs ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;
ALTER TABLE gateway_plugins ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;
ALTER TABLE route_plugins ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;
ALTER TABLE users ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;
ALTER TABLE api_keys ADD COLUMN IF NOT EXISTS revoked_at TIMESTAMPTZ;
ALTER TABLE api_keys ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;
ALTER TABLE aggregation_configs ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT now();
ALTER TABLE aggregation_configs ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;
ALTER TABLE aggregation_steps ADD COLUMN IF NOT EXISTS is_active BOOLEAN NOT NULL DEFAULT TRUE;
ALTER TABLE aggregation_steps ADD COLUMN IF NOT EXISTS created_at TIMESTAMPTZ NOT NULL DEFAULT now();
ALTER TABLE aggregation_steps ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT now();
ALTER TABLE aggregation_steps ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;
ALTER TABLE roles ADD COLUMN IF NOT EXISTS is_active BOOLEAN NOT NULL DEFAULT TRUE;
ALTER TABLE roles ADD COLUMN IF NOT EXISTS created_at TIMESTAMPTZ NOT NULL DEFAULT now();
ALTER TABLE roles ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT now();
ALTER TABLE roles ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;
ALTER TABLE permissions ADD COLUMN IF NOT EXISTS is_active BOOLEAN NOT NULL DEFAULT TRUE;
ALTER TABLE permissions ADD COLUMN IF NOT EXISTS created_at TIMESTAMPTZ NOT NULL DEFAULT now();
ALTER TABLE permissions ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT now();
ALTER TABLE permissions ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;
ALTER TABLE role_permissions ADD COLUMN IF NOT EXISTS created_at TIMESTAMPTZ NOT NULL DEFAULT now();

ALTER TABLE services DROP CONSTRAINT IF EXISTS services_name_unique;
ALTER TABLE routes DROP CONSTRAINT IF EXISTS routes_path_method_unique;
ALTER TABLE gateway_plugins DROP CONSTRAINT IF EXISTS gateway_plugins_code_unique;
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_username_unique;
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_email_unique;
ALTER TABLE api_keys DROP CONSTRAINT IF EXISTS api_keys_key_hash_unique;
ALTER TABLE rate_limit_policies DROP CONSTRAINT IF EXISTS rate_limit_policies_name_unique;
ALTER TABLE aggregation_configs DROP CONSTRAINT IF EXISTS aggregation_configs_name_unique;
ALTER TABLE aggregation_configs DROP CONSTRAINT IF EXISTS aggregation_configs_path_method_unique;
ALTER TABLE aggregation_steps DROP CONSTRAINT IF EXISTS aggregation_steps_aggregation_sequence_unique;
ALTER TABLE service_instances DROP CONSTRAINT IF EXISTS service_instances_service_host_port_unique;
ALTER TABLE route_plugins DROP CONSTRAINT IF EXISTS route_plugins_route_plugin_unique;
ALTER TABLE cors_configs DROP CONSTRAINT IF EXISTS cors_configs_route_id_unique;
ALTER TABLE permissions DROP CONSTRAINT IF EXISTS permissions_resource_action_unique;
ALTER TABLE roles DROP CONSTRAINT IF EXISTS roles_name_unique;

CREATE UNIQUE INDEX IF NOT EXISTS services_name_active_unique ON services(name) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX IF NOT EXISTS routes_path_method_active_unique ON routes(path, method) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX IF NOT EXISTS gateway_plugins_code_active_unique ON gateway_plugins(code) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX IF NOT EXISTS users_username_active_unique ON users(username) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX IF NOT EXISTS users_email_active_unique ON users(email) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX IF NOT EXISTS api_keys_key_hash_active_unique ON api_keys(key_hash) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX IF NOT EXISTS rate_limit_policies_name_active_unique ON rate_limit_policies(name) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX IF NOT EXISTS aggregation_configs_name_active_unique ON aggregation_configs(name) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX IF NOT EXISTS aggregation_configs_path_method_active_unique ON aggregation_configs(path, method) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX IF NOT EXISTS aggregation_steps_aggregation_sequence_active_unique ON aggregation_steps(aggregation_id, sequence) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX IF NOT EXISTS service_instances_service_host_port_active_unique ON service_instances(service_id, host, port) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX IF NOT EXISTS route_plugins_route_plugin_active_unique ON route_plugins(route_id, plugin_id) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX IF NOT EXISTS cors_configs_route_id_active_unique ON cors_configs(route_id) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX IF NOT EXISTS permissions_resource_action_active_unique ON permissions(resource, action) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX IF NOT EXISTS roles_name_active_unique ON roles(name) WHERE deleted_at IS NULL;

CREATE TRIGGER trg_rate_limit_policies_updated
BEFORE UPDATE ON rate_limit_policies
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER trg_ip_blacklist_updated
BEFORE UPDATE ON ip_blacklist
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER trg_aggregation_configs_updated
BEFORE UPDATE ON aggregation_configs
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER trg_cors_configs_updated
BEFORE UPDATE ON cors_configs
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER trg_aggregation_steps_updated
BEFORE UPDATE ON aggregation_steps
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER trg_roles_updated
BEFORE UPDATE ON roles
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER trg_permissions_updated
BEFORE UPDATE ON permissions
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

CREATE TABLE IF NOT EXISTS clients (
    id            UUID CONSTRAINT clients_pkey PRIMARY KEY DEFAULT gen_random_uuid(),
    name          VARCHAR(100) NOT NULL,
    client_type   VARCHAR(20) NOT NULL CONSTRAINT clients_client_type_check CHECK (client_type IN ('internal_app', 'partner', 'service')),
    owner_user_id UUID CONSTRAINT clients_owner_user_id_fkey REFERENCES users(id) ON DELETE SET NULL,
    is_active     BOOLEAN NOT NULL DEFAULT TRUE,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at    TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_clients_owner_user ON clients(owner_user_id) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX IF NOT EXISTS clients_name_active_unique ON clients(name) WHERE deleted_at IS NULL;

CREATE TRIGGER trg_clients_updated
BEFORE UPDATE ON clients
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

ALTER TABLE api_keys ADD COLUMN IF NOT EXISTS client_id UUID;
ALTER TABLE api_keys ADD COLUMN IF NOT EXISTS created_by UUID;

CREATE TEMP TABLE api_key_client_map AS
SELECT user_id, gen_random_uuid() AS client_id
FROM (
    SELECT DISTINCT user_id
    FROM api_keys
) legacy_api_key_users;

INSERT INTO clients (id, name, client_type, owner_user_id)
SELECT
    client_id,
    CASE
        WHEN user_id IS NULL THEN 'legacy-client-unowned'
        ELSE 'legacy-user-' || user_id::text
    END,
    'internal_app',
    user_id
FROM api_key_client_map;

UPDATE api_keys ak
SET client_id = m.client_id
FROM api_key_client_map m
WHERE ak.user_id IS NOT DISTINCT FROM m.user_id;

INSERT INTO permissions (id, resource, action)
SELECT gen_random_uuid(), split_part(scope_value, ':', 2), split_part(scope_value, ':', 1)
FROM (
    SELECT DISTINCT unnest(scopes) AS scope_value
    FROM api_keys
    WHERE scopes IS NOT NULL
) s
WHERE position(':' IN scope_value) > 0
ON CONFLICT DO NOTHING;

CREATE TABLE IF NOT EXISTS api_key_scopes (
    api_key_id    UUID NOT NULL CONSTRAINT api_key_scopes_api_key_id_fkey REFERENCES api_keys(id) ON DELETE CASCADE,
    permission_id UUID NOT NULL CONSTRAINT api_key_scopes_permission_id_fkey REFERENCES permissions(id) ON DELETE RESTRICT,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT api_key_scopes_pkey PRIMARY KEY (api_key_id, permission_id)
);

INSERT INTO api_key_scopes (api_key_id, permission_id)
SELECT ak.id, p.id
FROM api_keys ak
CROSS JOIN LATERAL unnest(ak.scopes) AS s(scope_value)
JOIN permissions p
  ON p.action = split_part(s.scope_value, ':', 1)
 AND p.resource = split_part(s.scope_value, ':', 2)
WHERE position(':' IN s.scope_value) > 0
ON CONFLICT DO NOTHING;

ALTER TABLE api_keys ALTER COLUMN client_id SET NOT NULL;
ALTER TABLE api_keys ADD CONSTRAINT api_keys_client_id_fkey FOREIGN KEY (client_id) REFERENCES clients(id) ON DELETE RESTRICT;
ALTER TABLE api_keys ADD CONSTRAINT api_keys_created_by_fkey FOREIGN KEY (created_by) REFERENCES users(id) ON DELETE SET NULL;
ALTER TABLE api_keys DROP CONSTRAINT IF EXISTS api_keys_user_id_fkey;
ALTER TABLE api_keys DROP COLUMN IF EXISTS user_id;
ALTER TABLE api_keys DROP COLUMN IF EXISTS scopes;

CREATE INDEX IF NOT EXISTS idx_apikeys_client ON api_keys(client_id) WHERE deleted_at IS NULL;

-- +goose Down
DROP INDEX IF EXISTS idx_apikeys_client;
ALTER TABLE api_keys ADD COLUMN IF NOT EXISTS user_id UUID;
ALTER TABLE api_keys ADD COLUMN IF NOT EXISTS scopes TEXT[] NOT NULL DEFAULT '{}';

UPDATE api_keys ak
SET user_id = c.owner_user_id
FROM clients c
WHERE ak.client_id = c.id;

UPDATE api_keys ak
SET scopes = COALESCE(scope_rows.scopes, '{}')
FROM (
    SELECT aks.api_key_id, array_agg(p.action || ':' || p.resource ORDER BY p.action, p.resource) AS scopes
    FROM api_key_scopes aks
    JOIN permissions p ON p.id = aks.permission_id
    GROUP BY aks.api_key_id
) scope_rows
WHERE ak.id = scope_rows.api_key_id;

ALTER TABLE api_keys DROP CONSTRAINT IF EXISTS api_keys_created_by_fkey;
ALTER TABLE api_keys DROP CONSTRAINT IF EXISTS api_keys_client_id_fkey;
ALTER TABLE api_keys ADD CONSTRAINT api_keys_user_id_fkey FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE;
DROP TABLE IF EXISTS api_key_scopes;
ALTER TABLE api_keys DROP COLUMN IF EXISTS created_by;
ALTER TABLE api_keys DROP COLUMN IF EXISTS client_id;
ALTER TABLE api_keys DROP COLUMN IF EXISTS deleted_at;
ALTER TABLE api_keys DROP COLUMN IF EXISTS revoked_at;

DROP TRIGGER IF EXISTS trg_clients_updated ON clients;
DROP INDEX IF EXISTS clients_name_active_unique;
DROP INDEX IF EXISTS idx_clients_owner_user;
DROP TABLE IF EXISTS clients;

DROP TRIGGER IF EXISTS trg_permissions_updated ON permissions;
DROP TRIGGER IF EXISTS trg_roles_updated ON roles;
DROP TRIGGER IF EXISTS trg_aggregation_steps_updated ON aggregation_steps;
DROP TRIGGER IF EXISTS trg_cors_configs_updated ON cors_configs;
DROP TRIGGER IF EXISTS trg_aggregation_configs_updated ON aggregation_configs;
DROP TRIGGER IF EXISTS trg_ip_blacklist_updated ON ip_blacklist;
DROP TRIGGER IF EXISTS trg_rate_limit_policies_updated ON rate_limit_policies;

DROP INDEX IF EXISTS roles_name_active_unique;
DROP INDEX IF EXISTS permissions_resource_action_active_unique;
DROP INDEX IF EXISTS cors_configs_route_id_active_unique;
DROP INDEX IF EXISTS route_plugins_route_plugin_active_unique;
DROP INDEX IF EXISTS service_instances_service_host_port_active_unique;
DROP INDEX IF EXISTS aggregation_steps_aggregation_sequence_active_unique;
DROP INDEX IF EXISTS aggregation_configs_path_method_active_unique;
DROP INDEX IF EXISTS aggregation_configs_name_active_unique;
DROP INDEX IF EXISTS rate_limit_policies_name_active_unique;
DROP INDEX IF EXISTS api_keys_key_hash_active_unique;
DROP INDEX IF EXISTS users_email_active_unique;
DROP INDEX IF EXISTS users_username_active_unique;
DROP INDEX IF EXISTS gateway_plugins_code_active_unique;
DROP INDEX IF EXISTS routes_path_method_active_unique;
DROP INDEX IF EXISTS services_name_active_unique;

ALTER TABLE services ADD CONSTRAINT services_name_unique UNIQUE (name);
ALTER TABLE routes ADD CONSTRAINT routes_path_method_unique UNIQUE (path, method);
ALTER TABLE gateway_plugins ADD CONSTRAINT gateway_plugins_code_unique UNIQUE (code);
ALTER TABLE users ADD CONSTRAINT users_username_unique UNIQUE (username);
ALTER TABLE users ADD CONSTRAINT users_email_unique UNIQUE (email);
ALTER TABLE api_keys ADD CONSTRAINT api_keys_key_hash_unique UNIQUE (key_hash);
ALTER TABLE rate_limit_policies ADD CONSTRAINT rate_limit_policies_name_unique UNIQUE (name);
ALTER TABLE aggregation_configs ADD CONSTRAINT aggregation_configs_name_unique UNIQUE (name);
ALTER TABLE aggregation_configs ADD CONSTRAINT aggregation_configs_path_method_unique UNIQUE (path, method);
ALTER TABLE aggregation_steps ADD CONSTRAINT aggregation_steps_aggregation_sequence_unique UNIQUE (aggregation_id, sequence);
ALTER TABLE service_instances ADD CONSTRAINT service_instances_service_host_port_unique UNIQUE (service_id, host, port);
ALTER TABLE route_plugins ADD CONSTRAINT route_plugins_route_plugin_unique UNIQUE (route_id, plugin_id);
ALTER TABLE cors_configs ADD CONSTRAINT cors_configs_route_id_unique UNIQUE (route_id);
ALTER TABLE permissions ADD CONSTRAINT permissions_resource_action_unique UNIQUE (resource, action);
ALTER TABLE roles ADD CONSTRAINT roles_name_unique UNIQUE (name);

ALTER TABLE role_permissions DROP COLUMN IF EXISTS created_at;
ALTER TABLE permissions DROP COLUMN IF EXISTS deleted_at;
ALTER TABLE permissions DROP COLUMN IF EXISTS updated_at;
ALTER TABLE permissions DROP COLUMN IF EXISTS created_at;
ALTER TABLE permissions DROP COLUMN IF EXISTS is_active;
ALTER TABLE roles DROP COLUMN IF EXISTS deleted_at;
ALTER TABLE roles DROP COLUMN IF EXISTS updated_at;
ALTER TABLE roles DROP COLUMN IF EXISTS created_at;
ALTER TABLE roles DROP COLUMN IF EXISTS is_active;
ALTER TABLE aggregation_steps DROP COLUMN IF EXISTS deleted_at;
ALTER TABLE aggregation_steps DROP COLUMN IF EXISTS updated_at;
ALTER TABLE aggregation_steps DROP COLUMN IF EXISTS created_at;
ALTER TABLE aggregation_steps DROP COLUMN IF EXISTS is_active;
ALTER TABLE aggregation_configs DROP COLUMN IF EXISTS deleted_at;
ALTER TABLE aggregation_configs DROP COLUMN IF EXISTS updated_at;
ALTER TABLE users DROP COLUMN IF EXISTS deleted_at;
ALTER TABLE route_plugins DROP COLUMN IF EXISTS deleted_at;
ALTER TABLE gateway_plugins DROP COLUMN IF EXISTS deleted_at;
ALTER TABLE cors_configs DROP COLUMN IF EXISTS deleted_at;
ALTER TABLE cors_configs DROP COLUMN IF EXISTS updated_at;
ALTER TABLE cors_configs DROP COLUMN IF EXISTS created_at;
ALTER TABLE cors_configs DROP COLUMN IF EXISTS is_active;
ALTER TABLE rate_limit_policies DROP COLUMN IF EXISTS deleted_at;
ALTER TABLE routes DROP COLUMN IF EXISTS deleted_at;
ALTER TABLE service_instances DROP COLUMN IF EXISTS deleted_at;
ALTER TABLE services DROP COLUMN IF EXISTS deleted_at;
