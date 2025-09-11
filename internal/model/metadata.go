package model

import "time"

// FileMetadata stores information about uploaded files and shortened URLs
type FileMetadata struct {
	ResourcePath   string     `json:"resource_path"`
	Token          string     `json:"token"`
	OriginalName   string     `json:"original_name,omitempty"`
	UploadDate     time.Time  `json:"upload_date"`
	ExpiresAt      *time.Time `json:"expires_at,omitempty"`
	Size           int64      `json:"size"`
	ContentType    string     `json:"content_type,omitempty"`
	OneTimeView    bool       `json:"one_time_view,omitempty"`
	OriginalURL    string     `json:"original_url,omitempty"`
	IsURLShortener bool       `json:"is_url_shortener,omitempty"`
	AccessCount    int        `json:"access_count,omitempty"`
	IPAddress      string     `json:"ip_address,omitempty"`
	CreatedAt      time.Time  `json:"created_at,omitempty"`
	UpdatedAt      time.Time  `json:"updated_at,omitempty"`
}

func (m *FileMetadata) ID() string {
	return m.ResourcePath
}

// AdminFileInfo represents file information for admin display
type AdminFileInfo struct {
	FileMetadata
	IsExpired bool `json:"is_expired"`
	DaysLeft  int  `json:"days_left"`
}
