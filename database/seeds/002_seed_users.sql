-- Seed demo users. Passwords are local-development credentials hashed with
-- bcrypt cost 10, matching helper/password.HashPassword.

BEGIN;

INSERT INTO users (id, username, email, password_hash, role_id, is_active)
VALUES
    (
        '40000000-0000-0000-0000-000000000001',
        'admin',
        'admin@example.com',
        '$2a$10$zzpEtUetLf03KJ5J0qXcgepHqDYnZe/WrL6TIf6d7Nx7rX2RWRBjK',
        '01972f6a-0001-7000-8000-000000000001',
        TRUE
    ),
    (
        '40000000-0000-0000-0000-000000000002',
        'developer',
        'developer@example.com',
        '$2a$10$VYzZBc7J7nnyPOm4ghbMjuUqh6kk0gLOnULw9RNrEZhEl9COdBKPu',
        '01972f6a-0001-7000-8000-000000000002',
        TRUE
    )
ON CONFLICT (username) DO NOTHING;

COMMIT;
