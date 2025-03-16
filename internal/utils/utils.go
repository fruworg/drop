package utils

import (
	"fmt"
	"strconv"
	"time"
)

// FormatFileSize converts bytes to human-readable format
func FormatFileSize(size int64) string {
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}
	div, exp := int64(unit), 0
	for n := size / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(size)/float64(div), "KMGTPE"[exp])
}

// ParseExpirationTime parses a string representing an expiration time and returns a time.Time object.
// It accepts the following formats:
//   - Integer number of hours to add to current time
//   - Unix timestamp in milliseconds
//   - RFC3339 formatted date-time string (e.g., "2006-01-02T15:04:05Z07:00")
//   - ISO date format (e.g., "2006-01-02")
//   - ISO datetime without timezone (e.g., "2006-01-02T15:04:05")
//   - SQL-like datetime format (e.g., "2006-01-02 15:04:05")
//
// Returns the parsed time.Time and nil error on success, or zero time.Time value and error on failure.
func ParseExpirationTime(expiresStr string) (time.Time, error) {
	// First try to parse as a simple integer for hours
	if hours, err := strconv.Atoi(expiresStr); err == nil && hours < 10000 {
		// If it's a small number (< 10000), interpret as hours
		return time.Now().Add(time.Duration(hours) * time.Hour), nil
	}

	// Next try as a millisecond timestamp (for large numbers)
	if ms, err := strconv.ParseInt(expiresStr, 10, 64); err == nil {
		return time.UnixMilli(ms).UTC(), nil
	}

	// Try string date formats
	formats := []string{
		time.RFC3339,
		"2006-01-02",
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
	}
	for _, format := range formats {
		if t, err := time.Parse(format, expiresStr); err == nil {
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("unrecognized date/time format")
}
