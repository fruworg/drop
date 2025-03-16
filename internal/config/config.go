package config

import (
	"encoding/json"
	"os"
)

// Constants for default paths
const (
	defaultUploadPath = "./uploads"
	configPath        = "./config/config.json"
)

// Constants for upload settings
const (
	maxUploadSize   = 100 * 1024 * 1024 // 100MB
	defaultIDLength = 4
)

// Config represents the application configuration
type Config struct {
	MinAge        int     `json:"min_age_days"`       // Minimum retention in days
	MaxAge        int     `json:"max_age_days"`       // Maximum retention in days
	MaxSize       float64 `json:"max_size_mib"`       // Maximum file size in MiB
	UploadPath    string  `json:"upload_path"`        // Path to uploaded files
	CheckInterval int     `json:"check_interval_min"` // How often to check for expired files (minutes)
	Enabled       bool    `json:"enabled"`            // Whether expiration is enabled
	BaseURL       string  `json:"base_url"`           // Base URL for links
	BadgerPath    string  `json:"badger_path"`        // Directory that hold the Badger DB
	MaxUploadSize int64   `json:"max_upload_size"`    // Maximum file size in bytes
	IdLength      int     `json:"id_length"`          // Length of the token
}

// DefaultConfig provides default config values
var defaultConfig = Config{
	MinAge:        30,    // 30 days
	MaxAge:        365,   // 1 year
	MaxSize:       512.0, // 512 MiB
	UploadPath:    defaultUploadPath,
	CheckInterval: 60, // Check once per hour
	Enabled:       true,
	BaseURL:       "http://localhost:8080/", // Change to your domain in production
	BadgerPath:    "./badger",
	MaxUploadSize: maxUploadSize,
	IdLength:      defaultIDLength,
}

// LoadConfig loads a configuration from file
func LoadConfig() (*Config, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return &defaultConfig, nil
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return &config, nil
}
