-- Migration from old JSON-based schema to new structured schema
-- This migration handles the transition from the old schema that only had id and data columns
-- to the new schema with individual columns

-- First, add the new columns
ALTER TABLE metadata ADD COLUMN file_path TEXT;
ALTER TABLE metadata ADD COLUMN token TEXT;
ALTER TABLE metadata ADD COLUMN original_name TEXT;
ALTER TABLE metadata ADD COLUMN upload_date DATETIME;
ALTER TABLE metadata ADD COLUMN expires_at DATETIME;
ALTER TABLE metadata ADD COLUMN size INTEGER;
ALTER TABLE metadata ADD COLUMN content_type TEXT;
ALTER TABLE metadata ADD COLUMN one_time_view BOOLEAN DEFAULT FALSE;

-- Migrate JSON data to structured columns using SQLite JSON functions
-- This handles the data migration directly in SQL to avoid application-level locking issues
UPDATE metadata 
SET 
    file_path = json_extract(data, '$.file_path'),
    token = json_extract(data, '$.token'),
    original_name = json_extract(data, '$.original_name'),
    upload_date = json_extract(data, '$.upload_date'),
    expires_at = json_extract(data, '$.expires_at'),
    size = json_extract(data, '$.size'),
    content_type = json_extract(data, '$.content_type'),
    one_time_view = CASE 
        WHEN json_extract(data, '$.one_time_view') = 'true' THEN 1 
        ELSE 0 
    END
WHERE file_path IS NULL AND data IS NOT NULL;