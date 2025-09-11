-- Rollback for JSON schema migration
-- This will remove the new columns added in the up migration

ALTER TABLE metadata DROP COLUMN one_time_view;
ALTER TABLE metadata DROP COLUMN content_type;
ALTER TABLE metadata DROP COLUMN size;
ALTER TABLE metadata DROP COLUMN expires_at;
ALTER TABLE metadata DROP COLUMN upload_date;
ALTER TABLE metadata DROP COLUMN original_name;
ALTER TABLE metadata DROP COLUMN token;
ALTER TABLE metadata DROP COLUMN file_path;
