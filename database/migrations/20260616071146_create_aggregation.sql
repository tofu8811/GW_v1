-- +goose Up
CREATE TABLE aggregation_configs (
    id          UUID CONSTRAINT aggregation_configs_pkey PRIMARY KEY,
    name        VARCHAR(100) NOT NULL CONSTRAINT aggregation_configs_name_unique UNIQUE,
    path        VARCHAR(255) NOT NULL,
    method      VARCHAR(10)  NOT NULL DEFAULT 'GET',
    is_active   BOOLEAN NOT NULL DEFAULT TRUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT aggregation_configs_path_method_unique UNIQUE (path, method)
);

CREATE TABLE aggregation_steps (
    id               UUID CONSTRAINT aggregation_steps_pkey PRIMARY KEY,
    aggregation_id   UUID NOT NULL CONSTRAINT aggregation_steps_aggregation_id_fkey REFERENCES aggregation_configs(id) ON DELETE CASCADE,
    service_id       UUID NOT NULL CONSTRAINT aggregation_steps_service_id_fkey REFERENCES services(id) ON DELETE RESTRICT,
    sequence         SMALLINT NOT NULL,
    depends_on       UUID CONSTRAINT aggregation_steps_depends_on_fkey REFERENCES aggregation_steps(id) ON DELETE SET NULL,
    is_required      BOOLEAN NOT NULL DEFAULT TRUE,
    request_template JSONB,
    response_mapping JSONB,
    CONSTRAINT aggregation_steps_aggregation_sequence_unique UNIQUE (aggregation_id, sequence)
);

-- +goose Down
DROP TABLE IF EXISTS aggregation_steps;
DROP TABLE IF EXISTS aggregation_configs;
