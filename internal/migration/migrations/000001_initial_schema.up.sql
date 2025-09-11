-- Initial schema for drop file upload service
-- Drop existing table if it exists (to handle legacy JSON schema)
DROP TABLE IF EXISTS metadata;

-- Create new table with improved structured schema
CREATE TABLE metadata (
    id TEXT PRIMARY KEY,
    resource_path TEXT NOT NULL,  -- renamed from file_path, more generic
    token TEXT NOT NULL,          -- management token, required
    original_name TEXT,            -- original filename or "URL Shortener"
    upload_date DATETIME NOT NULL, -- when uploaded/created
    expires_at DATETIME,          -- nullable for permanent entries
    size BIGINT NOT NULL DEFAULT 0, -- changed to BIGINT for large files
    content_type TEXT,             -- MIME type
    one_time_view BOOLEAN DEFAULT FALSE,
    original_url TEXT DEFAULT '', -- for URL shortening
    is_url_shortener BOOLEAN DEFAULT FALSE,
    access_count INTEGER DEFAULT 0, -- track how many times accessed
    ip_address TEXT,              -- IP address of uploader for audit
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP, -- explicit creation time
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP  -- last modification time
);

-- Create indexes for better performance
CREATE INDEX idx_metadata_resource_path ON metadata(resource_path);
CREATE INDEX idx_metadata_token ON metadata(token);
CREATE INDEX idx_metadata_upload_date ON metadata(upload_date);
CREATE INDEX idx_metadata_expires_at ON metadata(expires_at);
CREATE INDEX idx_metadata_is_url_shortener ON metadata(is_url_shortener);
CREATE INDEX idx_metadata_access_count ON metadata(access_count);
CREATE INDEX idx_metadata_created_at ON metadata(created_at);
CREATE INDEX idx_metadata_ip_address ON metadata(ip_address);
