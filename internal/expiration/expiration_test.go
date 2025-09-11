package expiration

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/marianozunino/drop/internal/config"
	"github.com/marianozunino/drop/internal/db"
	"github.com/marianozunino/drop/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestExpirationManager(t *testing.T) (*ExpirationManager, *db.DB, func()) {
	tempDir, err := os.MkdirTemp("", "expiration-test")
	require.NoError(t, err)

	testDBPath := filepath.Join(tempDir, "test.db")

	cfg := &config.Config{
		UploadPath:               tempDir,
		MinAge:                   1,
		MaxAge:                   30,
		MaxSize:                  250.0,
		CheckInterval:            60,
		ExpirationManagerEnabled: true,
		BaseURL:                  "http://localhost:8080/",
		SQLitePath:               testDBPath,
	}

	testDB, err := db.NewDB(cfg)
	require.NoError(t, err)

	manager, err := NewExpirationManager(cfg, testDB)
	require.NoError(t, err)

	cleanup := func() {
		testDB.Close()
		os.RemoveAll(tempDir)
	}

	return manager, testDB, cleanup
}

func createTestFileWithMetadata(t *testing.T, tempDir string, db *db.DB, filename string, content string, uploadTime time.Time, expiresAt time.Time) string {
	filePath := filepath.Join(tempDir, filename)
	err := os.WriteFile(filePath, []byte(content), 0644)
	require.NoError(t, err)

	// Set the file modification time
	err = os.Chtimes(filePath, uploadTime, uploadTime)
	require.NoError(t, err)

	meta := model.FileMetadata{
		ResourcePath: filePath,
		Token:        "test-token-" + filename,
		OriginalName: "original-" + filename,
		UploadDate:   uploadTime,
		ExpiresAt:    &expiresAt,
		Size:         int64(len(content)),
		ContentType:  "text/plain",
		OneTimeView:  false,
	}

	err = db.StoreMetadata(&meta)
	require.NoError(t, err)

	return filePath
}

// Expiration Manager Lifecycle Tests

func TestNewExpirationManager(t *testing.T) {
	manager, db, cleanup := setupTestExpirationManager(t)
	defer cleanup()

	assert.NotNil(t, manager)
	assert.NotNil(t, manager.Config)
	assert.NotNil(t, manager.db)
	assert.NotNil(t, manager.stopChan)
	assert.Equal(t, 1, manager.Config.MinAge)
	assert.Equal(t, 30, manager.Config.MaxAge)
	assert.Equal(t, 250.0, manager.Config.MaxSize)
	assert.True(t, manager.Config.ExpirationManagerEnabled)

	db.Close()
}

func TestNewExpirationManagerWithNilConfig(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "expiration-test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	testDBPath := filepath.Join(tempDir, "test.db")

	cfg := &config.Config{
		UploadPath:               tempDir,
		SQLitePath:               testDBPath,
		ExpirationManagerEnabled: true,
	}

	testDB, err := db.NewDB(cfg)
	require.NoError(t, err)
	defer testDB.Close()

	manager, err := NewExpirationManager(cfg, testDB)
	require.NoError(t, err)
	assert.NotNil(t, manager)
}

func TestExpirationManagerStartStop(t *testing.T) {
	manager, db, cleanup := setupTestExpirationManager(t)
	defer cleanup()

	// Test Start
	manager.Start()

	// Give it a moment to start
	time.Sleep(100 * time.Millisecond)

	// Test Stop
	manager.Stop()

	// Give it a moment to stop
	time.Sleep(100 * time.Millisecond)

	db.Close()
}

// Expiration Logic Tests

func TestCheckMetadataExpiration_WithExpiresAt(t *testing.T) {
	manager, _, cleanup := setupTestExpirationManager(t)
	defer cleanup()

	now := time.Now()
	pastTime := now.Add(-2 * time.Hour)
	futureTime := now.Add(2 * time.Hour)

	// Test expired file
	meta := model.FileMetadata{
		ResourcePath: "/test/expired.txt",
		Token:        "expired-token",
		ExpiresAt:    &pastTime,
		Size:         1024,
	}

	expired, err := manager.CheckMetadataExpiration(meta)
	require.NoError(t, err)
	assert.True(t, expired)

	// Test non-expired file
	meta.ExpiresAt = &futureTime
	expired, err = manager.CheckMetadataExpiration(meta)
	require.NoError(t, err)
	assert.False(t, expired)
}

func TestCheckMetadataExpiration_WithUploadDate(t *testing.T) {
	manager, _, cleanup := setupTestExpirationManager(t)
	defer cleanup()

	now := time.Now()

	// Test expired file (uploaded 2 days ago, should expire in 1 day for small files)
	// Small files get ~29 days retention, so 2 days ago should still be active
	pastUploadTime := now.Add(-2 * 24 * time.Hour)
	meta := model.FileMetadata{
		ResourcePath: "/test/expired.txt",
		Token:        "expired-token",
		UploadDate:   pastUploadTime,
		Size:         1024, // Small file gets longer retention
	}

	expired, err := manager.CheckMetadataExpiration(meta)
	require.NoError(t, err)
	assert.False(t, expired) // Should NOT be expired yet

	// Test actually expired file (uploaded 30+ days ago)
	veryOldUploadTime := now.Add(-35 * 24 * time.Hour)
	meta.UploadDate = veryOldUploadTime
	expired, err = manager.CheckMetadataExpiration(meta)
	require.NoError(t, err)
	assert.True(t, expired) // Should be expired

	// Test non-expired file (uploaded recently)
	recentUploadTime := now.Add(-1 * time.Hour)
	meta.UploadDate = recentUploadTime
	expired, err = manager.CheckMetadataExpiration(meta)
	require.NoError(t, err)
	assert.False(t, expired)
}

func TestCheckMetadataExpiration_NoDates(t *testing.T) {
	manager, _, cleanup := setupTestExpirationManager(t)
	defer cleanup()

	meta := model.FileMetadata{
		ResourcePath: "/test/no-dates.txt",
		Token:        "no-dates-token",
		Size:         1024,
	}

	expired, err := manager.CheckMetadataExpiration(meta)
	require.NoError(t, err)
	assert.False(t, expired)
}

func TestCleanupExpiredFiles_WithMetadata(t *testing.T) {
	manager, db, cleanup := setupTestExpirationManager(t)
	defer cleanup()

	now := time.Now()

	// Create expired file
	expiredTime := now.Add(-2 * 24 * time.Hour)
	expiredFile := createTestFileWithMetadata(t, manager.Config.UploadPath, db, "expired.txt", "expired content", expiredTime, expiredTime)

	// Create non-expired file
	recentTime := now.Add(-1 * time.Hour)
	futureTime := now.Add(24 * time.Hour)
	activeFile := createTestFileWithMetadata(t, manager.Config.UploadPath, db, "active.txt", "active content", recentTime, futureTime)

	// Run cleanup
	manager.cleanupExpiredFiles()

	// Check that expired file was removed
	_, err := os.Stat(expiredFile)
	assert.True(t, os.IsNotExist(err))

	// Check that active file still exists
	_, err = os.Stat(activeFile)
	assert.NoError(t, err)

	// Check that metadata was cleaned up
	allMetadata, err := db.ListAllMetadata()
	require.NoError(t, err)
	assert.Len(t, allMetadata, 1)
	assert.Equal(t, activeFile, allMetadata[0].ResourcePath)
}

func TestCleanupExpiredFiles_WithoutMetadata(t *testing.T) {
	manager, _, cleanup := setupTestExpirationManager(t)
	defer cleanup()

	now := time.Now()

	// Create file without metadata (old file - should be removed)
	oldTime := now.Add(-2 * 24 * time.Hour)
	oldFile := filepath.Join(manager.Config.UploadPath, "old-file.txt")
	err := os.WriteFile(oldFile, []byte("old content"), 0644)
	require.NoError(t, err)

	err = os.Chtimes(oldFile, oldTime, oldTime)
	require.NoError(t, err)

	// Create recent file without metadata (should be removed too since no metadata = expired)
	recentTime := now.Add(-1 * time.Hour)
	recentFile := filepath.Join(manager.Config.UploadPath, "recent-file.txt")
	err = os.WriteFile(recentFile, []byte("recent content"), 0644)
	require.NoError(t, err)

	err = os.Chtimes(recentFile, recentTime, recentTime)
	require.NoError(t, err)

	// Run cleanup
	manager.cleanupExpiredFiles()

	// Check that both files were removed (files without metadata are treated as expired)
	_, err = os.Stat(oldFile)
	assert.True(t, os.IsNotExist(err))

	_, err = os.Stat(recentFile)
	assert.True(t, os.IsNotExist(err))
}

func TestCleanupExpiredFiles_Disabled(t *testing.T) {
	manager, db, cleanup := setupTestExpirationManager(t)
	defer cleanup()

	// Disable expiration manager
	manager.Config.ExpirationManagerEnabled = false

	now := time.Now()
	expiredTime := now.Add(-2 * 24 * time.Hour)
	expiredFile := createTestFileWithMetadata(t, manager.Config.UploadPath, db, "expired.txt", "expired content", expiredTime, expiredTime)

	// Run cleanup
	manager.cleanupExpiredFiles()

	// Check that file was NOT removed
	_, err := os.Stat(expiredFile)
	assert.NoError(t, err)
}

func TestCleanupExpiredFiles_ErrorHandling(t *testing.T) {
	manager, _, cleanup := setupTestExpirationManager(t)
	defer cleanup()

	// Create a file with invalid metadata (simulate error)
	invalidFile := filepath.Join(manager.Config.UploadPath, "invalid.txt")
	err := os.WriteFile(invalidFile, []byte("invalid content"), 0644)
	require.NoError(t, err)

	// Run cleanup - should handle the error gracefully
	manager.cleanupExpiredFiles()

	// The file should be removed due to metadata error
	_, err = os.Stat(invalidFile)
	assert.True(t, os.IsNotExist(err))
}

// Integration Tests

func TestExpirationManagerIntegration(t *testing.T) {
	manager, db, cleanup := setupTestExpirationManager(t)
	defer cleanup()

	now := time.Now()

	// Create multiple files with different expiration times
	files := []struct {
		name         string
		content      string
		uploadTime   time.Time
		expiresAt    time.Time
		shouldExpire bool
	}{
		{
			name:         "expired1.txt",
			content:      "expired content 1",
			uploadTime:   now.Add(-3 * 24 * time.Hour),
			expiresAt:    now.Add(-1 * 24 * time.Hour),
			shouldExpire: true,
		},
		{
			name:         "expired2.txt",
			content:      "expired content 2",
			uploadTime:   now.Add(-35 * 24 * time.Hour), // Very old, should expire
			expiresAt:    time.Time{},                   // No explicit expiration, use upload date
			shouldExpire: true,
		},
		{
			name:         "active1.txt",
			content:      "active content 1",
			uploadTime:   now.Add(-1 * time.Hour),
			expiresAt:    now.Add(24 * time.Hour),
			shouldExpire: false,
		},
		{
			name:         "active2.txt",
			content:      "active content 2",
			uploadTime:   now.Add(-1 * time.Hour),
			expiresAt:    time.Time{}, // No explicit expiration, use upload date
			shouldExpire: false,
		},
	}

	var createdFiles []string
	for _, file := range files {
		filePath := createTestFileWithMetadata(t, manager.Config.UploadPath, db, file.name, file.content, file.uploadTime, file.expiresAt)
		createdFiles = append(createdFiles, filePath)
	}

	// Start the expiration manager
	manager.Start()
	time.Sleep(100 * time.Millisecond)

	// Run cleanup
	manager.cleanupExpiredFiles()

	// Stop the manager
	manager.Stop()

	// Check results
	for i, file := range files {
		_, err := os.Stat(createdFiles[i])
		if file.shouldExpire {
			assert.True(t, os.IsNotExist(err), "File %s should have been expired", file.name)
		} else {
			assert.NoError(t, err, "File %s should still exist", file.name)
		}
	}

	// Check metadata cleanup
	allMetadata, err := db.ListAllMetadata()
	require.NoError(t, err)

	expectedActiveFiles := 0
	for _, file := range files {
		if !file.shouldExpire {
			expectedActiveFiles++
		}
	}
	assert.Len(t, allMetadata, expectedActiveFiles)
}

func TestCalculateRetention(t *testing.T) {
	cfg := &config.Config{
		MinAge:                   1,
		MaxAge:                   30,
		MaxSize:                  250.0,
		UploadPath:               "./uploads",
		CheckInterval:            60,
		ExpirationManagerEnabled: true,
		BaseURL:                  "https://domain.com/",
	}

	manager := &ExpirationManager{Config: cfg}

	testCases := []struct {
		name            string
		fileSizeBytes   float64
		expectedDays    float64
		expectedFormula string
	}{
		{
			name:            "Small file (1 MiB)",
			fileSizeBytes:   1 * 1024 * 1024,
			expectedDays:    29.0,
			expectedFormula: "min_age + (min_age - max_age) * pow((file_size / max_size - 1), 3)",
		},
		{
			name:            "Medium file (100 MiB)",
			fileSizeBytes:   100 * 1024 * 1024,
			expectedDays:    7.0,
			expectedFormula: "min_age + (min_age - max_age) * pow((file_size / max_size - 1), 3)",
		},
		{
			name:            "Threshold file (250 MiB - exactly at MaxSize)",
			fileSizeBytes:   250 * 1024 * 1024,
			expectedDays:    1.0,
			expectedFormula: "min_age + (min_age - max_age) * pow((file_size / max_size - 1), 3)",
		},
		{
			name:            "Large file (500 MiB - 2x threshold)",
			fileSizeBytes:   500 * 1024 * 1024,
			expectedDays:    1.0,
			expectedFormula: "min_age + (min_age - max_age) * pow((file_size / max_size - 1), 3), clamped to min_age",
		},
		{
			name:            "Very large file (1000 MiB - 4x threshold)",
			fileSizeBytes:   1000 * 1024 * 1024,
			expectedDays:    1.0,
			expectedFormula: "min_age + (min_age - max_age) * pow((file_size / max_size - 1), 3), clamped to min_age",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			retention := manager.calculateRetention(tc.fileSizeBytes)
			actualDays := retention.Hours() / 24

			t.Logf("File size: %.2f MiB", tc.fileSizeBytes/(1024*1024))
			t.Logf("Actual retention: %.2f days", actualDays)
			t.Logf("Expected retention: %.2f days", tc.expectedDays)
			t.Logf("Formula used: %s", tc.expectedFormula)

			assert.InDelta(t, tc.expectedDays, actualDays, 0.01, "Retention calculation is incorrect")
		})
	}
}

func TestRetentionDecreaseWithSize(t *testing.T) {
	cfg := &config.Config{
		MinAge:                   1,
		MaxAge:                   30,
		MaxSize:                  250.0,
		UploadPath:               "./uploads",
		CheckInterval:            60,
		ExpirationManagerEnabled: true,
		BaseURL:                  "https://domain.com/",
	}

	manager := &ExpirationManager{Config: cfg}

	testSizes := []float64{
		1 * 1024 * 1024,
		100 * 1024 * 1024,
		250 * 1024 * 1024,
		300 * 1024 * 1024,
		500 * 1024 * 1024,
		1000 * 1024 * 1024,
		2000 * 1024 * 1024,
	}

	t.Log("Retention days by file size (demonstrating decrease with size):")
	t.Log("----------------------------------------------------------")
	t.Log("File Size (MiB) | Retention (days)")
	t.Log("----------------------------------------------------------")

	previousDays := -1.0
	for _, size := range testSizes {
		retention := manager.calculateRetention(size)
		days := retention.Hours() / 24
		sizeMiB := size / (1024 * 1024)

		t.Logf("%.2f | %.2f", sizeMiB, days)

		if previousDays > 0 {
			assert.LessOrEqual(t, days, previousDays,
				"Retention should decrease (or stay at MinAge) as file size increases")
		}

		previousDays = days
	}
}

func TestCorrectFormulaImplementation(t *testing.T) {
	cfg := &config.Config{
		MinAge:                   1,
		MaxAge:                   30,
		MaxSize:                  250.0,
		UploadPath:               "./uploads",
		CheckInterval:            60,
		ExpirationManagerEnabled: true,
		BaseURL:                  "https://domain.com/",
	}

	manager := &ExpirationManager{Config: cfg}

	largeFileSize := 500 * 1024 * 1024
	retention := manager.calculateRetention(float64(largeFileSize))
	actualDays := retention.Hours() / 24

	t.Logf("For a 500 MiB file (2x threshold):")
	t.Logf("  Expected: 1 day (MinAge) after clamping")
	t.Logf("  Actual: %.2f days", actualDays)

	assert.InDelta(t, 1.0, actualDays, 0.01,
		"A file twice the threshold size should have retention clamped to MinAge (1 day)")
}

func TestGetExpirationDate(t *testing.T) {
	cfg := &config.Config{
		MinAge:                   1,
		MaxAge:                   30,
		MaxSize:                  250.0,
		UploadPath:               "./uploads",
		CheckInterval:            60,
		ExpirationManagerEnabled: true,
		BaseURL:                  "https://domain.com/",
	}

	manager := &ExpirationManager{Config: cfg}

	now := time.Now()

	smallFileSize := int64(1 * 1024 * 1024)
	smallFileExpiration := manager.GetExpirationDate(smallFileSize)

	smallFileDiff := smallFileExpiration.Sub(now)
	expectedSmallDiff := time.Duration(29) * 24 * time.Hour

	t.Logf("1 MiB file expiration: %v (%.2f days from now)",
		smallFileExpiration, smallFileDiff.Hours()/24)

	assert.InDelta(t, expectedSmallDiff.Hours(), smallFileDiff.Hours(), 0.1,
		"Small file expiration should be 29 days from now")

	largeFileSize := int64(500 * 1024 * 1024)
	largeFileExpiration := manager.GetExpirationDate(largeFileSize)

	largeFileDiff := largeFileExpiration.Sub(now)
	t.Logf("500 MiB file expiration: %v (%.2f days from now)",
		largeFileExpiration, largeFileDiff.Hours()/24)

	assert.Less(t, largeFileDiff.Hours(), smallFileDiff.Hours(),
		"Retention for larger files should be less than smaller files")

	expectedLargeDiff := time.Duration(cfg.MinAge) * 24 * time.Hour
	assert.InDelta(t, expectedLargeDiff.Hours(), largeFileDiff.Hours(), 0.1,
		"Large file expiration should be MinAge days from now")
}

func TestExtremeFileSizes(t *testing.T) {
	cfg := &config.Config{
		MinAge:                   1,
		MaxAge:                   30,
		MaxSize:                  250.0,
		UploadPath:               "./uploads",
		CheckInterval:            60,
		ExpirationManagerEnabled: true,
		BaseURL:                  "https://domain.com/",
	}

	manager := &ExpirationManager{Config: cfg}

	testSizes := []float64{
		10000 * 1024 * 1024,
		100000 * 1024 * 1024,
		1000000 * 1024 * 1024,
	}

	t.Log("Testing extreme file sizes (should all be clamped to MinAge):")
	t.Log("---------------------------------------------------------")
	for _, size := range testSizes {
		retention := manager.calculateRetention(size)
		days := retention.Hours() / 24
		sizeMiB := size / (1024 * 1024)

		t.Logf("%.2f MiB (%.2f GB): %.2f days",
			sizeMiB, sizeMiB/1024, days)

		assert.InDelta(t, float64(cfg.MinAge), days, 0.01,
			"Extremely large files should be clamped to MinAge")
	}
}
