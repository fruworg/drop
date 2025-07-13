package config

import (
	"github.com/spf13/viper"
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
	PreviewBots              []string `mapstructure:"preview_bots"`
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

	if err := v.ReadInConfig(); err != nil {
		return nil, err
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// MaxSizeToBytes converts the MaxSize from MiB to bytes
func (c *Config) MaxSizeToBytes() int64 {
	return int64(c.MaxSize * 1024 * 1024)
}
