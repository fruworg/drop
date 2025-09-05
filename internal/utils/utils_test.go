package utils

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFormatFileSize(t *testing.T) {
	result := FormatFileSize(512)
	assert.Equal(t, "512 B", result)

	result = FormatFileSize(1024)
	assert.Equal(t, "1.0 KB", result)

	result = FormatFileSize(1536)
	assert.Equal(t, "1.5 KB", result)

	result = FormatFileSize(1024 * 1024)
	assert.Equal(t, "1.0 MB", result)

	result = FormatFileSize(2.5 * 1024 * 1024)
	assert.Equal(t, "2.5 MB", result)

	result = FormatFileSize(1024 * 1024 * 1024)
	assert.Equal(t, "1.0 GB", result)

	result = FormatFileSize(3972840000)
	assert.Equal(t, "3.7 GB", result)

	result = FormatFileSize(1024 * 1024 * 1024 * 1024)
	assert.Equal(t, "1.0 TB", result)

	result = FormatFileSize(1024 * 1024 * 1024 * 1024 * 1024)
	assert.Equal(t, "1.0 PB", result)

	result = FormatFileSize(1024 * 1024 * 1024 * 1024 * 1024 * 1024)
	assert.Equal(t, "1.0 EB", result)

	result = FormatFileSize(0)
	assert.Equal(t, "0 B", result)

	result = FormatFileSize(1024 * 1024 * 1024 * 1024 * 1024)
	assert.Equal(t, "1.0 PB", result)
}

func TestCalculateMD5(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "test.txt")

	content := "Hello, World! This is a test file for MD5 calculation."
	err := os.WriteFile(filePath, []byte(content), 0644)
	require.NoError(t, err)

	hash, err := CalculateMD5(filePath)
	require.NoError(t, err)

	assert.Len(t, hash, 32)
	assert.Regexp(t, "^[0-9a-f]+$", hash)

	emptyFilePath := filepath.Join(tempDir, "empty.txt")
	err = os.WriteFile(emptyFilePath, []byte(""), 0644)
	require.NoError(t, err)

	emptyHash, err := CalculateMD5(emptyFilePath)
	require.NoError(t, err)

	expectedEmptyHash := "d41d8cd98f00b204e9800998ecf8427e"
	assert.Equal(t, expectedEmptyHash, emptyHash)
}

func TestCalculateMD5WithNonExistentFile(t *testing.T) {
	hash, err := CalculateMD5("/non/existent/file.txt")
	assert.Error(t, err)
	assert.Empty(t, hash)
	assert.Contains(t, err.Error(), "failed to open file")
}

func TestCalculateMD5WithUnreadableFile(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "unreadable.txt")

	err := os.WriteFile(filePath, []byte("test content"), 0644)
	require.NoError(t, err)

	err = os.Chmod(filePath, 0000)
	require.NoError(t, err)

	hash, err := CalculateMD5(filePath)
	assert.Error(t, err)
	assert.Empty(t, hash)
	assert.Contains(t, err.Error(), "failed to open file")
}

func TestGenerateASCIITable(t *testing.T) {
	headers := []string{"Name", "Age", "City"}
	rows := []TableRow{
		{Fields: []string{"Alice", "25", "New York"}},
		{Fields: []string{"Bob", "30", "London"}},
		{Fields: []string{"Charlie", "35", "Tokyo"}},
	}

	result := GenerateASCIITable(headers, rows)

	assert.Contains(t, result, "Name")
	assert.Contains(t, result, "Age")
	assert.Contains(t, result, "City")
	assert.Contains(t, result, "Alice")
	assert.Contains(t, result, "Bob")
	assert.Contains(t, result, "Charlie")

	assert.Contains(t, result, "┌")
	assert.Contains(t, result, "┐")
	assert.Contains(t, result, "└")
	assert.Contains(t, result, "┘")
	assert.Contains(t, result, "│")
	assert.Contains(t, result, "─")
	assert.Contains(t, result, "┬")
	assert.Contains(t, result, "┴")
	assert.Contains(t, result, "╞")
	assert.Contains(t, result, "╡")
	assert.Contains(t, result, "═")
	assert.Contains(t, result, "╪")
	assert.Contains(t, result, "├")
	assert.Contains(t, result, "┤")
	assert.Contains(t, result, "┼")
}

func TestGenerateASCIITableWithEmptyHeaders(t *testing.T) {
	result := GenerateASCIITable([]string{}, []TableRow{})
	assert.Empty(t, result)
}

func TestGenerateASCIITableWithEmptyRows(t *testing.T) {
	headers := []string{"Name", "Age"}
	rows := []TableRow{}

	result := GenerateASCIITable(headers, rows)

	assert.Contains(t, result, "Name")
	assert.Contains(t, result, "Age")
	assert.Contains(t, result, "┌")
	assert.Contains(t, result, "┐")
	assert.Contains(t, result, "└")
	assert.Contains(t, result, "┘")
}

func TestGenerateASCIITableWithVaryingColumnWidths(t *testing.T) {
	headers := []string{"Short", "Very Long Column Name", "Medium"}
	rows := []TableRow{
		{Fields: []string{"A", "This is a very long value that should expand the column", "B"}},
		{Fields: []string{"C", "Short", "This is also quite long"}},
	}

	result := GenerateASCIITable(headers, rows)

	assert.Contains(t, result, "Short")
	assert.Contains(t, result, "Very Long Column Name")
	assert.Contains(t, result, "Medium")
	assert.Contains(t, result, "A")
	assert.Contains(t, result, "This is a very long value that should expand the column")
	assert.Contains(t, result, "B")
	assert.Contains(t, result, "C")
	assert.Contains(t, result, "This is also quite long")
}

func TestGenerateASCIITableWithMoreFieldsThanHeaders(t *testing.T) {
	headers := []string{"Name", "Age"}
	rows := []TableRow{
		{Fields: []string{"Alice", "25", "Extra Field"}},
	}

	result := GenerateASCIITable(headers, rows)

	assert.Contains(t, result, "Alice")
	assert.Contains(t, result, "25")
	assert.NotContains(t, result, "Extra Field")
}

func TestGenerateASCIITableWithFewerFieldsThanHeaders(t *testing.T) {
	headers := []string{"Name", "Age", "City"}
	rows := []TableRow{
		{Fields: []string{"Alice"}},
	}

	result := GenerateASCIITable(headers, rows)

	assert.Contains(t, result, "Alice")
	assert.Contains(t, result, "Name")
	assert.Contains(t, result, "Age")
	assert.Contains(t, result, "City")
}

func TestGenerateASCIITableSingleRow(t *testing.T) {
	headers := []string{"ID", "Status"}
	rows := []TableRow{
		{Fields: []string{"1", "Active"}},
	}

	result := GenerateASCIITable(headers, rows)

	assert.Contains(t, result, "ID")
	assert.Contains(t, result, "Status")
	assert.Contains(t, result, "1")
	assert.Contains(t, result, "Active")
	assert.Contains(t, result, "┌")
	assert.Contains(t, result, "┐")
	assert.Contains(t, result, "└")
	assert.Contains(t, result, "┘")
	assert.NotContains(t, result, "├")
	assert.NotContains(t, result, "┤")
	assert.NotContains(t, result, "┼")
}
