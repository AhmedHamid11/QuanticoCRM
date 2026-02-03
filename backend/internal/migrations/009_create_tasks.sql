-- Migration 009: Create tasks table
-- Tasks support polymorphic linking to any entity (Contact, Account, etc.)

CREATE TABLE IF NOT EXISTS tasks (
    id TEXT PRIMARY KEY,
    org_id TEXT NOT NULL,

    -- Core task fields
    subject TEXT NOT NULL,
    description TEXT DEFAULT '',
    status TEXT DEFAULT 'Open',           -- Open, In Progress, Completed, Deferred, Cancelled
    priority TEXT DEFAULT 'Normal',       -- Low, Normal, High, Urgent
    type TEXT DEFAULT 'Todo',             -- Call, Email, Meeting, Todo

    -- Due date (YYYY-MM-DD format)
    due_date TEXT,

    -- Polymorphic parent link
    parent_id TEXT,                       -- ID of linked record (Contact, Account, etc.)
    parent_type TEXT,                     -- Entity type name: "Contact", "Account", etc.
    parent_name TEXT DEFAULT '',          -- Denormalized display name for quick display

    -- Standard tracking fields
    assigned_user_id TEXT,
    created_by_id TEXT,
    modified_by_id TEXT,
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    modified_at TEXT NOT NULL DEFAULT (datetime('now')),
    deleted INTEGER DEFAULT 0,

    -- Custom fields support
    custom_fields TEXT DEFAULT '{}'
);

-- Primary indexes for multi-tenant queries
CREATE INDEX IF NOT EXISTS idx_tasks_org_id ON tasks(org_id);
CREATE INDEX IF NOT EXISTS idx_tasks_org_deleted ON tasks(org_id, deleted);

-- Polymorphic relationship indexes (critical for related list queries)
CREATE INDEX IF NOT EXISTS idx_tasks_parent ON tasks(parent_type, parent_id, deleted);
CREATE INDEX IF NOT EXISTS idx_tasks_parent_org ON tasks(org_id, parent_type, parent_id, deleted);

-- Status and due date queries (common filters)
CREATE INDEX IF NOT EXISTS idx_tasks_status ON tasks(org_id, status, deleted);
CREATE INDEX IF NOT EXISTS idx_tasks_due_date ON tasks(org_id, due_date, deleted);
CREATE INDEX IF NOT EXISTS idx_tasks_status_due ON tasks(org_id, status, due_date, deleted);

-- Assignment queries
CREATE INDEX IF NOT EXISTS idx_tasks_assigned_user ON tasks(assigned_user_id, deleted);

-- Sorting indexes
CREATE INDEX IF NOT EXISTS idx_tasks_created_at ON tasks(created_at, deleted);
CREATE UNIQUE INDEX IF NOT EXISTS idx_tasks_created_at_id ON tasks(created_at, id);

-- Type-specific queries
CREATE INDEX IF NOT EXISTS idx_tasks_type ON tasks(org_id, type, deleted);

-- Register Task entity definition
INSERT OR IGNORE INTO entity_defs (id, name, label, label_plural, icon, color, is_custom, is_customizable, has_activities, created_at, modified_at)
VALUES ('task', 'Task', 'Task', 'Tasks', 'check-square', '#f59e0b', 0, 1, 0, datetime('now'), datetime('now'));

-- Register Task field definitions
INSERT OR IGNORE INTO field_defs (id, entity_name, name, label, type, is_required, sort_order) VALUES
('task_subject', 'Task', 'subject', 'Subject', 'varchar', 1, 1),
('task_description', 'Task', 'description', 'Description', 'text', 0, 2),
('task_status', 'Task', 'status', 'Status', 'enum', 0, 3),
('task_priority', 'Task', 'priority', 'Priority', 'enum', 0, 4),
('task_type', 'Task', 'type', 'Type', 'enum', 0, 5),
('task_due_date', 'Task', 'dueDate', 'Due Date', 'date', 0, 6),
('task_parent_id', 'Task', 'parentId', 'Related To', 'varchar', 0, 7),
('task_parent_type', 'Task', 'parentType', 'Related Type', 'varchar', 0, 8),
('task_parent_name', 'Task', 'parentName', 'Related Name', 'varchar', 0, 9),
('task_assigned_user_id', 'Task', 'assignedUserId', 'Assigned User', 'varchar', 0, 10);

-- Add enum options for status field
UPDATE field_defs SET options = '["Open", "In Progress", "Completed", "Deferred", "Cancelled"]'
WHERE id = 'task_status';

-- Add enum options for priority field
UPDATE field_defs SET options = '["Low", "Normal", "High", "Urgent"]'
WHERE id = 'task_priority';

-- Add enum options for type field
UPDATE field_defs SET options = '["Call", "Email", "Meeting", "Todo"]'
WHERE id = 'task_type';

-- Register polymorphic relationships (Task -> Contact, Task -> Account)
-- For Contact related list: show Tasks where parent_type='Contact'
INSERT OR IGNORE INTO relationship_defs (id, name, from_entity, to_entity, from_field, to_field, relationship_type)
VALUES ('rel_contact_tasks', 'contactTasks', 'Contact', 'Task', 'id', 'parentId', 'hasMany');

-- For Account related list: show Tasks where parent_type='Account'
INSERT OR IGNORE INTO relationship_defs (id, name, from_entity, to_entity, from_field, to_field, relationship_type)
VALUES ('rel_account_tasks', 'accountTasks', 'Account', 'Task', 'id', 'parentId', 'hasMany');
