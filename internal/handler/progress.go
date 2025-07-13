package handler

import (
	"fmt"
	"io"
	"log"
	"time"
)

// ProgressReader wraps an io.Reader and reports progress.
type ProgressReader struct {
	reader    io.Reader
	total     int64
	current   int64
	filename  string
	startTime time.Time

	lastPercent            *int
	lastChunk              *int64
	lastLoggedAtCompletion *bool
}

// NewProgressReader creates a new ProgressReader.
func NewProgressReader(reader io.Reader, total int64, filename string) *ProgressReader {
	return &ProgressReader{
		reader:    reader,
		total:     total,
		filename:  filename,
		startTime: time.Now(),
	}
}

// Read implements io.Reader and logs progress.
func (pr *ProgressReader) Read(p []byte) (int, error) {
	n, err := pr.reader.Read(p)
	if n <= 0 {
		return n, err
	}

	pr.current += int64(n)

	if pr.total > 0 {
		// Only log at start and at completion
		if pr.current == int64(n) && pr.lastPercent == nil {
			log.Printf("Upload started: %s (total %s)", pr.filename, formatBytes(pr.total))
			pr.lastPercent = new(int)
		}
		if pr.current == pr.total && (pr.lastLoggedAtCompletion == nil || !*pr.lastLoggedAtCompletion) {
			elapsed := time.Since(pr.startTime)
			speed := float64(pr.current) / elapsed.Seconds() / 1024 / 1024 // MB/s
			log.Printf("Upload completed: %s (%s) - %.2f MB/s", pr.filename, formatBytes(pr.current), speed)
			flag := true
			pr.lastLoggedAtCompletion = &flag
		}
	}

	return n, err
}

// formatBytes returns a human-readable byte count.
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
