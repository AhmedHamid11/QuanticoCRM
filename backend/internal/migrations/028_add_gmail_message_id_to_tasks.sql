-- Migration 028: Add gmail_message_id to tasks
-- Stores the Gmail message ID for email tasks logged from the Chrome extension

ALTER TABLE tasks ADD COLUMN gmail_message_id TEXT;

-- Index for quick lookup of tasks by Gmail message ID (deduplication)
CREATE INDEX IF NOT EXISTS idx_tasks_gmail_message_id ON tasks(org_id, gmail_message_id) WHERE gmail_message_id IS NOT NULL;

-- Register field definition
INSERT OR IGNORE INTO field_defs (id, entity_name, name, label, type, is_required, sort_order) VALUES
('task_gmail_message_id', 'Task', 'gmailMessageId', 'Gmail Message ID', 'varchar', 0, 11);
