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

func ParseExpirationTime(expiresStr string) (time.Time, error) {
	if hours, err := strconv.Atoi(expiresStr); err == nil {
		return time.Now().Add(time.Duration(hours) * time.Hour), nil
	}

	if ms, err := strconv.ParseInt(expiresStr, 10, 64); err == nil {
		return time.Unix(0, ms*int64(time.Millisecond)), nil
	}

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
