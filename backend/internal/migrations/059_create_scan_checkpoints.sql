-- Scan checkpoints table for resume-from-failure capability
-- Stores progress state to enable resuming interrupted scans
CREATE TABLE IF NOT EXISTS scan_checkpoints (
    id TEXT PRIMARY KEY,
    job_id TEXT NOT NULL,
    last_offset INTEGER NOT NULL DEFAULT 0,
    last_processed_id TEXT,            -- Last record ID processed (for cursor-based resume)
    retry_count INTEGER NOT NULL DEFAULT 0,
    chunk_size INTEGER NOT NULL DEFAULT 500,
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at TEXT NOT NULL DEFAULT (datetime('now')),
    UNIQUE(job_id)
);
