-- Seed gateway test services.

BEGIN;

INSERT INTO services (
    id, name, description, protocol, lb_strategy,
    timeout_ms, retry_count, circuit_breaker_enabled, is_active
)
VALUES
    (
        '60000000-0000-0000-0000-000000000101',
        'auth-service',
        'Laravel auth/user service for gateway testing',
        'http',
        'round_robin',
        5000,
        1,
        FALSE,
        TRUE
    ),
    (
        '60000000-0000-0000-0000-000000000102',
        'product-service',
        'Laravel product service for gateway testing',
        'http',
        'round_robin',
        5000,
        1,
        FALSE,
        TRUE
    ),
    (
        '60000000-0000-0000-0000-000000000103',
        'order-service',
        'Laravel order service for gateway testing',
        'http',
        'round_robin',
        5000,
        1,
        FALSE,
        TRUE
    )
ON CONFLICT (name) DO UPDATE SET
    description = EXCLUDED.description,
    protocol = EXCLUDED.protocol,
    lb_strategy = EXCLUDED.lb_strategy,
    timeout_ms = EXCLUDED.timeout_ms,
    retry_count = EXCLUDED.retry_count,
    circuit_breaker_enabled = EXCLUDED.circuit_breaker_enabled,
    is_active = EXCLUDED.is_active;

COMMIT;
