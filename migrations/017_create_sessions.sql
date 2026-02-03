-- Sessions table (for refresh tokens and session management)
CREATE TABLE IF NOT EXISTS sessions (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    org_id TEXT NOT NULL,
    refresh_token_hash TEXT UNIQUE NOT NULL,
    user_agent TEXT,
    ip_address TEXT,
    is_impersonation INTEGER DEFAULT 0,
    impersonated_by TEXT,
    expires_at TEXT NOT NULL,
    created_at TEXT DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY (org_id) REFERENCES organizations(id) ON DELETE CASCADE,
    FOREIGN KEY (impersonated_by) REFERENCES users(id) ON DELETE SET NULL
);

CREATE INDEX IF NOT EXISTS idx_sessions_user ON sessions(user_id);
CREATE INDEX IF NOT EXISTS idx_sessions_org ON sessions(org_id);
CREATE INDEX IF NOT EXISTS idx_sessions_refresh ON sessions(refresh_token_hash);
CREATE INDEX IF NOT EXISTS idx_sessions_expires ON sessions(expires_at);
