package config

import (
	"encoding/json"
	"os"
	"strconv"
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
	defaultPort     = 3000
)

// Environment variable names
const (
	envPort = "PORT"
)

// Config represents the application configuration
type Config struct {
	Port                     int      `json:"port"`                       // Port to listen on
	MinAge                   int      `json:"min_age_days"`               // Minimum retention in days
	MaxAge                   int      `json:"max_age_days"`               // Maximum retention in days
	MaxSize                  float64  `json:"max_size_mib"`               // Maximum file size in MiB
	UploadPath               string   `json:"upload_path"`                // Path to uploaded files
	CheckInterval            int      `json:"check_interval_min"`         // How often to check for expired files (minutes)
	ExpirationManagerEnabled bool     `json:"expiration_manager_enabled"` // Whether expiration is enabled
	BaseURL                  string   `json:"base_url"`                   // Base URL for links
	SQLitePath               string   `json:"sqlite_path" env:"SQLITE_PATH" envDefault:"./dump.db"`
	IdLength                 int      `json:"id_length"`    // Length of the token
	PreviewBots              []string `json:"preview_bots"` // List of user-agent substrings that indicate preview bots
}

// DefaultConfig provides default config values
var defaultConfig = Config{
	Port:                     defaultPort, // Default port
	MinAge:                   30,          // 30 days
	MaxAge:                   365,         // 1 year
	MaxSize:                  512.0,       // 512 MiB
	UploadPath:               defaultUploadPath,
	CheckInterval:            60, // Check once per hour
	ExpirationManagerEnabled: true,
	BaseURL:                  "http://localhost:3002/",
	SQLitePath:               "/data/dump.db",
	IdLength:                 defaultIDLength,
	PreviewBots: []string{
		"slack",
		"slackbot",
		"facebookexternalhit",
		"twitterbot",
		"discordbot",
		"whatsapp",
		"googlebot",
		"linkedinbot",
		"telegram",
		"skype",
		"viber",
	},
}

// LoadConfig loads a configuration from file and then applies any environment variable overrides
func LoadConfig() (*Config, error) {
	// Start with default config
	config := defaultConfig

	// Try to load from file
	data, err := os.ReadFile(configPath)
	if err == nil {
		if err := json.Unmarshal(data, &config); err != nil {
			return nil, err
		}
	}

	// Apply environment variable overrides
	applyEnvOverrides(&config)

	return &config, nil
}

// applyEnvOverrides applies environment variable overrides to the configuration
func applyEnvOverrides(config *Config) {
	// Override port if set in environment
	if portStr := os.Getenv(envPort); portStr != "" {
		if port, err := strconv.Atoi(portStr); err == nil {
			config.Port = port
		}
	}

	// Add more environment variable overrides here as needed
}

// MaxSizeToBytes converts the MaxSize from MiB to bytes
func (c *Config) MaxSizeToBytes() int64 {
	return int64(c.MaxSize * 1024 * 1024)
}
