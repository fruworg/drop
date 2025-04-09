package model

import "time"

// FileMetadata stores information about uploaded files
type FileMetadata struct {
	FilePath     string    `json:"file_path"`
	Token        string    `json:"token"`
	OriginalName string    `json:"original_name,omitempty"`
	UploadDate   time.Time `json:"upload_date"`
	ExpiresAt    time.Time `json:"expires_at,omitempty"`
	Size         int64     `json:"size"`
	ContentType  string    `json:"content_type,omitempty"`
	OneTimeView  bool      `json:"one_time_view,omitempty"`
}

func (m *FileMetadata) ID() string {
	return m.FilePath
}
