-- Migration to rename encrypted_content to content in messages table
-- This removes the encryption naming since we're no longer using encryption

ALTER TABLE messages RENAME COLUMN encrypted_content TO content; 