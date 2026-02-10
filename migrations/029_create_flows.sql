-- Flow Definitions table
-- Stores screen flow configurations
CREATE TABLE IF NOT EXISTS flow_definitions (
    id TEXT PRIMARY KEY,
    org_id TEXT NOT NULL,
    name TEXT NOT NULL,
    description TEXT,
    version INTEGER DEFAULT 1,
    definition TEXT NOT NULL,  -- JSON blob containing full flow configuration
    is_active INTEGER DEFAULT 1,
    created_by TEXT NOT NULL,
    created_at TEXT DEFAULT CURRENT_TIMESTAMP,
    modified_at TEXT DEFAULT CURRENT_TIMESTAMP,
    modified_by TEXT
);

CREATE INDEX IF NOT EXISTS idx_flow_definitions_org ON flow_definitions(org_id);
CREATE INDEX IF NOT EXISTS idx_flow_definitions_org_active ON flow_definitions(org_id, is_active);

-- Flow Executions table
-- Tracks running and completed flow instances
CREATE TABLE IF NOT EXISTS flow_executions (
    id TEXT PRIMARY KEY,
    org_id TEXT NOT NULL,
    flow_id TEXT NOT NULL,
    user_id TEXT NOT NULL,
    status TEXT NOT NULL,  -- running, paused_at_screen, completed, failed
    current_step TEXT NOT NULL,
    variables TEXT,  -- JSON blob of current variable values
    screen_data TEXT,  -- JSON blob of accumulated screen inputs
    source_entity TEXT,  -- Entity type that triggered the flow (if any)
    source_record_id TEXT,  -- Record ID that triggered the flow (if any)
    error TEXT,  -- Error message if failed
    started_at TEXT DEFAULT CURRENT_TIMESTAMP,
    completed_at TEXT
);

CREATE INDEX IF NOT EXISTS idx_flow_executions_org ON flow_executions(org_id);
CREATE INDEX IF NOT EXISTS idx_flow_executions_flow ON flow_executions(flow_id);
CREATE INDEX IF NOT EXISTS idx_flow_executions_user ON flow_executions(user_id);
CREATE INDEX IF NOT EXISTS idx_flow_executions_status ON flow_executions(org_id, status);
