-- Seed gateway test service instances.

BEGIN;

INSERT INTO service_instances (id, service_id, host, port, weight, is_active)
VALUES
    (
        '70000000-0000-0000-0000-000000000101',
        '60000000-0000-0000-0000-000000000101',
        'localhost',
        3000,
        5,
        TRUE
    ),
    (
        '70000000-0000-0000-0000-000000000104',
        '60000000-0000-0000-0000-000000000101',
        'localhost',
        3003,
        3,
        TRUE
    ),
    (
        '70000000-0000-0000-0000-000000000105',
        '60000000-0000-0000-0000-000000000101',
        'localhost',
        3004,
        1,
        TRUE
    ),
    (
        '70000000-0000-0000-0000-000000000102',
        '60000000-0000-0000-0000-000000000102',
        'localhost',
        3001,
        1,
        TRUE
    ),
    (
        '70000000-0000-0000-0000-000000000103',
        '60000000-0000-0000-0000-000000000103',
        'localhost',
        3002,
        1,
        TRUE
    )
ON CONFLICT (service_id, host, port) DO UPDATE SET
    weight = EXCLUDED.weight,
    is_active = EXCLUDED.is_active;

COMMIT;
