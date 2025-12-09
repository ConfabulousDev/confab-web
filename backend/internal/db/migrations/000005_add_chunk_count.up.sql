-- Add chunk_count column to sync_files table
-- NULL means unknown (legacy files), will be backfilled
-- 0 means file entry exists but no chunks uploaded yet
ALTER TABLE sync_files ADD COLUMN chunk_count INTEGER;
