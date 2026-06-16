-- Seed demo users.
-- Password hashes below are placeholders. Replace with real bcrypt/argon2 hashes from the backend later.

BEGIN;

INSERT INTO users (id, username, email, password_hash, role_id, is_active)
VALUES
    (
        '40000000-0000-0000-0000-000000000001',
        'admin',
        'admin@example.com',
        '$2a$10$replace_with_real_bcrypt_hash',
        '11111111-1111-1111-1111-111111111111',
        TRUE
    ),
    (
        '40000000-0000-0000-0000-000000000002',
        'developer',
        'developer@example.com',
        '$2a$10$replace_with_real_bcrypt_hash',
        '22222222-2222-2222-2222-222222222222',
        TRUE
    )
ON CONFLICT (username) DO NOTHING;

COMMIT;
