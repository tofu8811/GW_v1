-- +goose Up
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

-- +goose Down
DROP INDEX IF EXISTS idx_instances_service;
DROP TABLE IF EXISTS service_instances;
