package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/marianozunino/drop/internal/app"
	"github.com/marianozunino/drop/internal/config"
	"github.com/marianozunino/drop/internal/testutil"
)

var (
	baseURL = "http://localhost:0" // Will be updated with actual port
	testApp *app.App
)

type TestResult struct {
	Name     string
	Passed   bool
	Error    string
	Duration time.Duration
}

func TestMain(m *testing.M) {
	// Setup test environment
	tempDir, err := os.MkdirTemp("", "drop-e2e-test")
	if err != nil {
		fmt.Printf("Failed to create temp directory: %v\n", err)
		os.Exit(1)
	}
	defer os.RemoveAll(tempDir)

	// Create test configuration
	testDBPath := filepath.Join(tempDir, "test.db")
	cfg := &config.Config{
		Port:                     8081, // Use a specific port for E2E tests
		UploadPath:               filepath.Join(tempDir, "uploads"),
		MinAge:                   1,
		MaxAge:                   30,
		MaxSize:                  250.0,
		CheckInterval:            60,
		ExpirationManagerEnabled: true,
		BaseURL:                  "http://localhost:8081/",
		SQLitePath:               testDBPath,
		IdLength:                 4,
		PreviewBots: []string{
			"slack",
			"slackbot",
			"facebookexternalhit",
			"twitterbot",
		},
		URLShorteningEnabled: true,
	}

	// Create and start test app
	testApp, err = createTestApp(cfg)
	if err != nil {
		fmt.Printf("Failed to create test app: %v\n", err)
		os.Exit(1)
	}

	// Start the server
	testApp.Start()

	// Wait for server to start
	time.Sleep(100 * time.Millisecond)
	baseURL = fmt.Sprintf("http://localhost:%d", cfg.Port)

	// Wait for server to be ready
	if !waitForServer(baseURL, 5*time.Second) {
		fmt.Printf("Server failed to start at %s\n", baseURL)
		testApp.Stop()
		os.Exit(1)
	}

	fmt.Println("Starting E2E tests against", baseURL)
	fmt.Println(strings.Repeat("=", 50))

	// Run tests
	code := m.Run()

	fmt.Println(strings.Repeat("=", 50))
	if code == 0 {
		fmt.Println("All tests passed!")
	} else {
		fmt.Println("Some tests failed!")
	}

	// Stop the server
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	testApp.Shutdown(ctx)
	testApp.Stop()

	os.Exit(code)
}

// createTestApp creates a test app instance with the given configuration
func createTestApp(cfg *config.Config) (*app.App, error) {
	// Create app with custom config
	testApp, err := app.NewWithConfig(cfg)
	if err != nil {
		return nil, err
	}

	// Run migrations for the test database
	err = testutil.RunTestMigrationsFromProjectRoot(cfg.SQLitePath)
	if err != nil {
		return nil, err
	}

	return testApp, nil
}

// waitForServer waits for the server to be ready
func waitForServer(url string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		resp, err := http.Get(url)
		if err == nil {
			resp.Body.Close()
			return resp.StatusCode == 200
		}
		time.Sleep(100 * time.Millisecond)
	}
	return false
}

func TestFileUpload(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		filename string
	}{
		{
			name:     "Simple text file",
			content:  "Hello, World! This is a test file.",
			filename: "test.txt",
		},
		{
			name:     "Large content",
			content:  strings.Repeat("This is a test line. ", 100),
			filename: "large.txt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start := time.Now()

			// Create test file
			testFile := createTempFile(tt.content, tt.filename)
			defer os.Remove(testFile)

			// Upload file
			uploadURL, err := uploadFile(testFile)
			if err != nil {
				t.Errorf("Upload failed: %v", err)
				return
			}

			// Verify file access
			downloadedContent, err := downloadFile(uploadURL)
			if err != nil {
				t.Errorf("Download failed: %v", err)
				return
			}

			// Verify content matches
			if downloadedContent != tt.content {
				t.Errorf("Content mismatch. Expected: %q, Got: %q", tt.content, downloadedContent)
				return
			}

			duration := time.Since(start)
			t.Logf("%s - Upload: %s, Duration: %v", tt.name, uploadURL, duration)
		})
	}
}

func TestFileUploadWithOptions(t *testing.T) {
	tests := []struct {
		name        string
		content     string
		filename    string
		options     map[string]string
		expectError bool
	}{
		{
			name:     "File with expiration (1 hour)",
			content:  "File with 1 hour expiration",
			filename: "expire_1h.txt",
			options: map[string]string{
				"expires": "1",
			},
		},
		{
			name:     "File with expiration (7 days)",
			content:  "File with 7 days expiration",
			filename: "expire_7d.txt",
			options: map[string]string{
				"expires": "168", // 7 days * 24 hours
			},
		},
		{
			name:     "File with secret ID",
			content:  "File with secret ID",
			filename: "secret.txt",
			options: map[string]string{
				"secret": "true",
			},
		},
		{
			name:     "File with one-time view",
			content:  "File with one-time view",
			filename: "onetime.txt",
			options: map[string]string{
				"one_time": "true",
			},
		},
		{
			name:     "File with secret + one-time",
			content:  "File with secret ID and one-time view",
			filename: "secret_onetime.txt",
			options: map[string]string{
				"secret":   "true",
				"one_time": "true",
			},
		},
		{
			name:     "File with expiration + secret",
			content:  "File with expiration and secret ID",
			filename: "expire_secret.txt",
			options: map[string]string{
				"expires": "2", // 2 hours
				"secret":  "true",
			},
		},
		{
			name:     "File with all options",
			content:  "File with all options enabled",
			filename: "all_options.txt",
			options: map[string]string{
				"expires":  "24", // 24 hours = 1 day
				"secret":   "true",
				"one_time": "true",
			},
		},
		{
			name:     "File with invalid expiration",
			content:  "File with invalid expiration",
			filename: "invalid_expire.txt",
			options: map[string]string{
				"expires": "invalid",
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start := time.Now()

			// Create test file
			testFile := createTempFile(tt.content, tt.filename)
			defer os.Remove(testFile)

			// Upload file with options
			uploadURL, _, err := uploadFileWithOptions(testFile, tt.options)
			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error for %s, but got none", tt.name)
				} else {
					t.Logf("%s - Correctly rejected: %v", tt.name, err)
				}
				return
			}

			if err != nil {
				t.Errorf("Upload failed: %v", err)
				return
			}

			// Verify file access
			downloadedContent, err := downloadFile(uploadURL)
			if err != nil {
				t.Errorf("Download failed: %v", err)
				return
			}

			// Verify content matches
			if downloadedContent != tt.content {
				t.Errorf("Content mismatch. Expected: %q, Got: %q", tt.content, downloadedContent)
				return
			}

			// Test one-time view behavior
			if tt.options["one_time"] == "true" {
				// Second access should fail
				_, err = downloadFile(uploadURL)
				if err == nil {
					t.Errorf("One-time file should be deleted after first access")
				} else {
					t.Logf("%s - One-time view working correctly", tt.name)
				}
			}

			duration := time.Since(start)
			t.Logf("%s - Upload: %s, Duration: %v", tt.name, uploadURL, duration)
		})
	}
}

func TestURLShorteningWithOptions(t *testing.T) {
	tests := []struct {
		name        string
		originalURL string
		options     map[string]string
		expectError bool
	}{
		{
			name:        "URL with expiration (1 hour)",
			originalURL: "https://example.com/expire-1h",
			options: map[string]string{
				"expires": "1",
			},
		},
		{
			name:        "URL with expiration (7 days)",
			originalURL: "https://example.com/expire-7d",
			options: map[string]string{
				"expires": "168", // 7 days * 24 hours
			},
		},
		{
			name:        "URL with secret ID",
			originalURL: "https://example.com/secret",
			options: map[string]string{
				"secret": "true",
			},
		},
		{
			name:        "URL with one-time view",
			originalURL: "https://example.com/onetime",
			options: map[string]string{
				"one_time": "true",
			},
		},
		{
			name:        "URL with secret + one-time",
			originalURL: "https://example.com/secret-onetime",
			options: map[string]string{
				"secret":   "true",
				"one_time": "true",
			},
		},
		{
			name:        "URL with expiration + secret",
			originalURL: "https://example.com/expire-secret",
			options: map[string]string{
				"expires": "2", // 2 hours
				"secret":  "true",
			},
		},
		{
			name:        "URL with all options",
			originalURL: "https://example.com/all-options",
			options: map[string]string{
				"expires":  "24", // 24 hours = 1 day
				"secret":   "true",
				"one_time": "true",
			},
		},
		{
			name:        "URL with invalid expiration",
			originalURL: "https://example.com/invalid-expire",
			options: map[string]string{
				"expires": "invalid",
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start := time.Now()

			// Shorten URL with options
			shortURL, _, err := shortenURLWithOptions(tt.originalURL, tt.options)
			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error for %s, but got none", tt.name)
				} else {
					t.Logf("%s - Correctly rejected: %v", tt.name, err)
				}
				return
			}

			if err != nil {
				t.Errorf("URL shortening failed: %v", err)
				return
			}

			// Verify redirection
			redirectURL, err := getRedirectURL(shortURL)
			if err != nil {
				t.Errorf("Redirection failed: %v", err)
				return
			}

			// Verify redirect URL matches original
			if redirectURL != tt.originalURL {
				t.Errorf("Redirect mismatch. Expected: %s, Got: %s", tt.originalURL, redirectURL)
				return
			}

			// Test one-time view behavior
			if tt.options["one_time"] == "true" {
				// Second access should fail
				_, err = getRedirectURL(shortURL)
				if err == nil {
					t.Errorf("One-time URL should be deleted after first access")
				} else {
					t.Logf("%s - One-time view working correctly", tt.name)
				}
			}

			duration := time.Since(start)
			t.Logf("%s - Short: %s → %s, Duration: %v", tt.name, shortURL, redirectURL, duration)
		})
	}
}

func TestInvalidInputs(t *testing.T) {
	t.Run("Invalid URL format", func(t *testing.T) {
		_, err := shortenURL("not-a-valid-url")
		if err == nil {
			t.Error("Expected error for invalid URL, but got none")
		} else {
			t.Logf("Invalid URL correctly rejected: %v", err)
		}
	})

	t.Run("Empty URL", func(t *testing.T) {
		_, err := shortenURL("")
		if err == nil {
			t.Error("Expected error for empty URL, but got none")
		} else {
			t.Logf("Empty URL correctly rejected: %v", err)
		}
	})

	t.Run("Non-existent file access", func(t *testing.T) {
		resp, err := http.Get(baseURL + "/nonexistent123")
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != 404 {
			t.Errorf("Expected 404 for non-existent file, got %d", resp.StatusCode)
		} else {
			t.Logf("Non-existent file correctly returns 404")
		}
	})
}

func TestChunkedUpload(t *testing.T) {
	t.Run("Chunked upload initialization", func(t *testing.T) {
		start := time.Now()

		// Test chunked upload initialization
		initResp, err := initChunkedUpload("test_chunked.txt", 1000)
		if err != nil {
			t.Errorf("Chunked upload init failed: %v", err)
			return
		}

		// Verify initialization response
		if initResp.UploadID == "" {
			t.Errorf("Upload ID should not be empty")
			return
		}

		if initResp.TotalChunks <= 0 {
			t.Errorf("Total chunks should be > 0, got %d", initResp.TotalChunks)
			return
		}

		if initResp.ChunkSize <= 0 {
			t.Errorf("Chunk size should be > 0, got %d", initResp.ChunkSize)
			return
		}

		duration := time.Since(start)
		t.Logf("Chunked upload init - UploadID: %s, Chunks: %d, ChunkSize: %d, Duration: %v",
			initResp.UploadID, initResp.TotalChunks, initResp.ChunkSize, duration)
	})

	t.Run("Chunked upload status check", func(t *testing.T) {
		start := time.Now()

		// Initialize upload
		initResp, err := initChunkedUpload("status_test.txt", 500)
		if err != nil {
			t.Errorf("Chunked upload init failed: %v", err)
			return
		}

		// Check status
		statusResp, err := getChunkedUploadStatus(initResp.UploadID)
		if err != nil {
			t.Errorf("Status check failed: %v", err)
			return
		}

		// Verify status response
		if statusResp.Progress < 0 || statusResp.Progress > 100 {
			t.Errorf("Progress should be between 0-100, got %d", statusResp.Progress)
			return
		}

		duration := time.Since(start)
		t.Logf("Chunked upload status - UploadID: %s, Progress: %d%%, Duration: %v",
			initResp.UploadID, statusResp.Progress, duration)
	})

	t.Run("Complete chunked upload", func(t *testing.T) {
		start := time.Now()

		content := "This is a test file for complete chunked upload with multiple chunks."
		filename := "complete_chunked.txt"
		chunkSize := 5

		initResp, err := initChunkedUpload(filename, len(content))
		if err != nil {
			t.Errorf("Chunked upload init failed: %v", err)
			return
		}

		t.Logf("Initialized upload session: %s", initResp.UploadID)

		for i := 0; i < initResp.TotalChunks; i++ {
			start := i * chunkSize
			end := start + chunkSize
			if end > len(content) {
				end = len(content)
			}

			chunkContent := content[start:end]
			err := uploadChunk(initResp.UploadID, i, chunkContent)
			if err != nil {
				t.Errorf("Failed to upload chunk %d: %v", i, err)
				return
			}
			t.Logf("Uploaded chunk %d/%d", i+1, initResp.TotalChunks)
		}

		uploadURL := fmt.Sprintf("%s/%s.txt", baseURL, initResp.UploadID)

		downloadedContent, err := downloadFile(uploadURL)
		if err != nil {
			t.Errorf("Download failed: %v", err)
			return
		}

		if downloadedContent != content {
			t.Errorf("Content mismatch. Expected: %q, Got: %q", content, downloadedContent)
			return
		}

		duration := time.Since(start)
		t.Logf("Complete chunked upload - Upload: %s, Duration: %v", uploadURL, duration)
	})
}

func TestFileDeletion(t *testing.T) {
	tests := []struct {
		name        string
		content     string
		filename    string
		options     map[string]string
		expectError bool
	}{
		{
			name:     "Delete regular file",
			content:  "This file will be deleted",
			filename: "delete_me.txt",
			options:  nil,
		},
		{
			name:     "Delete file with secret ID",
			content:  "This secret file will be deleted",
			filename: "delete_secret.txt",
			options: map[string]string{
				"secret": "true",
			},
		},
		{
			name:     "Delete file with expiration",
			content:  "This expiring file will be deleted",
			filename: "delete_expire.txt",
			options: map[string]string{
				"expires": "24", // 24 hours
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start := time.Now()

			// Create and upload file
			testFile := createTempFile(tt.content, tt.filename)
			defer os.Remove(testFile)

			uploadURL, token, err := uploadFileWithOptions(testFile, tt.options)
			if err != nil {
				t.Errorf("Upload failed: %v", err)
				return
			}

			// Extract file ID and token from upload URL
			fileID := extractFileIDFromURL(uploadURL)
			if fileID == "" {
				t.Errorf("Could not extract file ID from URL: %s", uploadURL)
				return
			}

			if token == "" {
				t.Errorf("No token received from upload")
				return
			}

			// Verify file exists before deletion
			_, err = downloadFile(uploadURL)
			if err != nil {
				t.Errorf("File should exist before deletion: %v", err)
				return
			}

			// Delete file
			err = deleteFile(fileID, token)
			if err != nil {
				t.Errorf("File deletion failed: %v", err)
				return
			}

			// Verify file is deleted
			_, err = downloadFile(uploadURL)
			if err == nil {
				t.Errorf("File should be deleted but still accessible")
			} else {
				t.Logf("%s - File successfully deleted", tt.name)
			}

			duration := time.Since(start)
			t.Logf("%s - Deleted: %s, Duration: %v", tt.name, uploadURL, duration)
		})
	}
}

func TestURLShortenerDeletion(t *testing.T) {
	tests := []struct {
		name        string
		originalURL string
		options     map[string]string
		expectError bool
	}{
		{
			name:        "Delete basic URL shortener",
			originalURL: "https://example.com/basic",
			options:     nil,
		},
		{
			name:        "Delete URL shortener with expiration",
			originalURL: "https://example.com/expire",
			options: map[string]string{
				"expires": "24", // 24 hours
			},
		},
		{
			name:        "Delete URL shortener with secret ID",
			originalURL: "https://example.com/secret",
			options: map[string]string{
				"secret": "true",
			},
		},
		{
			name:        "Delete URL shortener with one-time view (test auto-deletion)",
			originalURL: "https://example.com/onetime",
			options: map[string]string{
				"one_time": "true",
			},
			expectError: true, // One-time URLs auto-delete after first access
		},
		{
			name:        "Delete URL shortener with all options (test auto-deletion)",
			originalURL: "https://example.com/all-options",
			options: map[string]string{
				"expires":  "48", // 48 hours
				"secret":   "true",
				"one_time": "true",
			},
			expectError: true, // One-time URLs auto-delete after first access
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start := time.Now()

			// Shorten URL with options
			shortURL, token, err := shortenURLWithOptions(tt.originalURL, tt.options)
			if err != nil {
				t.Errorf("URL shortening failed: %v", err)
				return
			}

			// Extract short ID from URL
			shortID := extractFileIDFromURL(shortURL)
			if shortID == "" {
				t.Errorf("Could not extract short ID from URL: %s", shortURL)
				return
			}

			if token == "" {
				t.Errorf("No token received from shorten response")
				return
			}

			if tt.expectError {
				// For one-time view URLs, test that they auto-delete after first access
				redirectURL, err := getRedirectURL(shortURL)
				if err != nil {
					t.Errorf("One-time URL should work on first access: %v", err)
					return
				}

				if redirectURL != tt.originalURL {
					t.Errorf("Expected redirect to %s, got %s", tt.originalURL, redirectURL)
					return
				}

				// Try to access again - should fail because it's auto-deleted
				_, err = getRedirectURL(shortURL)
				if err == nil {
					t.Errorf("One-time URL should be auto-deleted after first access")
				} else {
					t.Logf("%s - One-time URL correctly auto-deleted after first access", tt.name)
				}
			} else {
				// For regular URLs, test manual deletion
				// Verify URL shortener works before deletion
				redirectURL, err := getRedirectURL(shortURL)
				if err != nil {
					t.Errorf("URL shortener should work before deletion: %v", err)
					return
				}

				if redirectURL != tt.originalURL {
					t.Errorf("Expected redirect to %s, got %s", tt.originalURL, redirectURL)
					return
				}

				// Delete URL shortener
				err = deleteFile(shortID, token)
				if err != nil {
					t.Errorf("URL shortener deletion failed: %v", err)
					return
				}

				// Verify URL shortener is deleted
				_, err = getRedirectURL(shortURL)
				if err == nil {
					t.Errorf("URL shortener should be deleted but still accessible")
				} else {
					t.Logf("%s - URL shortener successfully deleted", tt.name)
				}
			}

			duration := time.Since(start)
			t.Logf("%s - Deleted: %s, Duration: %v", tt.name, shortURL, duration)
		})
	}
}

func TestConcurrentOperations(t *testing.T) {
	t.Run("Concurrent file uploads", func(t *testing.T) {
		const numUploads = 5
		results := make(chan string, numUploads)
		errors := make(chan error, numUploads)

		for i := range numUploads {
			go func(id int) {
				content := fmt.Sprintf("Concurrent test file %d", id)
				testFile := createTempFile(content, fmt.Sprintf("concurrent_%d.txt", id))
				defer os.Remove(testFile)

				uploadURL, err := uploadFile(testFile)
				if err != nil {
					errors <- err
					return
				}
				results <- uploadURL
			}(i)
		}

		successCount := 0
		for i := range numUploads {
			select {
			case uploadURL := <-results:
				t.Logf("Concurrent upload %d: %s", i+1, uploadURL)
				successCount++
			case err := <-errors:
				t.Errorf("Concurrent upload failed: %v", err)
			case <-time.After(10 * time.Second):
				t.Error("Concurrent upload timeout")
			}
		}

		if successCount == numUploads {
			t.Logf("All %d concurrent uploads succeeded", numUploads)
		}
	})

	t.Run("Concurrent URL shortening", func(t *testing.T) {
		const numShortens = 5
		results := make(chan string, numShortens)
		errors := make(chan error, numShortens)

		for i := range numShortens {
			go func(id int) {
				testURL := fmt.Sprintf("https://example%d.com/test", id)
				shortURL, err := shortenURL(testURL)
				if err != nil {
					errors <- err
					return
				}
				results <- shortURL
			}(i)
		}

		successCount := 0
		for i := range numShortens {
			select {
			case shortURL := <-results:
				t.Logf("Concurrent shorten %d: %s", i+1, shortURL)
				successCount++
			case err := <-errors:
				t.Errorf("Concurrent shorten failed: %v", err)
			case <-time.After(10 * time.Second):
				t.Error("Concurrent shorten timeout")
			}
		}

		if successCount == numShortens {
			t.Logf("All %d concurrent URL shortenings succeeded", numShortens)
		}
	})
}

// Helper functions

func createTempFile(content, filename string) string {
	file, err := os.CreateTemp("", filename)
	if err != nil {
		panic(fmt.Sprintf("Failed to create temp file: %v", err))
	}
	defer file.Close()

	_, err = file.WriteString(content)
	if err != nil {
		panic(fmt.Sprintf("Failed to write to temp file: %v", err))
	}

	return file.Name()
}

func uploadFile(filePath string) (string, error) {
	uploadURL, _, err := uploadFileWithOptions(filePath, nil)
	return uploadURL, err
}

func uploadFileWithOptions(filePath string, options map[string]string) (string, string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", "", err
	}
	defer file.Close()

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	part, err := writer.CreateFormFile("file", filepath.Base(filePath))
	if err != nil {
		return "", "", err
	}

	_, err = io.Copy(part, file)
	if err != nil {
		return "", "", err
	}

	// Add options
	for key, value := range options {
		if value != "" {
			writer.WriteField(key, value)
		}
	}

	err = writer.Close()
	if err != nil {
		return "", "", err
	}

	req, err := http.NewRequest("POST", baseURL, &buf)
	if err != nil {
		return "", "", err
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return "", "", fmt.Errorf("upload failed with status %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", err
	}

	uploadURL := strings.TrimSpace(string(body))
	token := resp.Header.Get("X-Token")

	return uploadURL, token, nil
}

func downloadFile(uploadURL string) (string, error) {
	req, err := http.NewRequest("GET", uploadURL, nil)
	if err != nil {
		return "", err
	}

	// Don't accept gzip to avoid compression issues in tests
	req.Header.Set("Accept-Encoding", "identity")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("download failed with status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(body), nil
}

func shortenURL(originalURL string) (string, error) {
	shortURL, _, err := shortenURLWithOptions(originalURL, nil)
	return shortURL, err
}

func shortenURLWithOptions(originalURL string, options map[string]string) (string, string, error) {
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	writer.WriteField("shorten", "true")
	writer.WriteField("url", originalURL)

	// Add options
	for key, value := range options {
		if value != "" {
			writer.WriteField(key, value)
		}
	}

	err := writer.Close()
	if err != nil {
		return "", "", err
	}

	req, err := http.NewRequest("POST", baseURL, &buf)
	if err != nil {
		return "", "", err
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return "", "", fmt.Errorf("shorten failed with status %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", err
	}

	shortURL := strings.TrimSpace(string(body))
	token := resp.Header.Get("X-Token")

	return shortURL, token, nil
}

func getRedirectURL(shortURL string) (string, error) {
	client := &http.Client{
		Timeout: 30 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse // Don't follow redirects
		},
	}

	resp, err := client.Get(shortURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 302 {
		return "", fmt.Errorf("expected redirect (302), got status %d", resp.StatusCode)
	}

	location := resp.Header.Get("Location")
	if location == "" {
		return "", fmt.Errorf("no Location header in redirect response")
	}

	return location, nil
}

func initChunkedUpload(filename string, totalSize int) (*ChunkedUploadInitResponse, error) {
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	writer.WriteField("filename", filename)
	writer.WriteField("size", fmt.Sprintf("%d", totalSize))
	writer.WriteField("chunk_size", fmt.Sprintf("%d", 5))

	err := writer.Close()
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", baseURL+"/upload/init", &buf)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("init failed with status %d: %s", resp.StatusCode, string(body))
	}

	var initResp ChunkedUploadInitResponse
	err = json.NewDecoder(resp.Body).Decode(&initResp)
	if err != nil {
		return nil, err
	}

	return &initResp, nil
}

func uploadChunk(uploadID string, chunkIndex int, chunkContent string) error {
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	part, err := writer.CreateFormFile("chunk", fmt.Sprintf("chunk_%d", chunkIndex))
	if err != nil {
		return err
	}

	_, err = part.Write([]byte(chunkContent))
	if err != nil {
		return err
	}

	err = writer.Close()
	if err != nil {
		return err
	}

	url := fmt.Sprintf("%s/upload/chunk/%s/%d", baseURL, uploadID, chunkIndex)
	req, err := http.NewRequest("POST", url, &buf)
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("chunk upload failed with status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

func completeChunkedUpload(uploadID string) (string, error) {
	statusResp, err := getChunkedUploadStatus(uploadID)
	if err != nil {
		return "", fmt.Errorf("failed to check upload status: %w", err)
	}

	if statusResp.Progress != 100 {
		return "", fmt.Errorf("upload not complete: %d%%", statusResp.Progress)
	}

	fileExt := ".txt"
	return fmt.Sprintf("%s/%s%s", baseURL, uploadID, fileExt), nil
}

func getChunkedUploadStatus(uploadID string) (*ChunkedUploadStatusResponse, error) {
	url := fmt.Sprintf("%s/upload/status/%s", baseURL, uploadID)
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("status check failed with status %d: %s", resp.StatusCode, string(body))
	}

	var statusResp ChunkedUploadStatusResponse
	err = json.NewDecoder(resp.Body).Decode(&statusResp)
	if err != nil {
		return nil, err
	}

	return &statusResp, nil
}

func extractFileIDFromURL(uploadURL string) string {
	parts := strings.Split(uploadURL, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return ""
}

func deleteFile(fileID, token string) error {
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	writer.WriteField("delete", "true")
	writer.WriteField("token", token)

	err := writer.Close()
	if err != nil {
		return err
	}

	url := fmt.Sprintf("%s/%s", baseURL, fileID)
	req, err := http.NewRequest("POST", url, &buf)
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("delete failed with status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

type ChunkedUploadInitResponse struct {
	UploadID       string `json:"upload_id"`
	ChunkSize      int64  `json:"chunk_size"`
	TotalChunks    int    `json:"total_chunks"`
	UploadedChunks []int  `json:"uploaded_chunks"`
}

type ChunkedUploadStatusResponse struct {
	Progress       int   `json:"progress"`
	UploadedChunks []int `json:"uploaded_chunks"`
}
