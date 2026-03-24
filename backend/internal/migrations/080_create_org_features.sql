-- Feature flags per organization (tenant table)
CREATE TABLE IF NOT EXISTS org_features (
    feature_key TEXT PRIMARY KEY,
    enabled INTEGER NOT NULL DEFAULT 0,
    enabled_at TEXT,
    enabled_by TEXT,
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    modified_at TEXT NOT NULL DEFAULT (datetime('now'))
);
