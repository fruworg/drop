package handler

import (
	"fmt"
	"io"
	"time"
)

// SimpleProgressReader wraps an io.Reader and logs basic progress
type SimpleProgressReader struct {
	reader    io.Reader
	total     int64
	current   int64
	filename  string
	startTime time.Time
}

// NewSimpleProgressReader creates a new SimpleProgressReader
func NewSimpleProgressReader(reader io.Reader, total int64, filename string) *SimpleProgressReader {
	return &SimpleProgressReader{
		reader:    reader,
		total:     total,
		filename:  filename,
		startTime: time.Now(),
	}
}

// Read implements io.Reader and logs basic progress
func (pr *SimpleProgressReader) Read(p []byte) (int, error) {
	n, err := pr.reader.Read(p)
	if n > 0 {
		pr.current += int64(n)
	}

	return n, err
}

// formatBytes returns a human-readable byte count
func formatBytes(bytes int64) string {
	if bytes < 1024 {
		return fmt.Sprintf("%d B", bytes)
	}

	units := []string{"B", "KB", "MB", "GB", "TB"}
	size := float64(bytes)
	unitIndex := 0

	for size >= 1024 && unitIndex < len(units)-1 {
		size /= 1024
		unitIndex++
	}

	if size >= 10 {
		return fmt.Sprintf("%.1f %s", size, units[unitIndex])
	}
	return fmt.Sprintf("%.2f %s", size, units[unitIndex])
}
