-- Seed demo API key.
-- Raw demo key suggestion: gw_demo_dev_key
-- Replace key_hash with SHA-256(raw_key) from backend implementation later.

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
        'replace_with_sha256_hash_of_gw_demo_dev_key',
        'gw_demo',
        'Demo developer API key',
        '40000000-0000-0000-0000-000000000002',
        ARRAY['services:read', 'routes:read'],
        '50000000-0000-0000-0000-000000000003',
        now() + interval '90 days',
        TRUE
    )
ON CONFLICT (key_hash) DO NOTHING;

COMMIT;
