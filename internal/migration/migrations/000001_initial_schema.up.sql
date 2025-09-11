-- Initial schema migration
-- This migration creates the metadata table with the current schema

CREATE TABLE metadata (
    id TEXT PRIMARY KEY,
    file_path TEXT NOT NULL,
    token TEXT NOT NULL,
    original_name TEXT,
    upload_date DATETIME NOT NULL,
    expires_at DATETIME,
    size INTEGER NOT NULL,
    content_type TEXT,
    one_time_view BOOLEAN DEFAULT FALSE
);

-- Create indexes for better performance
CREATE INDEX IF NOT EXISTS idx_file_path ON metadata(file_path);
CREATE INDEX IF NOT EXISTS idx_original_name ON metadata(original_name);
CREATE INDEX IF NOT EXISTS idx_upload_date ON metadata(upload_date);
CREATE INDEX IF NOT EXISTS idx_expires_at ON metadata(expires_at);
CREATE INDEX IF NOT EXISTS idx_size ON metadata(size);
