-- Seed demo aggregation endpoint.

BEGIN;

INSERT INTO aggregation_configs (id, name, path, method, is_active)
VALUES
    (
        'b0000000-0000-0000-0000-000000000001',
        'demo-dashboard',
        '/api/dashboard',
        'GET',
        TRUE
    )
ON CONFLICT (path, method) DO NOTHING;

INSERT INTO aggregation_steps (
    id,
    aggregation_id,
    service_id,
    sequence,
    depends_on,
    is_required,
    request_template,
    response_mapping
)
VALUES
    (
        'c0000000-0000-0000-0000-000000000001',
        'b0000000-0000-0000-0000-000000000001',
        '60000000-0000-0000-0000-000000000001',
        1,
        NULL,
        TRUE,
        '{"method":"GET","path":"/products"}',
        '{"target":"products"}'
    ),
    (
        'c0000000-0000-0000-0000-000000000002',
        'b0000000-0000-0000-0000-000000000001',
        '60000000-0000-0000-0000-000000000002',
        2,
        NULL,
        FALSE,
        '{"method":"GET","path":"/users"}',
        '{"target":"users"}'
    )
ON CONFLICT (aggregation_id, sequence) DO NOTHING;

COMMIT;
