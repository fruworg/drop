package expiration

import (
	"testing"
	"time"

	"github.com/marianozunino/drop/internal/config"
	"github.com/stretchr/testify/assert"
)

func TestCalculateRetention(t *testing.T) {
	// Setup a test configuration
	cfg := config.Config{
		MinAge:        1,     // 1 day minimum retention
		MaxAge:        30,    // 30 days maximum retention
		MaxSize:       250.0, // 250 MiB threshold
		UploadPath:    "./uploads",
		CheckInterval: 60,
		Enabled:       true,
		BaseURL:       "https://domain.com/",
	}

	manager := &ExpirationManager{Config: cfg}

	// Test cases
	testCases := []struct {
		name            string
		fileSizeBytes   float64
		expectedDays    float64
		expectedFormula string
	}{
		{
			name:            "Small file (1 MiB)",
			fileSizeBytes:   1 * 1024 * 1024, // 1 MiB
			expectedDays:    30,              // Should get MaxAge (30 days)
			expectedFormula: "For files <= MaxSize: MaxAge",
		},
		{
			name:            "Medium file (100 MiB)",
			fileSizeBytes:   100 * 1024 * 1024, // 100 MiB
			expectedDays:    30,                // Should get MaxAge (30 days)
			expectedFormula: "For files <= MaxSize: MaxAge",
		},
		{
			name:            "Threshold file (250 MiB - exactly at MaxSize)",
			fileSizeBytes:   250 * 1024 * 1024, // 250 MiB
			expectedDays:    30,                // Should get MaxAge (30 days)
			expectedFormula: "For files <= MaxSize: MaxAge",
		},
		{
			name:            "Large file (500 MiB - 2x threshold)",
			fileSizeBytes:   500 * 1024 * 1024, // 500 MiB
			expectedDays:    1.0,               // Should be clamped to MinAge (1 day)
			expectedFormula: "MinAge + (MinAge - MaxAge) * ((fileSize / MaxSize) - 1)^3, clamped to MinAge",
		},
		{
			name:            "Very large file (1000 MiB - 4x threshold)",
			fileSizeBytes:   1000 * 1024 * 1024, // 1000 MiB
			expectedDays:    1,                  // Should be clamped to MinAge (1 day)
			expectedFormula: "MinAge + (MinAge - MaxAge) * ((fileSize / MaxSize) - 1)^3, clamped to MinAge",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Calculate retention using the function
			retention := manager.calculateRetention(tc.fileSizeBytes)

			// Convert retention to days for easier comparison
			actualDays := retention.Hours() / 24

			t.Logf("File size: %.2f MiB", tc.fileSizeBytes/(1024*1024))
			t.Logf("Actual retention: %.2f days", actualDays)
			t.Logf("Expected retention: %.2f days", tc.expectedDays)
			t.Logf("Formula used: %s", tc.expectedFormula)

			// Assert that the calculated days match the expected days (with some floating point tolerance)
			assert.InDelta(t, tc.expectedDays, actualDays, 0.01, "Retention calculation is incorrect")
		})
	}
}

func TestRetentionDecreaseWithSize(t *testing.T) {
	// Setup a test configuration
	cfg := config.Config{
		MinAge:        1,     // 1 day minimum retention
		MaxAge:        30,    // 30 days maximum retention
		MaxSize:       250.0, // 250 MiB threshold
		UploadPath:    "./uploads",
		CheckInterval: 60,
		Enabled:       true,
		BaseURL:       "https://domain.com/",
	}

	manager := &ExpirationManager{Config: cfg}

	// Test sizes in ascending order
	testSizes := []float64{
		1 * 1024 * 1024,    // 1 MiB - small file
		100 * 1024 * 1024,  // 100 MiB - medium file
		250 * 1024 * 1024,  // 250 MiB - exactly at threshold
		300 * 1024 * 1024,  // 300 MiB - just above threshold
		500 * 1024 * 1024,  // 500 MiB - large file
		1000 * 1024 * 1024, // 1000 MiB - very large file
		2000 * 1024 * 1024, // 2000 MiB - extremely large file
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

		// Skip the first iteration since we don't have a previous value
		if previousDays > 0 {
			// For files <= MaxSize, retention should be the same (MaxAge)
			if sizeMiB <= cfg.MaxSize {
				assert.Equal(t, previousDays, days,
					"For files <= MaxSize, retention should remain constant at MaxAge")
			} else {
				// For files > MaxSize, retention should decrease or stay at MinAge
				assert.LessOrEqual(t, days, previousDays,
					"Retention should decrease (or stay at MinAge) as file size increases")
			}
		}

		previousDays = days
	}
}

func TestCorrectFormulaImplementation(t *testing.T) {
	// Setup a test configuration
	cfg := config.Config{
		MinAge:        1,     // 1 day
		MaxAge:        30,    // 30 days
		MaxSize:       250.0, // 250 MiB
		UploadPath:    "./uploads",
		CheckInterval: 60,
		Enabled:       true,
		BaseURL:       "https://domain.com/",
	}

	manager := &ExpirationManager{Config: cfg}

	// Test a file that's exactly twice the threshold size
	// For file_size = 2 * max_size:
	// fileSizeRatio = (500/250) - 1 = 1
	// ageDiff = min_age - max_age = 1 - 30 = -29
	// additionalDays = -29 * (1^3) = -29
	// totalDays = 1 + (-29) = -28, clamped to min_age = 1
	largeFileSize := 500 * 1024 * 1024 // 500 MiB
	retention := manager.calculateRetention(float64(largeFileSize))
	actualDays := retention.Hours() / 24

	t.Logf("For a 500 MiB file (2x threshold):")
	t.Logf("  Expected: 1 day (MinAge) after clamping")
	t.Logf("  Actual: %.2f days", actualDays)

	assert.InDelta(t, 1.0, actualDays, 0.01,
		"A file twice the threshold size should have retention clamped to MinAge (1 day)")
}

func TestGetExpirationDate(t *testing.T) {
	// Setup a test configuration
	cfg := config.Config{
		MinAge:        1,     // 1 day
		MaxAge:        30,    // 30 days
		MaxSize:       250.0, // 250 MiB
		UploadPath:    "./uploads",
		CheckInterval: 60,
		Enabled:       true,
		BaseURL:       "https://domain.com/",
	}

	manager := &ExpirationManager{Config: cfg}

	now := time.Now()

	// Test small file (1 MiB)
	smallFileSize := int64(1 * 1024 * 1024)
	smallFileExpiration := manager.GetExpirationDate(smallFileSize)

	// The difference should be close to MaxAge (30 days)
	smallFileDiff := smallFileExpiration.Sub(now)
	expectedSmallDiff := time.Duration(cfg.MaxAge) * 24 * time.Hour

	t.Logf("1 MiB file expiration: %v (%.2f days from now)",
		smallFileExpiration, smallFileDiff.Hours()/24)

	assert.InDelta(t, expectedSmallDiff.Hours(), smallFileDiff.Hours(), 0.1,
		"Small file expiration should be MaxAge days from now")

	// Test for a file larger than MaxSize
	largeFileSize := int64(500 * 1024 * 1024) // 500 MiB
	largeFileExpiration := manager.GetExpirationDate(largeFileSize)

	largeFileDiff := largeFileExpiration.Sub(now)
	t.Logf("500 MiB file expiration: %v (%.2f days from now)",
		largeFileExpiration, largeFileDiff.Hours()/24)

	// Verify that larger files have less retention time
	assert.Less(t, largeFileDiff.Hours(), smallFileDiff.Hours(),
		"Retention for larger files should be less than smaller files")

	// The large file (500 MiB) should have MinAge (1 day) retention
	expectedLargeDiff := time.Duration(cfg.MinAge) * 24 * time.Hour
	assert.InDelta(t, expectedLargeDiff.Hours(), largeFileDiff.Hours(), 0.1,
		"Large file expiration should be MinAge days from now")
}

// This test validates the behavior with files much larger than the threshold
func TestExtremeFileSizes(t *testing.T) {
	// Setup a test configuration
	cfg := config.Config{
		MinAge:        1,     // 1 day
		MaxAge:        30,    // 30 days
		MaxSize:       250.0, // 250 MiB
		UploadPath:    "./uploads",
		CheckInterval: 60,
		Enabled:       true,
		BaseURL:       "https://domain.com/",
	}

	manager := &ExpirationManager{Config: cfg}

	// Test extremely large file sizes
	testSizes := []float64{
		10000 * 1024 * 1024,   // 10 GB
		100000 * 1024 * 1024,  // 100 GB
		1000000 * 1024 * 1024, // 1 TB
	}

	t.Log("Testing extreme file sizes (should all be clamped to MinAge):")
	t.Log("---------------------------------------------------------")
	for _, size := range testSizes {
		retention := manager.calculateRetention(size)
		days := retention.Hours() / 24
		sizeMiB := size / (1024 * 1024)

		t.Logf("%.2f MiB (%.2f GB): %.2f days",
			sizeMiB, sizeMiB/1024, days)

		// All extreme sizes should be clamped to MinAge
		assert.InDelta(t, float64(cfg.MinAge), days, 0.01,
			"Extremely large files should be clamped to MinAge")
	}
}
