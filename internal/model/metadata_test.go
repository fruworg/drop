package model

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestFileMetadataID(t *testing.T) {
	metadata := &FileMetadata{
		FilePath: "/uploads/test-file.txt",
		Token:    "abc123",
	}

	id := metadata.ID()
	assert.Equal(t, "/uploads/test-file.txt", id)
}

func TestFileMetadataJSONSerialization(t *testing.T) {
	now := time.Now()
	expiresAt := now.Add(24 * time.Hour)

	metadata := FileMetadata{
		FilePath:     "/uploads/test-file.txt",
		Token:        "test-token-123",
		OriginalName: "original-file.txt",
		UploadDate:   now,
		ExpiresAt:    expiresAt,
		Size:         1024,
		ContentType:  "text/plain",
		OneTimeView:  true,
	}

	jsonData, err := json.Marshal(metadata)
	assert.NoError(t, err)
	assert.NotEmpty(t, jsonData)

	var unmarshaled FileMetadata
	err = json.Unmarshal(jsonData, &unmarshaled)
	assert.NoError(t, err)

	assert.Equal(t, metadata.FilePath, unmarshaled.FilePath)
	assert.Equal(t, metadata.Token, unmarshaled.Token)
	assert.Equal(t, metadata.OriginalName, unmarshaled.OriginalName)
	assert.Equal(t, metadata.UploadDate.Unix(), unmarshaled.UploadDate.Unix())
	assert.Equal(t, metadata.ExpiresAt.Unix(), unmarshaled.ExpiresAt.Unix())
	assert.Equal(t, metadata.Size, unmarshaled.Size)
	assert.Equal(t, metadata.ContentType, unmarshaled.ContentType)
	assert.Equal(t, metadata.OneTimeView, unmarshaled.OneTimeView)
}

func TestFileMetadataWithMinimalFields(t *testing.T) {
	metadata := FileMetadata{
		FilePath: "/uploads/minimal.txt",
		Token:    "minimal-token",
		Size:     512,
	}

	id := metadata.ID()
	assert.Equal(t, "/uploads/minimal.txt", id)

	jsonData, err := json.Marshal(metadata)
	assert.NoError(t, err)
	assert.NotEmpty(t, jsonData)

	var unmarshaled FileMetadata
	err = json.Unmarshal(jsonData, &unmarshaled)
	assert.NoError(t, err)

	assert.Equal(t, metadata.FilePath, unmarshaled.FilePath)
	assert.Equal(t, metadata.Token, unmarshaled.Token)
	assert.Equal(t, metadata.Size, unmarshaled.Size)
	assert.Empty(t, unmarshaled.OriginalName)
	assert.Empty(t, unmarshaled.ContentType)
	assert.False(t, unmarshaled.OneTimeView)
	assert.True(t, unmarshaled.UploadDate.IsZero())
	assert.True(t, unmarshaled.ExpiresAt.IsZero())
}

func TestFileMetadataWithEmptyFields(t *testing.T) {
	metadata := FileMetadata{
		FilePath:     "/uploads/empty.txt",
		Token:        "empty-token",
		OriginalName: "",
		Size:         0,
		ContentType:  "",
		OneTimeView:  false,
	}

	id := metadata.ID()
	assert.Equal(t, "/uploads/empty.txt", id)

	jsonData, err := json.Marshal(metadata)
	assert.NoError(t, err)
	assert.NotEmpty(t, jsonData)

	var unmarshaled FileMetadata
	err = json.Unmarshal(jsonData, &unmarshaled)
	assert.NoError(t, err)

	assert.Equal(t, metadata.FilePath, unmarshaled.FilePath)
	assert.Equal(t, metadata.Token, unmarshaled.Token)
	assert.Empty(t, unmarshaled.OriginalName)
	assert.Equal(t, int64(0), unmarshaled.Size)
	assert.Empty(t, unmarshaled.ContentType)
	assert.False(t, unmarshaled.OneTimeView)
}

func TestFileMetadataTimeFields(t *testing.T) {
	uploadTime := time.Date(2023, 6, 15, 14, 30, 45, 0, time.UTC)
	expireTime := time.Date(2023, 7, 15, 14, 30, 45, 0, time.UTC)

	metadata := FileMetadata{
		FilePath:   "/uploads/time-test.txt",
		Token:      "time-token",
		UploadDate: uploadTime,
		ExpiresAt:  expireTime,
		Size:       2048,
	}

	jsonData, err := json.Marshal(metadata)
	assert.NoError(t, err)

	var unmarshaled FileMetadata
	err = json.Unmarshal(jsonData, &unmarshaled)
	assert.NoError(t, err)

	// Verify time fields are preserved correctly
	assert.Equal(t, uploadTime.Unix(), unmarshaled.UploadDate.Unix())
	assert.Equal(t, expireTime.Unix(), unmarshaled.ExpiresAt.Unix())
	assert.Equal(t, uploadTime.Location(), unmarshaled.UploadDate.Location())
	assert.Equal(t, expireTime.Location(), unmarshaled.ExpiresAt.Location())
}
