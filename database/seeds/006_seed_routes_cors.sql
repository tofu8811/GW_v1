-- Seed gateway test routes, API scopes, and CORS configs.

BEGIN;

INSERT INTO api_scopes (
    id, service_id, code, resource, action, description, is_active
)
VALUES
    (
        'c0000000-0000-0000-0000-000000000101',
        '60000000-0000-0000-0000-000000000102',
        'products:read',
        'products',
        'read',
        'Read products through product-service',
        TRUE
    ),
    (
        'c0000000-0000-0000-0000-000000000102',
        '60000000-0000-0000-0000-000000000102',
        'products:write',
        'products',
        'write',
        'Write products through product-service',
        TRUE
    ),
    (
        'c0000000-0000-0000-0000-000000000201',
        '60000000-0000-0000-0000-000000000103',
        'orders:read',
        'orders',
        'read',
        'Read orders through order-service',
        TRUE
    ),
    (
        'c0000000-0000-0000-0000-000000000202',
        '60000000-0000-0000-0000-000000000103',
        'orders:write',
        'orders',
        'write',
        'Write orders through order-service',
        TRUE
    )
ON CONFLICT (id) DO UPDATE SET
    service_id = EXCLUDED.service_id,
    code = EXCLUDED.code,
    resource = EXCLUDED.resource,
    action = EXCLUDED.action,
    description = EXCLUDED.description,
    is_active = EXCLUDED.is_active,
    deleted_at = NULL;

INSERT INTO routes (
    id, path, method, service_id, strip_prefix,
    rewrite_target, auth_required, required_scope_id, rate_limit_id,
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
        'c0000000-0000-0000-0000-000000000201',
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
        'c0000000-0000-0000-0000-000000000202',
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
        'c0000000-0000-0000-0000-000000000201',
        NULL,
        100,
        TRUE
    )
ON CONFLICT (id) DO UPDATE SET
    path = EXCLUDED.path,
    method = EXCLUDED.method,
    service_id = EXCLUDED.service_id,
    strip_prefix = EXCLUDED.strip_prefix,
    rewrite_target = EXCLUDED.rewrite_target,
    auth_required = EXCLUDED.auth_required,
    required_scope_id = EXCLUDED.required_scope_id,
    rate_limit_id = EXCLUDED.rate_limit_id,
    priority = EXCLUDED.priority,
    is_active = EXCLUDED.is_active,
    deleted_at = NULL;

COMMIT;
