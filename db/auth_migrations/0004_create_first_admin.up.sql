-- Create first admin user
-- Default password: Admin123!
-- ВАЖНО: Сменить пароль после первого входа!

INSERT INTO users (email, password_hash, role, created_at, updated_at)
VALUES (
    'admin@example.com',
    '$2a$10$RiEfkw/nnkK8eZkzXRcBVON3paKySbNBQEVYJg4QBkFi1ogk/BAES',
    'admin',
    NOW(),
    NOW()
)
ON CONFLICT (email) DO NOTHING;
