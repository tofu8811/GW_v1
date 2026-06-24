-- +goose Up
CREATE TABLE gateway_plugins (
    id                  UUID CONSTRAINT gateway_plugins_pkey PRIMARY KEY,
    code                VARCHAR(50) NOT NULL CONSTRAINT gateway_plugins_code_unique UNIQUE,
    name                VARCHAR(100) NOT NULL,
    description         TEXT,
    phase               VARCHAR(30) NOT NULL
                            CONSTRAINT gateway_plugins_phase_check CHECK (phase IN (
                                'before_request',
                                'proxy',
                                'after_response',
                                'on_error'
                            )),
    default_priority    INTEGER NOT NULL DEFAULT 100,
    config_schema       JSONB,
    is_active           BOOLEAN NOT NULL DEFAULT TRUE,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_gateway_plugins_phase
ON gateway_plugins(phase)
WHERE is_active;

CREATE TRIGGER trg_gateway_plugins_updated
BEFORE UPDATE ON gateway_plugins
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

-- +goose Down
DROP TRIGGER IF EXISTS trg_gateway_plugins_updated ON gateway_plugins;
DROP INDEX IF EXISTS idx_gateway_plugins_phase;
DROP TABLE IF EXISTS gateway_plugins;
