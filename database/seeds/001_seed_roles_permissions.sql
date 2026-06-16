-- Seed roles, permissions, and role-permission mappings.
-- Safe to run multiple times.

BEGIN;

INSERT INTO roles (id, name, description)
VALUES
    ('11111111-1111-1111-1111-111111111111', 'admin', 'System administrator'),
    ('22222222-2222-2222-2222-222222222222', 'developer', 'API developer')
ON CONFLICT (name) DO NOTHING;

INSERT INTO permissions (id, resource, action)
VALUES
    ('30000000-0000-0000-0000-000000000001', 'services', 'read'),
    ('30000000-0000-0000-0000-000000000002', 'services', 'write'),
    ('30000000-0000-0000-0000-000000000003', 'routes', 'read'),
    ('30000000-0000-0000-0000-000000000004', 'routes', 'write'),
    ('30000000-0000-0000-0000-000000000005', 'api_keys', 'read'),
    ('30000000-0000-0000-0000-000000000006', 'api_keys', 'write'),
    ('30000000-0000-0000-0000-000000000007', 'logs', 'read')
ON CONFLICT (resource, action) DO NOTHING;

INSERT INTO role_permissions (role_id, permission_id)
SELECT
    '11111111-1111-1111-1111-111111111111'::uuid,
    p.id
FROM permissions p
ON CONFLICT (role_id, permission_id) DO NOTHING;

INSERT INTO role_permissions (role_id, permission_id)
SELECT
    '22222222-2222-2222-2222-222222222222'::uuid,
    p.id
FROM permissions p
WHERE
    (p.resource IN ('services', 'routes', 'logs') AND p.action = 'read')
    OR (p.resource = 'api_keys' AND p.action IN ('read', 'write'))
ON CONFLICT (role_id, permission_id) DO NOTHING;

COMMIT;
