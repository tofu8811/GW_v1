-- Seed roles, permissions, and role-permission mappings.
-- Safe to run multiple times.

BEGIN;

-- Seed roles
INSERT INTO roles (id, name, description) VALUES
  ('01972f6a-0001-7000-8000-000000000001', 'admin', 'System administrator'),
  ('01972f6a-0001-7000-8000-000000000002', 'developer', 'Developer/client user who can use gateway APIs')
ON CONFLICT (name) DO NOTHING;


-- Seed permissions
INSERT INTO permissions (id, resource, action) VALUES
  ('01972f6a-0002-7000-8000-000000000001', 'services',    'read'),
  ('01972f6a-0002-7000-8000-000000000002', 'services',    'write'),

  ('01972f6a-0002-7000-8000-000000000003', 'routes',      'read'),
  ('01972f6a-0002-7000-8000-000000000004', 'routes',      'write'),

  ('01972f6a-0002-7000-8000-000000000005', 'plugins',     'read'),
  ('01972f6a-0002-7000-8000-000000000006', 'plugins',     'write'),

  ('01972f6a-0002-7000-8000-000000000007', 'rate_limits', 'read'),
  ('01972f6a-0002-7000-8000-000000000008', 'rate_limits', 'write'),

  ('01972f6a-0002-7000-8000-000000000009', 'ip_blacklist','read'),
  ('01972f6a-0002-7000-8000-000000000010', 'ip_blacklist','write'),

  ('01972f6a-0002-7000-8000-000000000011', 'aggregations','read'),
  ('01972f6a-0002-7000-8000-000000000012', 'aggregations','write'),

  ('01972f6a-0002-7000-8000-000000000013', 'api_keys',    'read'),
  ('01972f6a-0002-7000-8000-000000000014', 'api_keys',    'write'),

  ('01972f6a-0002-7000-8000-000000000015', 'users',       'read'),
  ('01972f6a-0002-7000-8000-000000000016', 'users',       'write'),

  ('01972f6a-0002-7000-8000-000000000017', 'roles',       'read'),
  ('01972f6a-0002-7000-8000-000000000018', 'roles',       'write'),

  ('01972f6a-0002-7000-8000-000000000019', 'permissions', 'read'),
  ('01972f6a-0002-7000-8000-000000000020', 'permissions', 'write'),

  ('01972f6a-0002-7000-8000-000000000021', 'logs',        'read'),
  ('01972f6a-0002-7000-8000-000000000022', 'metrics',     'read'),
  ('01972f6a-0002-7000-8000-000000000023', 'audit_logs',  'read'),

  ('01972f6a-0002-7000-8000-000000000024', 'health',      'read'),
  ('01972f6a-0002-7000-8000-000000000025', 'health',      'write'),

  ('01972f6a-0002-7000-8000-000000000026', 'cache',       'reload')
ON CONFLICT (resource, action) DO NOTHING;


-- Admin: có toàn bộ quyền
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r
JOIN permissions p ON TRUE
WHERE r.name = 'admin'
ON CONFLICT (role_id, permission_id) DO NOTHING;


-- Developer: quyền cho các chức năng client/developer
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r
JOIN permissions p
  ON (p.resource, p.action) IN (
    -- Gửi request qua Gateway
    ('services', 'read'),
    ('routes', 'read'),

    -- Xác thực, token/API key
    ('api_keys', 'read'),
    ('api_keys', 'write'),

    -- API Aggregation
    ('aggregations', 'read'),

    -- Rate limiting: chỉ xem giới hạn
    ('rate_limits', 'read'),

    -- Theo dõi request / debug
    ('logs', 'read'),
    ('metrics', 'read'),
    ('audit_logs', 'read'),

    -- Health check
    ('health', 'read')
  )
WHERE r.name = 'developer'
ON CONFLICT (role_id, permission_id) DO NOTHING;

COMMIT;
