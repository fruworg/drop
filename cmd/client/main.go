package main

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	baseURL string
	client  *Client
)

type UploadResponse struct {
	URL           string `json:"url"`
	Size          int64  `json:"size"`
	Token         string `json:"token"`
	MD5           string `json:"md5"`
	ExpiresAt     string `json:"expires_at"`
	ExpiresInDays int    `json:"expires_in_days"`
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

type ChunkedUploadCompleteResponse struct {
	Message       string `json:"message"`
	Progress      int    `json:"progress"`
	FileURL       string `json:"file_url"`
	MD5           string `json:"md5"`
	Token         string `json:"token"`
	ExpiresAt     string `json:"expires_at"`
	ExpiresInDays int    `json:"expires_in_days"`
}

type Client struct {
	BaseURL    string
	HTTPClient *http.Client
}

func NewClient(baseURL string) *Client {
	if !strings.HasSuffix(baseURL, "/") {
		baseURL += "/"
	}
	return &Client{
		BaseURL: baseURL,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Minute,
		},
	}
}

func (c *Client) UploadFile(filePath string, options map[string]string) (*UploadResponse, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	fileWriter, err := writer.CreateFormFile("file", filepath.Base(filePath))
	if err != nil {
		return nil, fmt.Errorf("failed to create form file: %w", err)
	}

	_, err = io.Copy(fileWriter, file)
	if err != nil {
		return nil, fmt.Errorf("failed to copy file: %w", err)
	}

	for key, value := range options {
		if value != "" {
			writer.WriteField(key, value)
		}
	}

	writer.Close()

	req, err := http.NewRequest("POST", c.BaseURL, &buf)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Accept", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to upload file: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("upload failed with status %d: %s", resp.StatusCode, string(body))
	}

	var uploadResp UploadResponse
	if err := json.NewDecoder(resp.Body).Decode(&uploadResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &uploadResp, nil
}

func (c *Client) UploadFromURL(remoteURL string, options map[string]string) (*UploadResponse, error) {
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	writer.WriteField("url", remoteURL)

	for key, value := range options {
		if value != "" {
			writer.WriteField(key, value)
		}
	}

	writer.Close()

	req, err := http.NewRequest("POST", c.BaseURL, &buf)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Accept", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to upload from URL: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("upload failed with status %d: %s", resp.StatusCode, string(body))
	}

	var uploadResp UploadResponse
	if err := json.NewDecoder(resp.Body).Decode(&uploadResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &uploadResp, nil
}

func (c *Client) InitChunkedUpload(filename string, size int64, chunkSize int64) (*ChunkedUploadInitResponse, error) {
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	writer.WriteField("filename", filename)
	writer.WriteField("size", strconv.FormatInt(size, 10))
	if chunkSize > 0 {
		writer.WriteField("chunk_size", strconv.FormatInt(chunkSize, 10))
	}

	writer.Close()

	req, err := http.NewRequest("POST", c.BaseURL+"upload/init", &buf)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Accept", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize chunked upload: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("init failed with status %d: %s", resp.StatusCode, string(body))
	}

	var initResp ChunkedUploadInitResponse
	if err := json.NewDecoder(resp.Body).Decode(&initResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &initResp, nil
}

func (c *Client) UploadChunk(uploadID string, chunkIndex int, chunkData []byte) (*ChunkedUploadCompleteResponse, error) {
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	chunkWriter, err := writer.CreateFormFile("chunk", fmt.Sprintf("chunk_%d", chunkIndex))
	if err != nil {
		return nil, fmt.Errorf("failed to create chunk form file: %w", err)
	}

	_, err = chunkWriter.Write(chunkData)
	if err != nil {
		return nil, fmt.Errorf("failed to write chunk data: %w", err)
	}

	writer.Close()

	chunkURL := fmt.Sprintf("%supload/chunk/%s/%d", c.BaseURL, uploadID, chunkIndex)
	req, err := http.NewRequest("POST", chunkURL, &buf)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to upload chunk: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("chunk upload failed with status %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var completionResp ChunkedUploadCompleteResponse
	if err := json.Unmarshal(body, &completionResp); err == nil && completionResp.Message == "Upload completed" {
		return &completionResp, nil
	}

	return nil, nil
}

func (c *Client) GetChunkedUploadStatus(uploadID string) (*ChunkedUploadStatusResponse, error) {
	statusURL := fmt.Sprintf("%supload/status/%s", c.BaseURL, uploadID)
	resp, err := c.HTTPClient.Get(statusURL)
	if err != nil {
		return nil, fmt.Errorf("failed to get upload status: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("status check failed with status %d: %s", resp.StatusCode, string(body))
	}

	var statusResp ChunkedUploadStatusResponse
	if err := json.NewDecoder(resp.Body).Decode(&statusResp); err != nil {
		return nil, fmt.Errorf("failed to decode status response: %w", err)
	}

	return &statusResp, nil
}

func (c *Client) UploadFileChunked(filePath string, chunkSize int64, showProgress bool) (*ChunkedUploadCompleteResponse, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	fileInfo, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("failed to get file info: %w", err)
	}

	fileSize := fileInfo.Size()
	if chunkSize <= 0 {
		chunkSize = 4 * 1024 * 1024
	}

	initResp, err := c.InitChunkedUpload(filepath.Base(filePath), fileSize, chunkSize)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize chunked upload: %w", err)
	}

	fmt.Printf("Initialized chunked upload: %s (%d chunks)\n", initResp.UploadID, initResp.TotalChunks)
	if showProgress {
		fmt.Printf("Uploading...\n")
	}

	for i := 0; i < initResp.TotalChunks; i++ {
		chunkData := make([]byte, initResp.ChunkSize)
		n, err := file.Read(chunkData)
		if err != nil && err != io.EOF {
			return nil, fmt.Errorf("failed to read chunk %d: %w", i, err)
		}

		if i == initResp.TotalChunks-1 {
			chunkData = chunkData[:n]
		}

		resp, err := c.UploadChunk(initResp.UploadID, i, chunkData)
		if err != nil {
			return nil, fmt.Errorf("failed to upload chunk %d: %w", i, err)
		}

		if resp != nil {
			printProgress(i+1, initResp.TotalChunks, showProgress)
			return resp, nil
		}

		printProgress(i+1, initResp.TotalChunks, showProgress)
	}

	statusResp, err := c.GetChunkedUploadStatus(initResp.UploadID)
	if err != nil {
		return &ChunkedUploadCompleteResponse{
			Message:  "Upload completed",
			Progress: 100,
			FileURL:  fmt.Sprintf("%s%s", c.BaseURL, initResp.UploadID),
			MD5:      "",
			Token:    "",
		}, nil
	}

	if statusResp.Progress != 100 {
		return nil, fmt.Errorf("upload incomplete: %d%%", statusResp.Progress)
	}

	return &ChunkedUploadCompleteResponse{
		Message:  "Upload completed",
		Progress: 100,
		FileURL:  fmt.Sprintf("%s%s", c.BaseURL, initResp.UploadID),
		MD5:      "",
		Token:    "",
	}, nil
}

func (c *Client) DeleteFile(fileURL, token string) error {
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	writer.WriteField("token", token)
	writer.WriteField("delete", "")
	writer.Close()

	req, err := http.NewRequest("POST", fileURL, &buf)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to delete file: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("delete failed with status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

func (c *Client) SetExpiration(fileURL, token, expires string) error {
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	writer.WriteField("token", token)
	writer.WriteField("expires", expires)
	writer.Close()

	req, err := http.NewRequest("POST", fileURL, &buf)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to set expiration: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("set expiration failed with status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

func printProgress(current, total int, showProgress bool) {
	if !showProgress {
		return
	}

	percent := float64(current) / float64(total) * 100
	barWidth := 30
	filled := int(float64(barWidth) * percent / 100)

	bar := strings.Repeat("█", filled) + strings.Repeat("░", barWidth-filled)
	fmt.Printf("\r%s %.1f%% (%d/%d)", bar, percent, current, total)

	if current == total {
		fmt.Println()
	}
}

func FormatExpiration(expiration string) string {
	if hours, err := strconv.Atoi(expiration); err == nil {
		return strconv.Itoa(hours)
	}

	if _, err := time.Parse(time.RFC3339, expiration); err == nil {
		return expiration
	}

	if _, err := time.Parse("2006-01-02", expiration); err == nil {
		return expiration
	}

	if _, err := time.Parse("2006-01-02T15:04:05", expiration); err == nil {
		return expiration
	}

	if _, err := time.Parse("2006-01-02 15:04:05", expiration); err == nil {
		return expiration
	}

	return expiration
}

func calculateFileMD5(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	hash := md5.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", fmt.Errorf("failed to calculate MD5: %w", err)
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

func verifyMD5(localMD5, serverMD5 string) bool {
	return strings.EqualFold(localMD5, serverMD5)
}

func formatExpirationDate(expiresAt string) string {
	if expiresAt == "" {
		return "Never"
	}

	// Parse the RFC3339 date
	t, err := time.Parse(time.RFC3339, expiresAt)
	if err != nil {
		return expiresAt // Return original if parsing fails
	}

	// Format as a nice readable date
	return t.Format("Jan 2, 2006 at 3:04 PM")
}

func formatDaysRemaining(days int) string {
	if days <= 0 {
		return "expired"
	} else if days == 1 {
		return "1 day"
	} else if days < 7 {
		return fmt.Sprintf("%d days", days)
	} else if days < 30 {
		weeks := days / 7
		if weeks == 1 {
			return "1 week"
		}
		return fmt.Sprintf("%d weeks", weeks)
	} else if days < 365 {
		months := days / 30
		if months == 1 {
			return "1 month"
		}
		return fmt.Sprintf("%d months", months)
	} else {
		years := days / 365
		if years == 1 {
			return "1 year"
		}
		return fmt.Sprintf("%d years", years)
	}
}

func parseSize(sizeStr string) (int64, error) {
	sizeStr = strings.ToUpper(strings.TrimSpace(sizeStr))

	var multiplier int64 = 1
	var size string

	if strings.HasSuffix(sizeStr, "KB") {
		multiplier = 1024
		size = strings.TrimSuffix(sizeStr, "KB")
	} else if strings.HasSuffix(sizeStr, "MB") {
		multiplier = 1024 * 1024
		size = strings.TrimSuffix(sizeStr, "MB")
	} else if strings.HasSuffix(sizeStr, "GB") {
		multiplier = 1024 * 1024 * 1024
		size = strings.TrimSuffix(sizeStr, "GB")
	} else {
		size = sizeStr
	}

	value, err := strconv.ParseInt(size, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid size format: %s", sizeStr)
	}

	return value * multiplier, nil
}

var rootCmd = &cobra.Command{
	Use:   "drop",
	Short: "MZ.DROP Client - Upload and manage files",
	Long: `MZ.DROP Client is a command-line tool for uploading files to a MZ.DROP server.

Features:
  • Upload local files or files from URLs
  • Automatic chunked upload for large files
  • File management (delete, set expiration)
  • Configuration management
  • Progress tracking for uploads
  • Automatic MD5 verification for integrity checking

Quick start:
  drop upload file.txt                    # Upload a file
  drop upload --url https://example.com/file.txt  # Upload from URL
  drop delete abc123 --token your-token   # Delete a file
  drop config set server https://drop.example.com/  # Set server URL`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		baseURL = viper.GetString("server")
		if baseURL == "" {
			baseURL = "http://localhost:3000/"
		}
		client = NewClient(baseURL)
	},
}

var uploadCmd = &cobra.Command{
	Use:     "upload [file]",
	Aliases: []string{"u", "up"},
	Short:   "Upload a file to the server",
	Long: `Upload a file to the MZ.DROP server.

You can upload:
  • Local files: drop upload file.txt
  • From URLs: drop upload --url https://example.com/file.txt
  • Large files (auto-chunked): drop upload large-file.zip

Options:
  --chunked, -c     Force chunked upload for any file size
  --secret          Generate a hard-to-guess URL
  --one-time, -o    Delete file after first download
  --expires, -e     Set expiration time`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		url, _ := cmd.Flags().GetString("url")
		chunked, _ := cmd.Flags().GetBool("chunked")
		chunkSize, _ := cmd.Flags().GetString("chunk-size")
		secret, _ := cmd.Flags().GetBool("secret")
		oneTime, _ := cmd.Flags().GetBool("one-time")
		expires, _ := cmd.Flags().GetString("expires")

		options := make(map[string]string)
		if secret {
			options["secret"] = ""
		}
		if oneTime {
			options["one_time"] = ""
		}
		if expires != "" {
			options["expires"] = FormatExpiration(expires)
		}

		if url != "" {
			if oneTime {
				fmt.Printf("Starting one-time upload from URL (file will be deleted after first download)...\n")
			}
			resp, err := client.UploadFromURL(url, options)
			if err != nil {
				return fmt.Errorf("error uploading from URL: %w", err)
			}
			printUploadResponse(resp, "") // No local MD5 for URL uploads
			return nil
		}

		if len(args) == 0 {
			return fmt.Errorf("file path required when not using --url")
		}

		filePath := args[0]

		// Calculate MD5 hash of local file for verification (unless disabled)
		var localMD5 string
		noVerify, _ := cmd.Root().PersistentFlags().GetBool("no-verify")
		if !noVerify {
			fmt.Printf("Calculating MD5 hash...\n")
			var err error
			localMD5, err = calculateFileMD5(filePath)
			if err != nil {
				return fmt.Errorf("failed to calculate MD5 hash: %w", err)
			}
		}

		// Check if we should auto-enable chunked upload
		shouldUseChunked := chunked
		if !shouldUseChunked {
			// Get file size
			fileInfo, err := os.Stat(filePath)
			if err != nil {
				return fmt.Errorf("failed to get file info: %w", err)
			}

			// Get auto-chunk threshold
			thresholdStr, _ := cmd.Root().PersistentFlags().GetString("auto-chunk-threshold")
			threshold, err := parseSize(thresholdStr)
			if err != nil {
				return fmt.Errorf("invalid auto-chunk-threshold: %w", err)
			}

			// Auto-enable chunked upload for large files
			if fileInfo.Size() > threshold {
				shouldUseChunked = true
				fmt.Printf("File size (%.1f MB) exceeds threshold (%s), using chunked upload\n",
					float64(fileInfo.Size())/1024/1024, thresholdStr)
			}
		}

		if shouldUseChunked {
			var chunkSizeBytes int64
			if chunkSize != "" {
				if sizeMB, err := strconv.ParseInt(chunkSize, 10, 64); err == nil {
					chunkSizeBytes = sizeMB * 1024 * 1024
				} else {
					return fmt.Errorf("invalid chunk size: %s", chunkSize)
				}
			}

			if oneTime {
				fmt.Printf("Starting one-time upload (file will be deleted after first download)...\n")
			}

			noProgress, _ := cmd.Root().PersistentFlags().GetBool("no-progress")
			showProgress := !noProgress
			resp, err := client.UploadFileChunked(filePath, chunkSizeBytes, showProgress)
			if err != nil {
				return fmt.Errorf("error uploading file chunked: %w", err)
			}
			printChunkedUploadResponse(resp, localMD5)
			return nil
		}

		if oneTime {
			fmt.Printf("Starting one-time upload (file will be deleted after first download)...\n")
		}

		resp, err := client.UploadFile(filePath, options)
		if err != nil {
			return fmt.Errorf("error uploading file: %w", err)
		}
		printUploadResponse(resp, localMD5)
		return nil
	},
}

var deleteCmd = &cobra.Command{
	Use:     "delete <file_id>",
	Aliases: []string{"d", "del"},
	Short:   "Delete an uploaded file",
	Long: `Delete a file from the server using its file ID and token.

The file ID is the last part of the file URL.
Example: drop delete abc123 --token your-token`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		fileID := args[0]
		token, _ := cmd.Flags().GetString("token")

		if token == "" {
			return fmt.Errorf("token is required for deletion")
		}

		fileURL := baseURL + fileID
		err := client.DeleteFile(fileURL, token)
		if err != nil {
			return fmt.Errorf("error deleting file: %w", err)
		}

		fmt.Printf("File %s deleted successfully!\n", fileID)
		return nil
	},
}

var expireCmd = &cobra.Command{
	Use:     "expire <file_id>",
	Aliases: []string{"e", "exp"},
	Short:   "Set expiration for an uploaded file",
	Long: `Set or update the expiration time for an uploaded file.

Expiration formats:
  • Hours: 24, 48, 72
  • RFC3339: 2024-12-31T23:59:59Z
  • ISO date: 2024-12-31
  • ISO datetime: 2024-12-31T23:59:59
  • SQL datetime: 2024-12-31 23:59:59

Example: drop expire abc123 --token your-token --expires 24`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		fileID := args[0]
		token, _ := cmd.Flags().GetString("token")
		expires, _ := cmd.Flags().GetString("expires")

		if token == "" {
			return fmt.Errorf("token is required")
		}
		if expires == "" {
			return fmt.Errorf("expiration time is required")
		}

		fileURL := baseURL + fileID
		err := client.SetExpiration(fileURL, token, FormatExpiration(expires))
		if err != nil {
			return fmt.Errorf("error setting expiration: %w", err)
		}

		fmt.Printf("Expiration set successfully for file %s!\n", fileID)
		return nil
	},
}

var configCmd = &cobra.Command{
	Use:     "config",
	Aliases: []string{"c", "cfg"},
	Short:   "Manage client configuration",
	Long: `Manage client configuration settings like server URL and default options.

Configuration is stored in ~/.drop/config.yaml`,
}

var configSetCmd = &cobra.Command{
	Use:     "set <key> <value>",
	Aliases: []string{"s"},
	Short:   "Set a configuration value",
	Long: `Set a configuration value.

Available keys:
  • server: Server URL (e.g., https://drop.example.com/)
  • auto-chunk-threshold: Auto-chunk threshold (e.g., 10MB)

Example: drop config set server https://drop.example.com/`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		key := args[0]
		value := args[1]

		viper.Set(key, value)
		err := viper.WriteConfig()
		if err != nil {
			return fmt.Errorf("error saving configuration: %w", err)
		}

		fmt.Printf("Set %s = %s\n", key, value)
		return nil
	},
}

var configGetCmd = &cobra.Command{
	Use:     "get <key>",
	Aliases: []string{"g"},
	Short:   "Get a configuration value",
	Long: `Get a configuration value.

Example: drop config get server`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		key := args[0]
		value := viper.GetString(key)

		if value == "" {
			fmt.Printf("%s is not set\n", key)
		} else {
			fmt.Printf("%s = %s\n", key, value)
		}
		return nil
	},
}

func printUploadResponse(resp *UploadResponse, localMD5 string) {
	fmt.Printf("Upload successful!\n")
	fmt.Printf("URL: %s\n", resp.URL)
	fmt.Printf("Size: %d bytes\n", resp.Size)
	fmt.Printf("Token: %s\n", resp.Token)

	// Verify MD5 and show result inline
	if localMD5 != "" {
		if verifyMD5(localMD5, resp.MD5) {
			fmt.Printf("MD5: %s ✓\n", resp.MD5)
		} else {
			fmt.Printf("MD5: %s (verification failed - local: %s)\n", resp.MD5, localMD5)
		}
	} else {
		fmt.Printf("MD5: %s\n", resp.MD5)
	}

	fmt.Printf("Expires: %s (%s)\n", formatExpirationDate(resp.ExpiresAt), formatDaysRemaining(resp.ExpiresInDays))
}

func printChunkedUploadResponse(resp *ChunkedUploadCompleteResponse, localMD5 string) {
	fmt.Printf("File URL: %s\n", resp.FileURL)
	fmt.Printf("Token: %s\n", resp.Token)

	// Verify MD5 and show result inline
	if localMD5 != "" {
		if verifyMD5(localMD5, resp.MD5) {
			fmt.Printf("MD5: %s ✓\n", resp.MD5)
		} else {
			fmt.Printf("MD5: %s (verification failed - local: %s)\n", resp.MD5, localMD5)
		}
	} else {
		fmt.Printf("MD5: %s\n", resp.MD5)
	}

	// Show expiration information if available
	if resp.ExpiresAt != "" {
		fmt.Printf("Expires: %s (%s)\n", formatExpirationDate(resp.ExpiresAt), formatDaysRemaining(resp.ExpiresInDays))
	}
}

func init() {
	homeDir, _ := os.UserHomeDir()
	configDir := filepath.Join(homeDir, ".drop")
	os.MkdirAll(configDir, 0755)

	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(configDir)
	viper.AddConfigPath(".")
	viper.ReadInConfig() // Ignore errors if config file doesn't exist

	rootCmd.PersistentFlags().StringP("server", "s", "", "Server URL (default: http://localhost:3000/)")
	rootCmd.PersistentFlags().Bool("no-progress", false, "Disable progress bar for chunked uploads")
	rootCmd.PersistentFlags().Bool("no-verify", false, "Skip MD5 verification after upload")
	rootCmd.PersistentFlags().String("auto-chunk-threshold", "10MB", "Auto-enable chunked upload for files larger than this size (e.g., 10MB, 100MB)")

	viper.BindPFlag("server", rootCmd.PersistentFlags().Lookup("server"))
	viper.BindPFlag("no-progress", rootCmd.PersistentFlags().Lookup("no-progress"))

	uploadCmd.Flags().StringP("url", "u", "", "Upload file from URL instead of local file")
	uploadCmd.Flags().BoolP("chunked", "c", false, "Force chunked upload for any file size")
	uploadCmd.Flags().String("chunk-size", "4", "Chunk size in MB for chunked uploads (default: 4)")
	uploadCmd.Flags().Bool("secret", false, "Generate a hard-to-guess URL")
	uploadCmd.Flags().BoolP("one-time", "o", false, "Delete file after first download")
	uploadCmd.Flags().StringP("expires", "e", "", "Set expiration time (hours, RFC3339, ISO date/datetime, SQL datetime)")

	deleteCmd.Flags().StringP("token", "t", "", "File token (required)")

	expireCmd.Flags().StringP("token", "t", "", "File token (required)")
	expireCmd.Flags().StringP("expires", "e", "", "Expiration time (required)")

	rootCmd.AddCommand(uploadCmd)
	rootCmd.AddCommand(deleteCmd)
	rootCmd.AddCommand(expireCmd)
	rootCmd.AddCommand(configCmd)

	configCmd.AddCommand(configSetCmd)
	configCmd.AddCommand(configGetCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
