-- Seed the admin user if one doesn't exist
-- The password should be changed immediately after first login
-- This is a placeholder migration; the actual admin creation is handled
-- by the application code to ensure the password is properly hashed.

-- Create admin-specific permissions if not already present
INSERT INTO permissions (name)
VALUES ('admin:seed')
ON CONFLICT (name) DO NOTHING;

-- Note: The admin user is created by the application at startup
-- if the ADMIN_EMAIL and ADMIN_PASSWORD environment variables are set.
-- This migration only ensures the permissions exist.