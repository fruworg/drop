package config

type Config struct {
	MinAge        int     `json:"min_age_days"`       // Minimum retention in days
	MaxAge        int     `json:"max_age_days"`       // Maximum retention in days
	MaxSize       float64 `json:"max_size_mib"`       // Maximum file size in MiB
	UploadPath    string  `json:"upload_path"`        // Path to uploaded files
	CheckInterval int     `json:"check_interval_min"` // How often to check for expired files (minutes)
	Enabled       bool    `json:"enabled"`            // Whether expiration is enabled
	BaseURL       string  `json:"base_url"`
}

// DefaultConfig provides default config values
var DefaultConfig = Config{
	MinAge:        30,    // 30 days
	MaxAge:        365,   // 1 year
	MaxSize:       512.0, // 512 MiB
	UploadPath:    "./uploads",
	CheckInterval: 60, // Check once per hour
	Enabled:       true,
	BaseURL:       "http://localhost:8080/", // Change to your domain in production
}
