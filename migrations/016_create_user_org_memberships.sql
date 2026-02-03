-- User-Organization memberships (many-to-many with roles)
CREATE TABLE IF NOT EXISTS user_org_memberships (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    org_id TEXT NOT NULL,
    role TEXT DEFAULT 'member',
    is_default INTEGER DEFAULT 0,
    joined_at TEXT DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY (org_id) REFERENCES organizations(id) ON DELETE CASCADE,
    UNIQUE(user_id, org_id)
);

-- Roles: owner (full control), admin (manage users), member (regular user)

CREATE INDEX IF NOT EXISTS idx_memberships_user ON user_org_memberships(user_id);
CREATE INDEX IF NOT EXISTS idx_memberships_org ON user_org_memberships(org_id);
CREATE INDEX IF NOT EXISTS idx_memberships_default ON user_org_memberships(user_id, is_default);
