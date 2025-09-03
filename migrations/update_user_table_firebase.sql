-- Migration to replace password with firebase_id in users table
-- First, add the new firebase_id column
ALTER TABLE users ADD COLUMN firebase_id VARCHAR(255) UNIQUE;

-- Create index on firebase_id for better performance
CREATE INDEX idx_users_firebase_id ON users(firebase_id);

-- Drop the password column (this will fail if there's data, so run this after ensuring no password data exists)
ALTER TABLE users DROP COLUMN password; 