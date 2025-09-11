package migration

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/marianozunino/drop/internal/model"
)

// CheckIfJSONMigrationNeeded checks if there are records that need JSON migration
func CheckIfJSONMigrationNeeded(dbPath string) (bool, int, error) {
	// Open a connection to check
	db, err := sql.Open("sqlite3", dbPath+"?_busy_timeout=5000")
	if err != nil {
		return false, 0, fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM metadata WHERE file_path IS NULL").Scan(&count)
	if err != nil {
		return false, 0, fmt.Errorf("failed to check for JSON data: %w", err)
	}

	return count > 0, count, nil
}

// CheckIfJSONMigrationNeededWithDB checks if there are records that need JSON migration using existing DB connection
func CheckIfJSONMigrationNeededWithDB(db *sql.DB) (bool, int, error) {
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM metadata WHERE file_path IS NULL").Scan(&count)
	if err != nil {
		return false, 0, fmt.Errorf("failed to check for JSON data: %w", err)
	}

	return count > 0, count, nil
}

// MigrateJSONDataWithDB migrates data using the existing database connection
func MigrateJSONDataWithDB(db *sql.DB) error {
	log.Println("Migrating JSON data to structured columns...")

	// Check if there are any records with NULL file_path (indicating old JSON data)
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM metadata WHERE file_path IS NULL").Scan(&count)
	if err != nil {
		return fmt.Errorf("failed to check for JSON data: %w", err)
	}

	if count == 0 {
		log.Println("No JSON data to migrate")
		return nil
	}

	log.Printf("Found %d records with JSON data to migrate", count)

	// Get all records with NULL file_path
	rows, err := db.Query("SELECT id, data FROM metadata WHERE file_path IS NULL")
	if err != nil {
		return fmt.Errorf("failed to query JSON data: %w", err)
	}
	defer rows.Close()

	// Process each record
	for rows.Next() {
		var id, data string
		if err := rows.Scan(&id, &data); err != nil {
			log.Printf("Failed to scan record: %v", err)
			continue
		}

		log.Printf("Processing record %s...", id)

		// Parse JSON data
		var metadata model.FileMetadata
		if err := json.Unmarshal([]byte(data), &metadata); err != nil {
			log.Printf("Failed to parse JSON for %s: %v", id, err)
			continue
		}

		log.Printf("Parsed JSON for %s, updating database...", id)

		// Update the record with structured data
		if err := updateRecordWithRetry(db, id, metadata); err != nil {
			log.Printf("Failed to update record %s: %v", id, err)
			continue
		}

		log.Printf("Successfully migrated record %s", id)
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("error iterating over records: %w", err)
	}

	log.Println("JSON data migration completed successfully")
	return nil
}

// MigrateJSONData migrates data from the old JSON-based schema to the new structured schema
// This function should be called after running the schema migration
func MigrateJSONData(dbPath string) error {
	log.Println("Migrating JSON data to structured columns...")

	// Open a new connection for the migration to avoid locking issues
	migrationDB, err := sql.Open("sqlite3", dbPath+"?_busy_timeout=30000&_journal_mode=WAL&_synchronous=NORMAL")
	if err != nil {
		return fmt.Errorf("failed to open migration database connection: %w", err)
	}
	defer migrationDB.Close()

	// Set connection pool settings
	migrationDB.SetMaxOpenConns(1)
	migrationDB.SetMaxIdleConns(1)
	migrationDB.SetConnMaxLifetime(0)

	// Check if there are any records with NULL file_path (indicating old JSON data)
	var count int
	err = migrationDB.QueryRow("SELECT COUNT(*) FROM metadata WHERE file_path IS NULL").Scan(&count)
	if err != nil {
		return fmt.Errorf("failed to check for JSON data: %w", err)
	}

	if count == 0 {
		log.Println("No JSON data to migrate")
		return nil
	}

	log.Printf("Found %d records with JSON data to migrate", count)

	// Migrate existing data from JSON to individual columns
	rows, err := migrationDB.Query("SELECT id, data FROM metadata WHERE file_path IS NULL")
	if err != nil {
		return fmt.Errorf("failed to query JSON data: %w", err)
	}
	defer rows.Close()

	migratedCount := 0
	for rows.Next() {
		var id, data string
		err := rows.Scan(&id, &data)
		if err != nil {
			log.Printf("Failed to scan row: %v", err)
			continue
		}

		log.Printf("Processing record %s...", id)

		// Parse JSON data
		var metadata model.FileMetadata
		err = json.Unmarshal([]byte(data), &metadata)
		if err != nil {
			log.Printf("Failed to parse JSON for record %s: %v", id, err)
			continue
		}

		log.Printf("Parsed JSON for %s, updating database...", id)

		// Update the record with individual columns (with retry logic)
		if err := updateRecordWithRetry(migrationDB, id, metadata); err != nil {
			log.Printf("Failed to update record %s after retries: %v", id, err)
			continue
		}

		log.Printf("Successfully updated record %s", id)
		migratedCount++
	}

	log.Printf("Successfully migrated %d records from JSON to structured format", migratedCount)
	return nil
}

// updateRecordWithRetry attempts to update a record with retry logic for database locks
func updateRecordWithRetry(db *sql.DB, id string, metadata model.FileMetadata) error {
	maxRetries := 5
	retryDelay := 500 * time.Millisecond

	for attempt := 0; attempt < maxRetries; attempt++ {
		log.Printf("Attempt %d/%d to update record %s", attempt+1, maxRetries, id)

		stmt, err := db.Prepare(`
			UPDATE metadata SET 
				file_path = ?, token = ?, original_name = ?, 
				upload_date = ?, expires_at = ?, size = ?, 
				content_type = ?, one_time_view = ?
			WHERE id = ?
		`)
		if err != nil {
			log.Printf("Failed to prepare statement for %s (attempt %d): %v", id, attempt+1, err)
			if attempt == maxRetries-1 {
				return fmt.Errorf("failed to prepare update statement: %w", err)
			}
			time.Sleep(retryDelay)
			continue
		}

		log.Printf("Executing UPDATE for %s (attempt %d)", id, attempt+1)
		_, err = stmt.Exec(
			metadata.FilePath,
			metadata.Token,
			metadata.OriginalName,
			metadata.UploadDate,
			metadata.ExpiresAt,
			metadata.Size,
			metadata.ContentType,
			metadata.OneTimeView,
			id,
		)
		stmt.Close()

		if err != nil {
			log.Printf("UPDATE failed for %s (attempt %d): %v", id, attempt+1, err)
			if attempt == maxRetries-1 {
				return fmt.Errorf("failed to update record after %d attempts: %w", maxRetries, err)
			}
			time.Sleep(retryDelay)
			retryDelay *= 2 // Exponential backoff
			continue
		}

		log.Printf("UPDATE successful for %s", id)
		return nil // Success
	}

	return fmt.Errorf("failed to update record after all retries")
}
