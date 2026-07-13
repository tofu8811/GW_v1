-- +goose Up
WITH developer_role AS (
    SELECT id
    FROM roles
    WHERE name = 'developer'
      AND is_active = TRUE
      AND deleted_at IS NULL
),
developer_permissions(resource, action) AS (
    VALUES
        ('health', 'read'),
        ('logs', 'read'),
        ('metrics', 'read')
),
allowed_permissions AS (
    SELECT p.id
    FROM permissions p
    JOIN developer_permissions dp
      ON p.resource = dp.resource
     AND p.action = dp.action
    WHERE p.is_active = TRUE
      AND p.deleted_at IS NULL
)
DELETE FROM role_permissions rp
USING developer_role dr
WHERE rp.role_id = dr.id
  AND NOT EXISTS (
      SELECT 1
      FROM allowed_permissions ap
      WHERE ap.id = rp.permission_id
  );

WITH developer_role AS (
    SELECT id
    FROM roles
    WHERE name = 'developer'
      AND is_active = TRUE
      AND deleted_at IS NULL
),
developer_permissions(resource, action) AS (
    VALUES
        ('health', 'read'),
        ('logs', 'read'),
        ('metrics', 'read')
)
INSERT INTO role_permissions (role_id, permission_id)
SELECT dr.id, p.id
FROM developer_role dr
JOIN developer_permissions dp
  ON TRUE
JOIN permissions p
  ON p.resource = dp.resource
 AND p.action = dp.action
 AND p.is_active = TRUE
 AND p.deleted_at IS NULL
ON CONFLICT (role_id, permission_id) DO NOTHING;

-- +goose Down
WITH developer_role AS (
    SELECT id
    FROM roles
    WHERE name = 'developer'
      AND is_active = TRUE
      AND deleted_at IS NULL
),
developer_permissions(resource, action) AS (
    VALUES
        ('aggregations', 'read'),
        ('aggregations', 'write'),
        ('api_keys', 'read'),
        ('api_keys', 'write'),
        ('audit_logs', 'read'),
        ('health', 'read'),
        ('logs', 'read'),
        ('metrics', 'read'),
        ('rate_limits', 'read'),
        ('rate_limits', 'write'),
        ('routes', 'read'),
        ('services', 'read')
)
INSERT INTO role_permissions (role_id, permission_id)
SELECT dr.id, p.id
FROM developer_role dr
JOIN developer_permissions dp
  ON TRUE
JOIN permissions p
  ON p.resource = dp.resource
 AND p.action = dp.action
 AND p.is_active = TRUE
 AND p.deleted_at IS NULL
ON CONFLICT (role_id, permission_id) DO NOTHING;
