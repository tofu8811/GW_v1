-- +goose Up
CREATE TABLE services (
    id                      UUID CONSTRAINT services_pkey PRIMARY KEY,
    name                    VARCHAR(100) NOT NULL CONSTRAINT services_name_unique UNIQUE,
    description             TEXT,
    protocol                VARCHAR(10) NOT NULL DEFAULT 'http'
                                CONSTRAINT services_protocol_check CHECK (protocol IN ('http', 'grpc')),
    lb_strategy             VARCHAR(20) NOT NULL DEFAULT 'round_robin'
                                CONSTRAINT services_lb_strategy_check CHECK (lb_strategy IN ('round_robin', 'weighted')),
    timeout_ms              INTEGER NOT NULL DEFAULT 5000 CONSTRAINT services_timeout_ms_check CHECK (timeout_ms > 0),
    retry_count             SMALLINT NOT NULL DEFAULT 0 CONSTRAINT services_retry_count_check CHECK (retry_count >= 0),
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
