-- Organization invitations
CREATE TABLE IF NOT EXISTS org_invitations (
    id TEXT PRIMARY KEY,
    org_id TEXT NOT NULL,
    email TEXT NOT NULL,
    role TEXT DEFAULT 'member',
    token TEXT UNIQUE NOT NULL,
    invited_by TEXT NOT NULL,
    expires_at TEXT NOT NULL,
    accepted_at TEXT,
    created_at TEXT DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (org_id) REFERENCES organizations(id) ON DELETE CASCADE,
    FOREIGN KEY (invited_by) REFERENCES users(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_invitations_org ON org_invitations(org_id);
CREATE INDEX IF NOT EXISTS idx_invitations_email ON org_invitations(email);
CREATE INDEX IF NOT EXISTS idx_invitations_token ON org_invitations(token);
