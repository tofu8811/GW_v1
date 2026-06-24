-- +goose Up
CREATE TABLE service_instances (
    id          UUID CONSTRAINT service_instances_pkey PRIMARY KEY,
    service_id  UUID NOT NULL CONSTRAINT service_instances_service_id_fkey REFERENCES services(id) ON DELETE CASCADE,
    host        VARCHAR(255) NOT NULL,
    port        INTEGER NOT NULL CONSTRAINT service_instances_port_check CHECK (port BETWEEN 1 AND 65535),
    weight      SMALLINT NOT NULL DEFAULT 1 CONSTRAINT service_instances_weight_check CHECK (weight >= 0),
    is_active   BOOLEAN NOT NULL DEFAULT TRUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT service_instances_service_host_port_unique UNIQUE (service_id, host, port)
);

CREATE INDEX idx_instances_service
ON service_instances(service_id)
WHERE is_active;

-- +goose Down
DROP INDEX IF EXISTS idx_instances_service;
DROP TABLE IF EXISTS service_instances;
