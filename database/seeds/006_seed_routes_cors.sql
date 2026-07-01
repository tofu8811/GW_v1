-- Seed gateway test routes and CORS configs.

BEGIN;

INSERT INTO routes (
    id, path, method, service_id, strip_prefix,
    rewrite_target, auth_required, rate_limit_id,
    priority, is_active
)
VALUES
    (
        '80000000-0000-0000-0000-000000000101',
        '/api/user/register',
        'POST',
        '60000000-0000-0000-0000-000000000101',
        FALSE,
        '/api/user/register',
        FALSE,
        NULL,
        100,
        TRUE
    ),
    (
        '80000000-0000-0000-0000-000000000102',
        '/api/auth/login',
        'POST',
        '60000000-0000-0000-0000-000000000101',
        FALSE,
        '/api/auth/login',
        FALSE,
        NULL,
        100,
        TRUE
    ),
    (
        '80000000-0000-0000-0000-000000000103',
        '/api/products',
        'GET',
        '60000000-0000-0000-0000-000000000102',
        FALSE,
        '/api/products',
        FALSE,
        NULL,
        100,
        TRUE
    ),
    (
        '80000000-0000-0000-0000-000000000104',
        '/api/product/{id}',
        'GET',
        '60000000-0000-0000-0000-000000000102',
        FALSE,
        '/api/product/{id}',
        FALSE,
        NULL,
        100,
        TRUE
    ),
    (
        '80000000-0000-0000-0000-000000000105',
        '/api/orders',
        'GET',
        '60000000-0000-0000-0000-000000000103',
        FALSE,
        '/api/orders',
        TRUE,
        NULL,
        100,
        TRUE
    ),
    (
        '80000000-0000-0000-0000-000000000106',
        '/api/order/create',
        'POST',
        '60000000-0000-0000-0000-000000000103',
        FALSE,
        '/api/order/create',
        TRUE,
        NULL,
        100,
        TRUE
    ),
    (
        '80000000-0000-0000-0000-000000000107',
        '/api/order/{id}',
        'GET',
        '60000000-0000-0000-0000-000000000103',
        FALSE,
        '/api/order/{id}',
        TRUE,
        NULL,
        100,
        TRUE
    )
ON CONFLICT (path, method) DO UPDATE SET
    service_id = EXCLUDED.service_id,
    strip_prefix = EXCLUDED.strip_prefix,
    rewrite_target = EXCLUDED.rewrite_target,
    auth_required = EXCLUDED.auth_required,
    rate_limit_id = EXCLUDED.rate_limit_id,
    priority = EXCLUDED.priority,
    is_active = EXCLUDED.is_active;

-- INSERT INTO cors_configs (
--     id,
--     route_id,
--     allowed_origins,
--     allowed_methods,
--     allowed_headers,
--     allow_credentials,
--     max_age
-- )
-- VALUES
--     (
--         '90000000-0000-0000-0000-000000000101',
--         '80000000-0000-0000-0000-000000000101',
--         ARRAY['http://localhost:3000', 'http://localhost:5173'],
--         ARRAY['POST', 'OPTIONS'],
--         ARRAY['Content-Type', 'Authorization', 'X-API-Key'],
--         FALSE,
--         3600
--     ),
--     (
--         '90000000-0000-0000-0000-000000000102',
--         '80000000-0000-0000-0000-000000000102',
--         ARRAY['http://localhost:3000', 'http://localhost:5173'],
--         ARRAY['POST', 'OPTIONS'],
--         ARRAY['Content-Type', 'Authorization', 'X-API-Key'],
--         FALSE,
--         3600
--     ),
--     (
--         '90000000-0000-0000-0000-000000000103',
--         '80000000-0000-0000-0000-000000000103',
--         ARRAY['http://localhost:3000', 'http://localhost:5173'],
--         ARRAY['GET', 'OPTIONS'],
--         ARRAY['Content-Type', 'Authorization', 'X-API-Key'],
--         FALSE,
--         3600
--     ),
--     (
--         '90000000-0000-0000-0000-000000000104',
--         '80000000-0000-0000-0000-000000000104',
--         ARRAY['http://localhost:3000', 'http://localhost:5173'],
--         ARRAY['GET', 'OPTIONS'],
--         ARRAY['Content-Type', 'Authorization', 'X-API-Key'],
--         FALSE,
--         3600
--     ),
--     (
--         '90000000-0000-0000-0000-000000000105',
--         '80000000-0000-0000-0000-000000000105',
--         ARRAY['http://localhost:3000', 'http://localhost:5173'],
--         ARRAY['GET', 'OPTIONS'],
--         ARRAY['Content-Type', 'Authorization', 'X-API-Key'],
--         TRUE,
--         3600
--     ),
--     (
--         '90000000-0000-0000-0000-000000000106',
--         '80000000-0000-0000-0000-000000000106',
--         ARRAY['http://localhost:3000', 'http://localhost:5173'],
--         ARRAY['POST', 'OPTIONS'],
--         ARRAY['Content-Type', 'Authorization', 'X-API-Key'],
--         TRUE,
--         3600
--     ),
--     (
--         '90000000-0000-0000-0000-000000000107',
--         '80000000-0000-0000-0000-000000000107',
--         ARRAY['http://localhost:3000', 'http://localhost:5173'],
--         ARRAY['GET', 'OPTIONS'],
--         ARRAY['Content-Type', 'Authorization', 'X-API-Key'],
--         TRUE,
--         3600
--     )
-- ON CONFLICT (route_id) DO UPDATE SET
--     allowed_origins = EXCLUDED.allowed_origins,
--     allowed_methods = EXCLUDED.allowed_methods,
--     allowed_headers = EXCLUDED.allowed_headers,
--     allow_credentials = EXCLUDED.allow_credentials,
--     max_age = EXCLUDED.max_age;

COMMIT;
