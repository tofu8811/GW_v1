-- Seed default rate limit policies.

BEGIN;

INSERT INTO rate_limit_policies (id, name, limit_type, max_requests, window_seconds)
VALUES
    ('50000000-0000-0000-0000-000000000001', 'Default IP limit', 'ip', 100, 60),
    ('50000000-0000-0000-0000-000000000002', 'Default user limit', 'user', 1000, 60),
    ('50000000-0000-0000-0000-000000000003', 'Default API key limit', 'api_key', 500, 60)
ON CONFLICT (name) DO NOTHING;

COMMIT;
