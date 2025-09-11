package utils

import (
	"crypto/md5"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
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
	if hours, err := strconv.Atoi(expiresStr); err == nil && hours < 10000 {
		return time.Now().Add(time.Duration(hours) * time.Hour), nil
	}

	if ms, err := strconv.ParseInt(expiresStr, 10, 64); err == nil {
		return time.UnixMilli(ms).UTC(), nil
	}

	formats := []string{
		time.RFC3339,
		"2006-01-02",
		"2006-01-02T15:04:05",
		"2006-01-02T15:04", // HTML datetime-local format
		"2006-01-02 15:04:05",
	}
	for _, format := range formats {
		if t, err := time.Parse(format, expiresStr); err == nil {
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("unrecognized date/time format")
}

// CalculateMD5 calculates the MD5 hash of a file and returns it as a hexadecimal string
func CalculateMD5(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	hash := md5.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}

// TableRow represents a single row in an ASCII table
type TableRow struct {
	Fields []string
}

// GenerateASCIITable creates a formatted ASCII table from headers and rows
func GenerateASCIITable(headers []string, rows []TableRow) string {
	if len(headers) == 0 {
		return ""
	}

	// Calculate column widths based on headers and content
	colWidths := make([]int, len(headers))
	for i, header := range headers {
		colWidths[i] = len(header)
	}

	// Check all rows to find maximum width for each column
	for _, row := range rows {
		for i, field := range row.Fields {
			if i < len(colWidths) && len(field) > colWidths[i] {
				colWidths[i] = len(field)
			}
		}
	}

	// Build the table
	var result string

	// Top border
	result += "┌"
	for i, width := range colWidths {
		result += strings.Repeat("─", width+2)
		if i < len(colWidths)-1 {
			result += "┬"
		}
	}
	result += "┐\n"

	// Headers
	result += "│"
	for i, header := range headers {
		result += " " + header + strings.Repeat(" ", colWidths[i]-len(header)+1) + "│"
	}
	result += "\n"

	// Separator line
	result += "╞"
	for i, width := range colWidths {
		result += strings.Repeat("═", width+2)
		if i < len(colWidths)-1 {
			result += "╪"
		}
	}
	result += "╡\n"

	// Data rows
	for i, row := range rows {
		result += "│"
		for j, field := range row.Fields {
			if j < len(colWidths) {
				result += " " + field + strings.Repeat(" ", colWidths[j]-len(field)+1) + "│"
			}
		}
		result += "\n"

		// Add separator line between rows (except after the last row)
		if i < len(rows)-1 {
			result += "├"
			for j, width := range colWidths {
				result += strings.Repeat("─", width+2)
				if j < len(colWidths)-1 {
					result += "┼"
				}
			}
			result += "┤\n"
		}
	}

	// Bottom border
	result += "└"
	for i, width := range colWidths {
		result += strings.Repeat("─", width+2)
		if i < len(colWidths)-1 {
			result += "┴"
		}
	}
	result += "┘"

	return result
}
