-- Seed demo services.

BEGIN;

INSERT INTO services (
    id,
    name,
    description,
    protocol,
    lb_strategy,
    timeout_ms,
    retry_count,
    circuit_breaker_enabled,
    is_active
)
VALUES
    (
        '60000000-0000-0000-0000-000000000001',
        'product-service',
        'Demo product microservice',
        'http',
        'round_robin',
        5000,
        1,
        FALSE,
        TRUE
    ),
    (
        '60000000-0000-0000-0000-000000000002',
        'user-service',
        'Demo user microservice',
        'http',
        'round_robin',
        5000,
        1,
        FALSE,
        TRUE
    )
ON CONFLICT (name) DO NOTHING;

COMMIT;
