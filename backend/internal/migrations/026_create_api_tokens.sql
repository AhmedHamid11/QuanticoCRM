-- API tokens for programmatic access
-- Tokens are scoped to a specific org and can have configurable permissions

CREATE TABLE api_tokens (
    id TEXT PRIMARY KEY,
    org_id TEXT NOT NULL,
    created_by TEXT NOT NULL,
    name TEXT NOT NULL,
    token_hash TEXT UNIQUE NOT NULL,
    token_prefix TEXT NOT NULL,  -- First 8 chars for identification (e.g., "fcr_abc1...")
    scopes TEXT NOT NULL DEFAULT '["read", "write"]',  -- JSON array of allowed scopes
    last_used_at TEXT,
    expires_at TEXT,  -- NULL means no expiration
    is_active INTEGER DEFAULT 1,
    created_at TEXT DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (org_id) REFERENCES organizations(id) ON DELETE CASCADE,
    FOREIGN KEY (created_by) REFERENCES users(id) ON DELETE CASCADE
);

-- Index for looking up tokens by hash (used during auth)
CREATE INDEX idx_api_tokens_hash ON api_tokens(token_hash);

-- Index for listing tokens by org
CREATE INDEX idx_api_tokens_org ON api_tokens(org_id, is_active);

-- Index for cleanup of expired tokens
CREATE INDEX idx_api_tokens_expires ON api_tokens(expires_at) WHERE expires_at IS NOT NULL;
