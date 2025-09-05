package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadConfigWithDefaults(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")

	configContent := `port: 8080
min_age_days: 7
max_age_days: 30`

	err := os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	cfg, err := LoadConfig(configPath)
	require.NoError(t, err)

	assert.Equal(t, 8080, cfg.Port)
	assert.Equal(t, 7, cfg.MinAge)
	assert.Equal(t, 30, cfg.MaxAge)

	assert.Equal(t, 512.0, cfg.MaxSize)
	assert.Equal(t, "./uploads", cfg.UploadPath)
	assert.Equal(t, 60, cfg.CheckInterval)
	assert.True(t, cfg.ExpirationManagerEnabled)
	assert.Equal(t, "http://localhost:3002/", cfg.BaseURL)
	assert.Equal(t, "/data/dump.db", cfg.SQLitePath)
	assert.Equal(t, 4, cfg.IdLength)
	assert.Equal(t, 4.0, cfg.ChunkSize)
	assert.Equal(t, 64, cfg.StreamingBufferSize)

	expectedBots := []string{
		"slack", "slackbot", "facebookexternalhit", "twitterbot",
		"discordbot", "whatsapp", "googlebot", "linkedinbot",
		"telegram", "skype", "viber",
	}
	assert.Equal(t, expectedBots, cfg.PreviewBots)
}

func TestLoadConfigWithEmptyPath(t *testing.T) {
	cfg, err := LoadConfig("")

	assert.Error(t, err)
	assert.Nil(t, cfg)
}

func TestLoadConfigWithNonExistentFile(t *testing.T) {
	cfg, err := LoadConfig("/non/existent/path.yaml")

	assert.Error(t, err)
	assert.Nil(t, cfg)
}

func TestLoadConfigWithInvalidYAML(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "invalid.yaml")

	invalidContent := `port: 8080
invalid: yaml: content: [`

	err := os.WriteFile(configPath, []byte(invalidContent), 0644)
	require.NoError(t, err)

	cfg, err := LoadConfig(configPath)
	assert.Error(t, err)
	assert.Nil(t, cfg)
}

func TestLoadConfigWithAllFields(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "full_config.yaml")

	fullConfigContent := `port: 9000
min_age_days: 1
max_age_days: 365
max_size_mib: 1024.0
upload_path: "/custom/uploads"
check_interval_min: 120
expiration_manager_enabled: false
base_url: "https://example.com/"
sqlite_path: "/custom/data.db"
id_length: 8
chunk_size_mib: 8.0
streaming_buffer_size_kb: 128
preview_bots:
  - "custombot"
  - "anotherbot"`

	err := os.WriteFile(configPath, []byte(fullConfigContent), 0644)
	require.NoError(t, err)

	cfg, err := LoadConfig(configPath)
	require.NoError(t, err)

	assert.Equal(t, 9000, cfg.Port)
	assert.Equal(t, 1, cfg.MinAge)
	assert.Equal(t, 365, cfg.MaxAge)
	assert.Equal(t, 1024.0, cfg.MaxSize)
	assert.Equal(t, "/custom/uploads", cfg.UploadPath)
	assert.Equal(t, 120, cfg.CheckInterval)
	assert.False(t, cfg.ExpirationManagerEnabled)
	assert.Equal(t, "https://example.com/", cfg.BaseURL)
	assert.Equal(t, "/custom/data.db", cfg.SQLitePath)
	assert.Equal(t, 8, cfg.IdLength)
	assert.Equal(t, 8.0, cfg.ChunkSize)
	assert.Equal(t, 128, cfg.StreamingBufferSize)
	assert.Equal(t, []string{"custombot", "anotherbot"}, cfg.PreviewBots)
}

func TestMaxSizeToBytes(t *testing.T) {
	cfg := &Config{MaxSize: 512.0}

	result := cfg.MaxSizeToBytes()
	expected := int64(512 * 1024 * 1024)

	assert.Equal(t, expected, result)
}

func TestChunkSizeToBytes(t *testing.T) {
	cfg := &Config{ChunkSize: 4.0}

	result := cfg.ChunkSizeToBytes()
	expected := int64(4 * 1024 * 1024)

	assert.Equal(t, expected, result)
}

func TestStreamingBufferSizeToBytes(t *testing.T) {
	cfg := &Config{StreamingBufferSize: 64}

	result := cfg.StreamingBufferSizeToBytes()
	expected := 64 * 1024

	assert.Equal(t, expected, result)
}

func TestConfigConversionMethods(t *testing.T) {
	cfg := &Config{
		MaxSize:             256.5,
		ChunkSize:           2.5,
		StreamingBufferSize: 32,
	}

	maxBytes := cfg.MaxSizeToBytes()
	expectedMaxBytes := int64(256.5 * 1024 * 1024)
	assert.Equal(t, expectedMaxBytes, maxBytes)

	chunkBytes := cfg.ChunkSizeToBytes()
	expectedChunkBytes := int64(2.5 * 1024 * 1024)
	assert.Equal(t, expectedChunkBytes, chunkBytes)

	bufferBytes := cfg.StreamingBufferSizeToBytes()
	expectedBufferBytes := 32 * 1024
	assert.Equal(t, expectedBufferBytes, bufferBytes)
}
