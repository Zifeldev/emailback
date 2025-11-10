-- Remove first admin user
DELETE FROM users WHERE email = 'admin@example.com' AND role = 'admin';
