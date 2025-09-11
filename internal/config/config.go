package config

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
	"github.com/tg123/go-htpasswd"
)

// Config represents the application configuration
// All fields can be set via config file or environment variables.
type Config struct {
	Port                     int      `mapstructure:"port"`
	MinAge                   int      `mapstructure:"min_age_days"`
	MaxAge                   int      `mapstructure:"max_age_days"`
	MaxSize                  float64  `mapstructure:"max_size_mib"`
	UploadPath               string   `mapstructure:"upload_path"`
	CheckInterval            int      `mapstructure:"check_interval_min"`
	ExpirationManagerEnabled bool     `mapstructure:"expiration_manager_enabled"`
	BaseURL                  string   `mapstructure:"base_url"`
	SQLitePath               string   `mapstructure:"sqlite_path"`
	IdLength                 int      `mapstructure:"id_length"`
	ChunkSize                float64  `mapstructure:"chunk_size_mib"`
	PreviewBots              []string `mapstructure:"preview_bots"`
	StreamingBufferSize      int      `mapstructure:"streaming_buffer_size_kb"`
	AdminPanelEnabled        bool     `mapstructure:"admin_panel_enabled"`
	AdminPasswordHash        string   `mapstructure:"admin_password_hash"`
}

// LoadConfig loads configuration from file and environment variables using Viper.
// If configPath is empty, it defaults to "./config/config.yaml".
func LoadConfig(configPath string) (*Config, error) {
	v := viper.New()

	if configPath == "" {
		configPath = "./config/config.yaml"
	}
	v.SetConfigFile(configPath)
	v.SetConfigType("yaml")

	// Set sensible defaults
	v.SetDefault("port", 3000)
	v.SetDefault("min_age_days", 30)
	v.SetDefault("max_age_days", 365)
	v.SetDefault("max_size_mib", 512.0)
	v.SetDefault("upload_path", "./uploads")
	v.SetDefault("check_interval_min", 60)
	v.SetDefault("expiration_manager_enabled", true)
	v.SetDefault("base_url", "http://localhost:3002/")
	v.SetDefault("sqlite_path", "/data/dump.db")
	v.SetDefault("id_length", 4)
	v.SetDefault("chunk_size_mib", 4.0)
	v.SetDefault("preview_bots", []string{
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
	})
	v.SetDefault("streaming_buffer_size_kb", 64)
	v.SetDefault("admin_panel_enabled", true)
	v.SetDefault("admin_password_hash", "")

	if err := v.ReadInConfig(); err != nil {
		return nil, err
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, err
	}

	// Validate admin panel configuration
	if cfg.AdminPanelEnabled && cfg.AdminPasswordHash == "" {
		return nil, fmt.Errorf("admin panel is enabled but admin_password_hash is not set. Please generate a password hash using: htpasswd -n admin yourpassword")
	}

	return &cfg, nil
}

// MaxSizeToBytes converts the MaxSize from MiB to bytes
func (c *Config) MaxSizeToBytes() int64 {
	return int64(c.MaxSize * 1024 * 1024)
}

// ChunkSizeToBytes converts the ChunkSize from MiB to bytes
func (c *Config) ChunkSizeToBytes() int64 {
	return int64(c.ChunkSize * 1024 * 1024)
}

// StreamingBufferSizeToBytes converts the StreamingBufferSize from KB to bytes
func (c *Config) StreamingBufferSizeToBytes() int {
	return c.StreamingBufferSize * 1024
}

// ValidateAdminPassword checks if the provided username and password matches the htpasswd hash
// Supports Apache MD5 ($apr1$) format
// Format: username:hash (e.g., "admin:$apr1$...")
func (c *Config) ValidateAdminPassword(username, password string) bool {
	if c.AdminPasswordHash == "" {
		return false
	}

	parts := strings.Split(c.AdminPasswordHash, ":")
	if len(parts) != 2 {
		return false
	}

	expectedUsername := parts[0]
	hash := parts[1]

	if username != expectedUsername {
		return false
	}

	encodedPasswd, err := htpasswd.AcceptMd5(hash)
	if err != nil {
		return false
	}
	return encodedPasswd.MatchesPassword(password)
}
