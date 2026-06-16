CREATE OR REPLACE FUNCTION set_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- 1. services
CREATE TABLE services (
    id                      UUID PRIMARY KEY,
    name                    VARCHAR(100) NOT NULL UNIQUE,
    description             TEXT,
    protocol                VARCHAR(10) NOT NULL DEFAULT 'http'
                                CHECK (protocol IN ('http', 'grpc')),
    lb_strategy             VARCHAR(20) NOT NULL DEFAULT 'round_robin'
                                CHECK (lb_strategy IN ('round_robin', 'weighted')),
    timeout_ms              INTEGER NOT NULL DEFAULT 5000 CHECK (timeout_ms > 0),
    retry_count             SMALLINT NOT NULL DEFAULT 0 CHECK (retry_count >= 0),
    circuit_breaker_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    is_active               BOOLEAN NOT NULL DEFAULT TRUE,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- 2. service_instances
CREATE TABLE service_instances (
    id          UUID PRIMARY KEY,
    service_id  UUID NOT NULL REFERENCES services(id) ON DELETE CASCADE,
    host        VARCHAR(255) NOT NULL,
    port        INTEGER NOT NULL CHECK (port BETWEEN 1 AND 65535),
    weight      SMALLINT NOT NULL DEFAULT 1 CHECK (weight >= 0),
    is_active   BOOLEAN NOT NULL DEFAULT TRUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (service_id, host, port)
);

CREATE INDEX idx_instances_service
ON service_instances(service_id)
WHERE is_active;

-- 3. rate_limit_policies
CREATE TABLE rate_limit_policies (
    id              UUID PRIMARY KEY,
    name            VARCHAR(100) NOT NULL UNIQUE,
    limit_type      VARCHAR(10) NOT NULL
                        CHECK (limit_type IN ('ip', 'user', 'api_key')),
    max_requests    INTEGER NOT NULL CHECK (max_requests > 0),
    window_seconds  INTEGER NOT NULL CHECK (window_seconds > 0),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- 4. routes
CREATE TABLE routes (
    id              UUID PRIMARY KEY,
    path            VARCHAR(255) NOT NULL,
    method          VARCHAR(10) NOT NULL DEFAULT 'GET'
                        CHECK (method IN ('GET','POST','PUT','PATCH','DELETE','HEAD','OPTIONS','ANY')),
    service_id      UUID NOT NULL REFERENCES services(id) ON DELETE RESTRICT,
    strip_prefix    BOOLEAN NOT NULL DEFAULT FALSE,
    rewrite_target  VARCHAR(255),
    auth_required   BOOLEAN NOT NULL DEFAULT TRUE,
    rate_limit_id   UUID REFERENCES rate_limit_policies(id) ON DELETE SET NULL,
    priority        INTEGER NOT NULL DEFAULT 0,
    is_active       BOOLEAN NOT NULL DEFAULT TRUE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (path, method)
);

CREATE INDEX idx_routes_lookup
ON routes(path, method)
WHERE is_active;

-- 5. cors_configs
CREATE TABLE cors_configs (
    id                UUID PRIMARY KEY,
    route_id          UUID NOT NULL UNIQUE REFERENCES routes(id) ON DELETE CASCADE,
    allowed_origins   TEXT[] NOT NULL DEFAULT '{}',
    allowed_methods   TEXT[] NOT NULL DEFAULT '{}',
    allowed_headers   TEXT[] NOT NULL DEFAULT '{}',
    allow_credentials BOOLEAN NOT NULL DEFAULT FALSE,
    max_age           INTEGER NOT NULL DEFAULT 3600
);

-- 6. roles
CREATE TABLE roles (
    id          UUID PRIMARY KEY,
    name        VARCHAR(50) NOT NULL UNIQUE,
    description TEXT
);

-- 7. permissions
CREATE TABLE permissions (
    id          UUID PRIMARY KEY,
    resource    VARCHAR(100) NOT NULL,
    action      VARCHAR(50)  NOT NULL,
    UNIQUE (resource, action)
);

-- 8. role_permissions
CREATE TABLE role_permissions (
    role_id       UUID NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    permission_id UUID NOT NULL REFERENCES permissions(id) ON DELETE CASCADE,
    PRIMARY KEY (role_id, permission_id)
);

-- 9. users
CREATE TABLE users (
    id            UUID PRIMARY KEY,
    username      VARCHAR(50)  NOT NULL UNIQUE,
    email         VARCHAR(255) NOT NULL UNIQUE,
    password_hash VARCHAR(255) NOT NULL,
    role_id       UUID NOT NULL REFERENCES roles(id) ON DELETE RESTRICT,
    is_active     BOOLEAN NOT NULL DEFAULT TRUE,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- 10. api_keys
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

-- 11. ip_blacklist
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

-- 12. aggregation_configs
CREATE TABLE aggregation_configs (
    id          UUID PRIMARY KEY,
    name        VARCHAR(100) NOT NULL UNIQUE,
    path        VARCHAR(255) NOT NULL,
    method      VARCHAR(10)  NOT NULL DEFAULT 'GET',
    is_active   BOOLEAN NOT NULL DEFAULT TRUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (path, method)
);

-- 13. aggregation_steps
CREATE TABLE aggregation_steps (
    id               UUID PRIMARY KEY,
    aggregation_id   UUID NOT NULL REFERENCES aggregation_configs(id) ON DELETE CASCADE,
    service_id       UUID NOT NULL REFERENCES services(id) ON DELETE RESTRICT,
    sequence         SMALLINT NOT NULL,
    depends_on       UUID REFERENCES aggregation_steps(id) ON DELETE SET NULL,
    is_required      BOOLEAN NOT NULL DEFAULT TRUE,
    request_template JSONB,
    response_mapping JSONB,
    UNIQUE (aggregation_id, sequence)
);

-- 14. audit_logs
CREATE TABLE audit_logs (
    id          BIGSERIAL PRIMARY KEY,
    user_id     UUID REFERENCES users(id) ON DELETE SET NULL,
    action      VARCHAR(20) NOT NULL CHECK (action IN ('create', 'update', 'delete')),
    entity_type VARCHAR(50) NOT NULL,
    entity_id   UUID,
    old_value   JSONB,
    new_value   JSONB,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_audit_entity
ON audit_logs(entity_type, entity_id);

CREATE INDEX idx_audit_time
ON audit_logs(created_at DESC);

-- triggers updated_at
CREATE TRIGGER trg_services_updated
BEFORE UPDATE ON services
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER trg_routes_updated
BEFORE UPDATE ON routes
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER trg_users_updated
BEFORE UPDATE ON users
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER trg_api_keys_updated
BEFORE UPDATE ON api_keys
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

-- copy file vào container -> chạy
-- docker cp .\gateway-schema.sql gateway-postgres:/tmp/gateway-schema.sql
-- docker exec -it gateway-postgres psql -U gateway_user -d gateway_db -f /tmp/gateway-schema.sql
-- uuid v7 sinh ở backend
-- migration dùng goose: github.com/pressly/goose/v3/cmd/goose@latest
