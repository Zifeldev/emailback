-- Add role column to users table
ALTER TABLE users ADD COLUMN role VARCHAR(20) NOT NULL DEFAULT 'user';

-- Create index on role for faster queries
CREATE INDEX idx_users_role ON users(role);

-- Update existing users to have user role (already default)
-- You can manually update specific users to admin:
-- UPDATE users SET role = 'admin' WHERE username = 'admin@example.com';
