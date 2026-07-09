-- +goose Up
WITH desired_permissions(id, resource, action) AS (
    VALUES
        ('01972f6a-0002-7000-8000-000000000031'::uuid, 'cors_policies', 'read'),
        ('01972f6a-0002-7000-8000-000000000032'::uuid, 'cors_policies', 'write')
)
INSERT INTO permissions (id, resource, action, is_active)
SELECT id, resource, action, TRUE
FROM desired_permissions dp
WHERE NOT EXISTS (
    SELECT 1
    FROM permissions p
    WHERE p.resource = dp.resource
      AND p.action = dp.action
      AND p.deleted_at IS NULL
)
ON CONFLICT (id) DO UPDATE SET
    resource = EXCLUDED.resource,
    action = EXCLUDED.action,
    is_active = TRUE,
    updated_at = now(),
    deleted_at = NULL;

WITH desired_permissions(resource, action) AS (
    VALUES
        ('cors_policies', 'read'),
        ('cors_policies', 'write')
)
UPDATE permissions p
SET is_active = TRUE,
    updated_at = now(),
    deleted_at = NULL
FROM desired_permissions dp
WHERE p.resource = dp.resource
  AND p.action = dp.action;

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r
JOIN permissions p
  ON p.resource = 'cors_policies'
 AND p.action IN ('read', 'write')
 AND p.is_active = TRUE
 AND p.deleted_at IS NULL
WHERE r.id = '01972f6a-0001-7000-8000-000000000001'
  AND r.name = 'admin'
  AND r.is_active = TRUE
  AND r.deleted_at IS NULL
ON CONFLICT (role_id, permission_id) DO NOTHING;

-- +goose Down
WITH target_permissions AS (
    SELECT id
    FROM permissions
    WHERE resource = 'cors_policies'
      AND action IN ('read', 'write')
)
DELETE FROM role_permissions rp
USING target_permissions tp
WHERE rp.permission_id = tp.id;

UPDATE permissions
SET is_active = FALSE,
    updated_at = now(),
    deleted_at = now()
WHERE resource = 'cors_policies'
  AND action IN ('read', 'write');