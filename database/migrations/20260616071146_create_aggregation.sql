-- +goose Up
CREATE TABLE aggregation_configs (
    id          UUID PRIMARY KEY,
    name        VARCHAR(100) NOT NULL UNIQUE,
    path        VARCHAR(255) NOT NULL,
    method      VARCHAR(10)  NOT NULL DEFAULT 'GET',
    is_active   BOOLEAN NOT NULL DEFAULT TRUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (path, method)
);

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

-- +goose Down
DROP TABLE IF EXISTS aggregation_steps;
DROP TABLE IF EXISTS aggregation_configs;
