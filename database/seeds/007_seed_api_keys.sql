-- Seed demo API key.
-- Raw demo key suggestion: gw_demo_dev_key
-- This raw key is for local development only. Production keys must be created
-- through POST /admin/api-keys so the raw value is returned only once.

BEGIN;

INSERT INTO clients (
    id,
    name,
    client_type,
    owner_user_id,
    is_active
)
VALUES (
    'b0000000-0000-0000-0000-000000000001',
    'demo-developer-client',
    'internal_app',
    '40000000-0000-0000-0000-000000000002',
    TRUE
)
ON CONFLICT (id) DO UPDATE SET
    name = EXCLUDED.name,
    client_type = EXCLUDED.client_type,
    owner_user_id = EXCLUDED.owner_user_id,
    is_active = EXCLUDED.is_active;

INSERT INTO api_keys (
    id,
    key_hash,
    key_prefix,
    label,
    client_id,
    rate_limit_id,
    expires_at,
    is_active,
    created_by
)
VALUES
    (
        'a0000000-0000-0000-0000-000000000001',
        '2d0a5af466f6c08642ae831a9ed890eb3f134b871b9c007c466e176d12eeccdd',
        'gw_demo',
        'Demo developer API key',
        'b0000000-0000-0000-0000-000000000001',
        '50000000-0000-0000-0000-000000000003',
        now() + interval '90 days',
        TRUE,
        '40000000-0000-0000-0000-000000000001'
    )
ON CONFLICT (id) DO UPDATE SET
    key_hash = EXCLUDED.key_hash,
    key_prefix = EXCLUDED.key_prefix,
    label = EXCLUDED.label,
    client_id = EXCLUDED.client_id,
    rate_limit_id = EXCLUDED.rate_limit_id,
    expires_at = EXCLUDED.expires_at,
    is_active = EXCLUDED.is_active,
    revoked_at = NULL,
    deleted_at = NULL,
    created_by = EXCLUDED.created_by;

INSERT INTO api_key_scopes (api_key_id, scope_id)
VALUES
    ('a0000000-0000-0000-0000-000000000001', 'c0000000-0000-0000-0000-000000000201'),
    ('a0000000-0000-0000-0000-000000000001', 'c0000000-0000-0000-0000-000000000202')
ON CONFLICT DO NOTHING;

COMMIT;
