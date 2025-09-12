package db

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/marianozunino/drop/internal/config"
	"github.com/marianozunino/drop/internal/model"
	"github.com/marianozunino/drop/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestDB(t *testing.T) (*DB, func()) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	cfg := &config.Config{
		SQLitePath: dbPath,
	}

	db, err := NewDB(cfg)
	require.NoError(t, err)

	// Run migrations for tests
	err = testutil.RunTestMigrations(dbPath)
	require.NoError(t, err)

	cleanup := func() {
		db.Close()
		os.RemoveAll(tempDir)
	}

	return db, cleanup
}

func TestNewDB(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "new_test.db")

	cfg := &config.Config{
		SQLitePath: dbPath,
	}

	db, err := NewDB(cfg)
	require.NoError(t, err)
	defer db.Close()

	_, err = os.Stat(dbPath)
	assert.NoError(t, err)

	err = db.Ping()
	assert.NoError(t, err)
}

func TestNewDBWithInvalidPath(t *testing.T) {
	cfg := &config.Config{
		SQLitePath: "/invalid/path/that/does/not/exist/test.db",
	}

	db, err := NewDB(cfg)
	assert.Error(t, err)
	assert.Nil(t, db)
}

func TestStoreMetadata(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	now := time.Now()
	expiresAt := now.Add(24 * time.Hour)

	metadata := &model.FileMetadata{
		ResourcePath: "/uploads/test-file.txt",
		Token:        "test-token-123",
		OriginalName: "original-file.txt",
		UploadDate:   now,
		ExpiresAt:    &expiresAt,
		Size:         1024,
		ContentType:  "text/plain",
		OneTimeView:  true,
	}

	err := db.StoreMetadata(metadata)
	assert.NoError(t, err)
}

func TestStoreMetadataWithInvalidJSON(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	metadata := &model.FileMetadata{
		ResourcePath: "/uploads/test-file.txt",
		Token:        "test-token",
		Size:         1024,
	}

	err := db.StoreMetadata(metadata)
	assert.NoError(t, err)
}

func TestGetMetadataByID(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	now := time.Now()
	expiresAt := now.Add(24 * time.Hour)

	originalMetadata := &model.FileMetadata{
		ResourcePath: "/uploads/test-file.txt",
		Token:        "test-token-123",
		OriginalName: "original-file.txt",
		UploadDate:   now,
		ExpiresAt:    &expiresAt,
		Size:         1024,
		ContentType:  "text/plain",
		OneTimeView:  true,
	}

	err := db.StoreMetadata(originalMetadata)
	require.NoError(t, err)

	retrievedMetadata, err := db.GetMetadataByID(originalMetadata.ID())
	require.NoError(t, err)

	// Verify the retrieved metadata matches the original
	assert.Equal(t, originalMetadata.ResourcePath, retrievedMetadata.ResourcePath)
	assert.Equal(t, originalMetadata.Token, retrievedMetadata.Token)
	assert.Equal(t, originalMetadata.OriginalName, retrievedMetadata.OriginalName)
	assert.Equal(t, originalMetadata.UploadDate.Unix(), retrievedMetadata.UploadDate.Unix())
	assert.Equal(t, originalMetadata.ExpiresAt.Unix(), retrievedMetadata.ExpiresAt.Unix())
	assert.Equal(t, originalMetadata.Size, retrievedMetadata.Size)
	assert.Equal(t, originalMetadata.ContentType, retrievedMetadata.ContentType)
	assert.Equal(t, originalMetadata.OneTimeView, retrievedMetadata.OneTimeView)
}

func TestGetMetadataByIDNotFound(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	metadata, err := db.GetMetadataByID("non-existent-id")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no metadata found with ID")
	assert.Empty(t, metadata.ResourcePath)
}

func TestListAllMetadata(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	metadata1 := &model.FileMetadata{
		ResourcePath: "/uploads/file1.txt",
		Token:        "token1",
		Size:         1024,
	}

	metadata2 := &model.FileMetadata{
		ResourcePath: "/uploads/file2.txt",
		Token:        "token2",
		Size:         2048,
	}

	metadata3 := &model.FileMetadata{
		ResourcePath: "/uploads/file3.txt",
		Token:        "token3",
		Size:         4096,
	}

	err := db.StoreMetadata(metadata1)
	require.NoError(t, err)

	err = db.StoreMetadata(metadata2)
	require.NoError(t, err)

	err = db.StoreMetadata(metadata3)
	require.NoError(t, err)

	allMetadata, err := db.ListAllMetadata()
	require.NoError(t, err)

	assert.Len(t, allMetadata, 3)

	filePaths := make(map[string]bool)
	for _, meta := range allMetadata {
		filePaths[meta.ResourcePath] = true
	}

	assert.True(t, filePaths["/uploads/file1.txt"])
	assert.True(t, filePaths["/uploads/file2.txt"])
	assert.True(t, filePaths["/uploads/file3.txt"])
}

func TestListAllMetadataEmpty(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	allMetadata, err := db.ListAllMetadata()
	require.NoError(t, err)

	assert.Empty(t, allMetadata)
}

func TestDeleteMetadata(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	metadata := &model.FileMetadata{
		ResourcePath: "/uploads/file-to-delete.txt",
		Token:        "delete-token",
		Size:         1024,
	}

	err := db.StoreMetadata(metadata)
	require.NoError(t, err)

	retrievedMetadata, err := db.GetMetadataByID(metadata.ID())
	require.NoError(t, err)
	assert.Equal(t, metadata.ResourcePath, retrievedMetadata.ResourcePath)

	err = db.DeleteMetadata(metadata)
	assert.NoError(t, err)

	_, err = db.GetMetadataByID(metadata.ID())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no metadata found with ID")
}

func TestDeleteMetadataNonExistent(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	metadata := &model.FileMetadata{
		ResourcePath: "/uploads/non-existent.txt",
		Token:        "non-existent-token",
		Size:         1024,
	}

	err := db.DeleteMetadata(metadata)
	assert.NoError(t, err)
}

func TestStoreMetadataReplace(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	originalMetadata := &model.FileMetadata{
		ResourcePath: "/uploads/same-file.txt",
		Token:        "original-token",
		Size:         1024,
	}

	err := db.StoreMetadata(originalMetadata)
	require.NoError(t, err)

	updatedMetadata := &model.FileMetadata{
		ResourcePath: "/uploads/same-file.txt",
		Token:        "updated-token",
		Size:         2048,
	}

	err = db.StoreMetadata(updatedMetadata)
	require.NoError(t, err)

	retrievedMetadata, err := db.GetMetadataByID(originalMetadata.ID())
	require.NoError(t, err)

	assert.Equal(t, updatedMetadata.Token, retrievedMetadata.Token)
	assert.Equal(t, updatedMetadata.Size, retrievedMetadata.Size)
	assert.NotEqual(t, originalMetadata.Token, retrievedMetadata.Token)
}

func TestClose(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	err := db.Ping()
	assert.NoError(t, err)

	err = db.Close()
	assert.NoError(t, err)

	err = db.Ping()
	assert.Error(t, err)
}

func TestConcurrentOperations(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	done := make(chan bool, 10)

	for i := 0; i < 10; i++ {
		go func(index int) {
			metadata := &model.FileMetadata{
				ResourcePath: filepath.Join("/uploads", "concurrent", "file"+string(rune(index))+".txt"),
				Token:        "token" + string(rune(index)),
				Size:         int64(1024 * index),
			}

			err := db.StoreMetadata(metadata)
			assert.NoError(t, err)

			retrievedMetadata, err := db.GetMetadataByID(metadata.ID())
			assert.NoError(t, err)
			assert.Equal(t, metadata.ResourcePath, retrievedMetadata.ResourcePath)

			done <- true
		}(i)
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	allMetadata, err := db.ListAllMetadata()
	require.NoError(t, err)
	assert.Len(t, allMetadata, 10)
}
