-- Migration to add message deletion and editing functionality
-- This adds fields to support WhatsApp-like message deletion and editing

-- Add fields for message deletion
ALTER TABLE messages ADD COLUMN deleted_at TIMESTAMP WITH TIME ZONE;
ALTER TABLE messages ADD COLUMN deleted_for_everyone BOOLEAN DEFAULT FALSE;
ALTER TABLE messages ADD COLUMN deleted_for_users TEXT[]; -- Array of user IDs who have deleted this message

-- Add fields for message editing
ALTER TABLE messages ADD COLUMN edited_at TIMESTAMP WITH TIME ZONE;
ALTER TABLE messages ADD COLUMN original_content TEXT; -- Store original content when message is edited
ALTER TABLE messages ADD COLUMN edit_count INTEGER DEFAULT 0; -- Track how many times message was edited

-- Add index for better performance on deletion queries
CREATE INDEX idx_messages_deleted_at ON messages(deleted_at);
CREATE INDEX idx_messages_deleted_for_everyone ON messages(deleted_for_everyone);
CREATE INDEX idx_messages_edited_at ON messages(edited_at); 