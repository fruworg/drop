package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewClient(t *testing.T) {
	client := NewClient("http://example.com")
	assert.Equal(t, "http://example.com/", client.BaseURL)
	assert.NotNil(t, client.HTTPClient)
	assert.Equal(t, 30*time.Minute, client.HTTPClient.Timeout)

	client = NewClient("http://example.com/")
	assert.Equal(t, "http://example.com/", client.BaseURL)

	client = NewClient("")
	assert.Equal(t, "/", client.BaseURL)
}

func TestUploadResponseJSON(t *testing.T) {
	response := UploadResponse{
		URL:           "http://example.com/file.txt",
		Size:          1024,
		Token:         "abc123",
		MD5:           "d41d8cd98f00b204e9800998ecf8427e",
		ExpiresAt:     "2023-12-31T23:59:59Z",
		ExpiresInDays: 30,
	}

	jsonData, err := json.Marshal(response)
	require.NoError(t, err)

	var unmarshaled UploadResponse
	err = json.Unmarshal(jsonData, &unmarshaled)
	require.NoError(t, err)

	assert.Equal(t, response.URL, unmarshaled.URL)
	assert.Equal(t, response.Size, unmarshaled.Size)
	assert.Equal(t, response.Token, unmarshaled.Token)
	assert.Equal(t, response.MD5, unmarshaled.MD5)
	assert.Equal(t, response.ExpiresAt, unmarshaled.ExpiresAt)
	assert.Equal(t, response.ExpiresInDays, unmarshaled.ExpiresInDays)
}

func TestChunkedUploadInitResponseJSON(t *testing.T) {
	response := ChunkedUploadInitResponse{
		UploadID:       "upload-123",
		ChunkSize:      4 * 1024 * 1024,
		TotalChunks:    10,
		UploadedChunks: []int{1, 2, 3},
	}

	jsonData, err := json.Marshal(response)
	require.NoError(t, err)

	var unmarshaled ChunkedUploadInitResponse
	err = json.Unmarshal(jsonData, &unmarshaled)
	require.NoError(t, err)

	assert.Equal(t, response.UploadID, unmarshaled.UploadID)
	assert.Equal(t, response.ChunkSize, unmarshaled.ChunkSize)
	assert.Equal(t, response.TotalChunks, unmarshaled.TotalChunks)
	assert.Equal(t, response.UploadedChunks, unmarshaled.UploadedChunks)
}

func TestChunkedUploadStatusResponseJSON(t *testing.T) {
	response := ChunkedUploadStatusResponse{
		Progress:       50,
		UploadedChunks: []int{1, 2, 3, 4, 5},
	}

	jsonData, err := json.Marshal(response)
	require.NoError(t, err)

	var unmarshaled ChunkedUploadStatusResponse
	err = json.Unmarshal(jsonData, &unmarshaled)
	require.NoError(t, err)

	assert.Equal(t, response.Progress, unmarshaled.Progress)
	assert.Equal(t, response.UploadedChunks, unmarshaled.UploadedChunks)
}

func TestChunkedUploadCompleteResponseJSON(t *testing.T) {
	response := ChunkedUploadCompleteResponse{
		Message:       "Upload completed",
		Progress:      100,
		FileURL:       "http://example.com/file.txt",
		MD5:           "d41d8cd98f00b204e9800998ecf8427e",
		Token:         "abc123",
		ExpiresAt:     "2023-12-31T23:59:59Z",
		ExpiresInDays: 30,
	}

	jsonData, err := json.Marshal(response)
	require.NoError(t, err)

	var unmarshaled ChunkedUploadCompleteResponse
	err = json.Unmarshal(jsonData, &unmarshaled)
	require.NoError(t, err)

	assert.Equal(t, response.Message, unmarshaled.Message)
	assert.Equal(t, response.Progress, unmarshaled.Progress)
	assert.Equal(t, response.FileURL, unmarshaled.FileURL)
	assert.Equal(t, response.MD5, unmarshaled.MD5)
	assert.Equal(t, response.Token, unmarshaled.Token)
	assert.Equal(t, response.ExpiresAt, unmarshaled.ExpiresAt)
	assert.Equal(t, response.ExpiresInDays, unmarshaled.ExpiresInDays)
}

func TestClientUploadFile(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/", r.URL.Path)
		assert.Contains(t, r.Header.Get("Content-Type"), "multipart/form-data")
		assert.Equal(t, "application/json", r.Header.Get("Accept"))

		err := r.ParseMultipartForm(32 << 20)
		require.NoError(t, err)

		file, header, err := r.FormFile("file")
		require.NoError(t, err)
		defer file.Close()

		assert.Equal(t, "test.txt", header.Filename)

		assert.Equal(t, "value1", r.FormValue("option1"))
		assert.Equal(t, "value2", r.FormValue("option2"))

		response := UploadResponse{
			URL:           "http://example.com/test.txt",
			Size:          1024,
			Token:         "test-token",
			MD5:           "d41d8cd98f00b204e9800998ecf8427e",
			ExpiresAt:     "2023-12-31T23:59:59Z",
			ExpiresInDays: 30,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "test.txt")
	content := "Hello, World!"
	err := os.WriteFile(filePath, []byte(content), 0644)
	require.NoError(t, err)

	client := NewClient(server.URL)
	options := map[string]string{
		"option1": "value1",
		"option2": "value2",
		"empty":   "",
	}

	response, err := client.UploadFile(filePath, options)
	require.NoError(t, err)

	assert.Equal(t, "http://example.com/test.txt", response.URL)
	assert.Equal(t, int64(1024), response.Size)
	assert.Equal(t, "test-token", response.Token)
	assert.Equal(t, "d41d8cd98f00b204e9800998ecf8427e", response.MD5)
	assert.Equal(t, "2023-12-31T23:59:59Z", response.ExpiresAt)
	assert.Equal(t, 30, response.ExpiresInDays)
}

func TestClientUploadFileWithNonExistentFile(t *testing.T) {
	client := NewClient("http://example.com/")
	options := map[string]string{}

	response, err := client.UploadFile("/non/existent/file.txt", options)
	assert.Error(t, err)
	assert.Nil(t, response)
}

func TestClientUploadFileWithServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal Server Error"))
	}))
	defer server.Close()

	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "test.txt")
	err := os.WriteFile(filePath, []byte("test content"), 0644)
	require.NoError(t, err)

	client := NewClient(server.URL)
	options := map[string]string{}

	response, err := client.UploadFile(filePath, options)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "upload failed with status 500")
	assert.Nil(t, response)
}

func TestClientUploadFromURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/", r.URL.Path)

		err := r.ParseMultipartForm(32 << 20)
		require.NoError(t, err)

		assert.Equal(t, "http://example.com/remote-file.txt", r.FormValue("url"))
		assert.Equal(t, "value1", r.FormValue("option1"))

		response := UploadResponse{
			URL:           "http://example.com/remote-file.txt",
			Size:          2048,
			Token:         "remote-token",
			MD5:           "remote-md5-hash",
			ExpiresAt:     "2023-12-31T23:59:59Z",
			ExpiresInDays: 7,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := NewClient(server.URL)
	options := map[string]string{
		"option1": "value1",
		"empty":   "",
	}

	response, err := client.UploadFromURL("http://example.com/remote-file.txt", options)
	require.NoError(t, err)

	assert.Equal(t, "http://example.com/remote-file.txt", response.URL)
	assert.Equal(t, int64(2048), response.Size)
	assert.Equal(t, "remote-token", response.Token)
	assert.Equal(t, "remote-md5-hash", response.MD5)
	assert.Equal(t, "2023-12-31T23:59:59Z", response.ExpiresAt)
	assert.Equal(t, 7, response.ExpiresInDays)
}

func TestClientInitChunkedUpload(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/upload/init", r.URL.Path)

		err := r.ParseMultipartForm(32 << 20)
		require.NoError(t, err)

		assert.Equal(t, "test-file.txt", r.FormValue("filename"))
		assert.Equal(t, "1048576", r.FormValue("size"))
		assert.Equal(t, "4194304", r.FormValue("chunk_size"))

		response := ChunkedUploadInitResponse{
			UploadID:       "upload-123",
			ChunkSize:      4 * 1024 * 1024,
			TotalChunks:    1,
			UploadedChunks: []int{},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := NewClient(server.URL)

	response, err := client.InitChunkedUpload("test-file.txt", 1024*1024, 4*1024*1024)
	require.NoError(t, err)

	assert.Equal(t, "upload-123", response.UploadID)
	assert.Equal(t, int64(4*1024*1024), response.ChunkSize)
	assert.Equal(t, 1, response.TotalChunks)
	assert.Empty(t, response.UploadedChunks)
}

func TestClientInitChunkedUploadWithoutChunkSize(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		err := r.ParseMultipartForm(32 << 20)
		require.NoError(t, err)

		assert.Empty(t, r.FormValue("chunk_size"))

		response := ChunkedUploadInitResponse{
			UploadID:       "upload-456",
			ChunkSize:      4 * 1024 * 1024,
			TotalChunks:    2,
			UploadedChunks: []int{},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := NewClient(server.URL)

	response, err := client.InitChunkedUpload("test-file.txt", 1024*1024, 0)
	require.NoError(t, err)

	assert.Equal(t, "upload-456", response.UploadID)
	assert.Equal(t, int64(4*1024*1024), response.ChunkSize)
	assert.Equal(t, 2, response.TotalChunks)
}

func TestClientUploadChunk(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Contains(t, r.URL.Path, "/upload/chunk/upload-123/0")

		err := r.ParseMultipartForm(32 << 20)
		require.NoError(t, err)

		file, header, err := r.FormFile("chunk")
		require.NoError(t, err)
		defer file.Close()

		assert.Equal(t, "chunk_0", header.Filename)

		response := ChunkedUploadCompleteResponse{
			Message:       "Upload completed",
			Progress:      100,
			FileURL:       "http://example.com/test.txt",
			MD5:           "test-md5-hash",
			Token:         "test-token",
			ExpiresAt:     "2023-12-31T23:59:59Z",
			ExpiresInDays: 30,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := NewClient(server.URL)
	chunkData := []byte("test chunk data")

	response, err := client.UploadChunk("upload-123", 0, chunkData)
	require.NoError(t, err)

	assert.Equal(t, "Upload completed", response.Message)
	assert.Equal(t, 100, response.Progress)
	assert.Equal(t, "http://example.com/test.txt", response.FileURL)
	assert.Equal(t, "test-md5-hash", response.MD5)
	assert.Equal(t, "test-token", response.Token)
}

func TestClientUploadChunkNotComplete(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status": "chunk uploaded"}`))
	}))
	defer server.Close()

	client := NewClient(server.URL)
	chunkData := []byte("test chunk data")

	response, err := client.UploadChunk("upload-123", 0, chunkData)
	require.NoError(t, err)
	assert.Nil(t, response)
}

func TestClientGetChunkedUploadStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/upload/status/upload-123", r.URL.Path)

		response := ChunkedUploadStatusResponse{
			Progress:       50,
			UploadedChunks: []int{0, 1, 2},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := NewClient(server.URL)

	response, err := client.GetChunkedUploadStatus("upload-123")
	require.NoError(t, err)

	assert.Equal(t, 50, response.Progress)
	assert.Equal(t, []int{0, 1, 2}, response.UploadedChunks)
}

func TestClientGetChunkedUploadStatusWithError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("Upload not found"))
	}))
	defer server.Close()

	client := NewClient(server.URL)

	response, err := client.GetChunkedUploadStatus("non-existent-upload")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "status check failed with status 404")
	assert.Nil(t, response)
}

func TestVerifyMD5(t *testing.T) {
	result := verifyMD5("d41d8cd98f00b204e9800998ecf8427e", "d41d8cd98f00b204e9800998ecf8427e")
	assert.True(t, result)

	result = verifyMD5("d41d8cd98f00b204e9800998ecf8427e", "5d41402abc4b2a76b9719d911017c592")
	assert.False(t, result)

	result = verifyMD5("", "")
	assert.True(t, result)

	result = verifyMD5("", "d41d8cd98f00b204e9800998ecf8427e")
	assert.False(t, result)
}

func TestFormatExpirationDate(t *testing.T) {
	result := formatExpirationDate("2023-12-31T23:59:59Z")
	assert.Contains(t, result, "Dec 31, 2023")

	result = formatExpirationDate("")
	assert.Equal(t, "Never", result)

	result = formatExpirationDate("invalid-date")
	assert.Equal(t, "invalid-date", result)
}

func TestFormatDaysRemaining(t *testing.T) {
	result := formatDaysRemaining(30)
	assert.Equal(t, "1 month", result)

	result = formatDaysRemaining(1)
	assert.Equal(t, "1 day", result)

	result = formatDaysRemaining(0)
	assert.Equal(t, "expired", result)

	result = formatDaysRemaining(-5)
	assert.Equal(t, "expired", result)
}
