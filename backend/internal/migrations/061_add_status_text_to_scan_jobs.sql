-- Add status_text column to scan_jobs for persisting progress messages
-- (e.g. "Preparing data: 5000/17077 records" during backfill phase)
-- Previously this only went to SSE which fails because EventSource can't send auth headers
ALTER TABLE scan_jobs ADD COLUMN status_text TEXT;
