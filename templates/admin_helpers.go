package templates

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/marianozunino/drop/internal/model"
)

// Helper functions for admin templates

func CountExpiredFiles(files []model.AdminFileInfo) int {
	count := 0
	for _, file := range files {
		if file.IsExpired {
			count++
		}
	}
	return count
}

func CountOneTimeFiles(files []model.AdminFileInfo) int {
	count := 0
	for _, file := range files {
		if file.OneTimeView {
			count++
		}
	}
	return count
}

func FormatTotalSize(files []model.AdminFileInfo) string {
	var totalSize int64
	for _, file := range files {
		totalSize += file.Size
	}
	return FormatBytes(totalSize)
}

func FormatGlobalTotalSize(totalSize int64) string {
	return FormatBytes(totalSize)
}

func FormatBytes(bytes int64) string {
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

func GetSortURL(field, currentSortField, currentSortDirection, searchQuery, currentCursor string, limit int) string {
	if field == currentSortField {
		// Toggle direction if clicking the same field
		newDirection := "asc"
		if currentSortDirection == "asc" {
			newDirection = "desc"
		}
		url := "/admin?sort=" + field + "&dir=" + newDirection + "&limit=" + strconv.Itoa(limit)
		if searchQuery != "" {
			url += "&search=" + searchQuery
		}
		// Reset cursor when changing sort direction
		return url
	}
	// Default to ascending for new field
	url := "/admin?sort=" + field + "&dir=asc&limit=" + strconv.Itoa(limit)
	if searchQuery != "" {
		url += "&search=" + searchQuery
	}
	// Reset cursor when changing sort field
	return url
}

func GetPaginationURL(sortField, sortDirection, searchQuery, cursor string, limit int) string {
	url := "/admin?sort=" + sortField + "&dir=" + sortDirection + "&limit=" + strconv.Itoa(limit)
	if searchQuery != "" {
		url += "&search=" + searchQuery
	}
	if cursor != "" {
		url += "&cursor=" + cursor
	}
	return url
}

func GetDeleteURL(filename, token, sortField, sortDirection, searchQuery string, limit int) string {
	url := "/admin/file/" + filename + "/delete"
	params := []string{}

	// Always include the token for security
	if token != "" {
		params = append(params, "token="+token)
	}

	if searchQuery != "" {
		params = append(params, "search="+searchQuery)
	}
	if sortField != "" {
		params = append(params, "sort="+sortField)
	}
	if sortDirection != "" {
		params = append(params, "dir="+sortDirection)
	}
	if limit > 0 {
		params = append(params, "limit="+strconv.Itoa(limit))
	}

	if len(params) > 0 {
		url += "?" + strings.Join(params, "&")
	}

	return url
}
