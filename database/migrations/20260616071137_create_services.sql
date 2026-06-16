-- +goose Up
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

CREATE TRIGGER trg_services_updated
BEFORE UPDATE ON services
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

-- +goose Down
DROP TRIGGER IF EXISTS trg_services_updated ON services;
DROP TABLE IF EXISTS services;
