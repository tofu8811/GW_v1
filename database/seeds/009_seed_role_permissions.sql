-- Seed role-permission mappings only.
-- Roles are expected to already exist.
-- Safe to run multiple times.

BEGIN;

-- Admin: full access to every active permission.
INSERT INTO role_permissions (role_id, permission_id)
SELECT
    r.id,
    p.id
FROM roles r
JOIN permissions p
    ON p.is_active = true
   AND p.deleted_at IS NULL
WHERE r.id = '01972f6a-0001-7000-8000-000000000001'
  AND r.name = 'admin'
  AND r.is_active = true
  AND r.deleted_at IS NULL
ON CONFLICT (role_id, permission_id) DO NOTHING;

-- Developer: can read routes/services, manage own API keys/scopes,
-- read logs/metrics, and read health.
WITH developer_permissions(resource, action) AS (
    VALUES
        ('services', 'read'),
        ('routes', 'read'),
        ('api_keys', 'read'),
        ('api_keys', 'write'),
        ('api_key_scopes', 'read'),
        ('api_key_scopes', 'write'),
        ('logs', 'read'),
        ('metrics', 'read'),
        ('health', 'read')
)
INSERT INTO role_permissions (role_id, permission_id)
SELECT
    r.id,
    p.id
FROM roles r
JOIN developer_permissions dp
    ON true
JOIN permissions p
    ON p.resource = dp.resource
   AND p.action = dp.action
   AND p.is_active = true
   AND p.deleted_at IS NULL
WHERE r.id = '01972f6a-0001-7000-8000-000000000002'
  AND r.name = 'developer'
  AND r.is_active = true
  AND r.deleted_at IS NULL
ON CONFLICT (role_id, permission_id) DO NOTHING;

COMMIT;
