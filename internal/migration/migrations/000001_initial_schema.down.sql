-- Rollback for initial schema migration
-- This will drop the metadata table and all its indexes

DROP INDEX IF EXISTS idx_size;
DROP INDEX IF EXISTS idx_expires_at;
DROP INDEX IF EXISTS idx_upload_date;
DROP INDEX IF EXISTS idx_original_name;
DROP INDEX IF EXISTS idx_file_path;

DROP TABLE IF EXISTS metadata;
