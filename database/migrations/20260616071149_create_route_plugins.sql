-- +goose Up
CREATE TABLE route_plugins (
    id              UUID PRIMARY KEY,
    route_id        UUID NOT NULL REFERENCES routes(id) ON DELETE CASCADE,
    plugin_id       UUID NOT NULL REFERENCES gateway_plugins(id) ON DELETE RESTRICT,
    priority        INTEGER NOT NULL DEFAULT 100,
    config          JSONB NOT NULL DEFAULT '{}'::jsonb,
    is_required     BOOLEAN NOT NULL DEFAULT TRUE,
    is_active       BOOLEAN NOT NULL DEFAULT TRUE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (route_id, plugin_id)
);

CREATE INDEX idx_route_plugins_route
ON route_plugins(route_id, priority)
WHERE is_active;

CREATE INDEX idx_route_plugins_plugin
ON route_plugins(plugin_id)
WHERE is_active;

CREATE TRIGGER trg_route_plugins_updated
BEFORE UPDATE ON route_plugins
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

-- +goose Down
DROP TRIGGER IF EXISTS trg_route_plugins_updated ON route_plugins;
DROP INDEX IF EXISTS idx_route_plugins_plugin;
DROP INDEX IF EXISTS idx_route_plugins_route;
DROP TABLE IF EXISTS route_plugins;
