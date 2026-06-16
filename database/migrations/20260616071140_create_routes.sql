-- +goose Up
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

CREATE TRIGGER trg_routes_updated
BEFORE UPDATE ON routes
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

-- +goose Down
DROP TRIGGER IF EXISTS trg_routes_updated ON routes;
DROP INDEX IF EXISTS idx_routes_lookup;
DROP TABLE IF EXISTS routes;
