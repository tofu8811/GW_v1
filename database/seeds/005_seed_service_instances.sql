-- Seed demo service instances.

BEGIN;

INSERT INTO service_instances (id, service_id, host, port, weight, is_active)
VALUES
    (
        '70000000-0000-0000-0000-000000000001',
        '60000000-0000-0000-0000-000000000001',
        'localhost',
        8081,
        1,
        TRUE
    ),
    (
        '70000000-0000-0000-0000-000000000002',
        '60000000-0000-0000-0000-000000000001',
        'localhost',
        8082,
        1,
        TRUE
    ),
    (
        '70000000-0000-0000-0000-000000000003',
        '60000000-0000-0000-0000-000000000002',
        'localhost',
        8091,
        1,
        TRUE
    )
ON CONFLICT (service_id, host, port) DO NOTHING;

COMMIT;
