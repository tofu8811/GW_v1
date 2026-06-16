-- Seed demo routes and CORS configs.

BEGIN;

INSERT INTO routes (
    id,
    path,
    method,
    service_id,
    strip_prefix,
    rewrite_target,
    auth_required,
    rate_limit_id,
    priority,
    is_active
)
VALUES
    (
        '80000000-0000-0000-0000-000000000001',
        '/api/products',
        'GET',
        '60000000-0000-0000-0000-000000000001',
        TRUE,
        '/products',
        FALSE,
        '50000000-0000-0000-0000-000000000001',
        10,
        TRUE
    ),
    (
        '80000000-0000-0000-0000-000000000002',
        '/api/users',
        'GET',
        '60000000-0000-0000-0000-000000000002',
        TRUE,
        '/users',
        TRUE,
        '50000000-0000-0000-0000-000000000002',
        10,
        TRUE
    )
ON CONFLICT (path, method) DO NOTHING;

INSERT INTO cors_configs (
    id,
    route_id,
    allowed_origins,
    allowed_methods,
    allowed_headers,
    allow_credentials,
    max_age
)
VALUES
    (
        '90000000-0000-0000-0000-000000000001',
        '80000000-0000-0000-0000-000000000001',
        ARRAY['http://localhost:3000', 'http://localhost:5173'],
        ARRAY['GET', 'OPTIONS'],
        ARRAY['Content-Type', 'Authorization', 'X-API-Key'],
        FALSE,
        3600
    ),
    (
        '90000000-0000-0000-0000-000000000002',
        '80000000-0000-0000-0000-000000000002',
        ARRAY['http://localhost:3000', 'http://localhost:5173'],
        ARRAY['GET', 'OPTIONS'],
        ARRAY['Content-Type', 'Authorization', 'X-API-Key'],
        TRUE,
        3600
    )
ON CONFLICT (route_id) DO NOTHING;

COMMIT;
