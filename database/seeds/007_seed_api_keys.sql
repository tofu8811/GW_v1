-- Seed demo API key.
-- Raw demo key suggestion: gw_demo_dev_key
-- This raw key is for local development only. Production keys must be created
-- through POST /admin/api-keys so the raw value is returned only once.

BEGIN;

INSERT INTO api_keys (
    id,
    key_hash,
    key_prefix,
    label,
    user_id,
    scopes,
    rate_limit_id,
    expires_at,
    is_active
)
VALUES
    (
        'a0000000-0000-0000-0000-000000000001',
		'2d0a5af466f6c08642ae831a9ed890eb3f134b871b9c007c466e176d12eeccdd',
        'gw_demo',
        'Demo developer API key',
        '40000000-0000-0000-0000-000000000002',
		ARRAY['GET:/api/orders', 'POST:/api/order/create', 'GET:/api/order/{id}'],
        '50000000-0000-0000-0000-000000000003',
        now() + interval '90 days',
        TRUE
    )
ON CONFLICT (id) DO UPDATE SET
    key_hash = EXCLUDED.key_hash,
    key_prefix = EXCLUDED.key_prefix,
    label = EXCLUDED.label,
    user_id = EXCLUDED.user_id,
    scopes = EXCLUDED.scopes,
    rate_limit_id = EXCLUDED.rate_limit_id,
    expires_at = EXCLUDED.expires_at,
    is_active = EXCLUDED.is_active;

COMMIT;
