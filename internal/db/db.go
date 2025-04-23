package db

import (
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/marianozunino/drop/internal/config"
	"github.com/marianozunino/drop/internal/model"
	_ "github.com/mattn/go-sqlite3"
)

type DB struct {
	*sql.DB
}

type Storeable interface {
	ID() string
}

// NewDB creates a new SQLite database connection
func NewDB(config *config.Config) (*DB, error) {
	// Create a new SQLite database connection
	db, err := sql.Open("sqlite3", config.SQLitePath)
	if err != nil {
		return nil, err
	}

	// Test connection
	if err := db.Ping(); err != nil {
		return nil, err
	}

	// Create table if it doesn't exist
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS metadata (
			id TEXT PRIMARY KEY,
			data TEXT NOT NULL
		)
	`)
	if err != nil {
		return nil, err
	}

	return &DB{db}, nil
}

// Close closes the database connection
func (db *DB) Close() error {
	return db.DB.Close()
}

// StoreMetadata stores metadata in SQLite
func (db *DB) StoreMetadata(metadata Storeable) error {
	// Serialize metadata to JSON
	value, err := json.Marshal(metadata)
	if err != nil {
		return err
	}

	// Use prepared statement to prevent SQL injection
	stmt, err := db.Prepare(`
		INSERT OR REPLACE INTO metadata (id, data) VALUES (?, ?)
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	// Execute statement
	_, err = stmt.Exec(metadata.ID(), string(value))
	return err
}

// GetMetadataByID retrieves metadata from SQLite
func (db *DB) GetMetadataByID(ID string) (model.FileMetadata, error) {
	var metadata model.FileMetadata
	var data string

	err := db.QueryRow("SELECT data FROM metadata WHERE id = ?", ID).Scan(&data)
	if err != nil {
		if err == sql.ErrNoRows {
			return metadata, fmt.Errorf("no metadata found with ID: %s", ID)
		}
		return metadata, err
	}

	err = json.Unmarshal([]byte(data), &metadata)
	return metadata, err
}

// ListAllMetadata lists all metadata
func (db *DB) ListAllMetadata() ([]model.FileMetadata, error) {
	var metadataList []model.FileMetadata

	rows, err := db.Query("SELECT data FROM metadata")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var data string
		err := rows.Scan(&data)
		if err != nil {
			return nil, err
		}

		var metadata model.FileMetadata
		err = json.Unmarshal([]byte(data), &metadata)
		if err != nil {
			return nil, err
		}

		metadataList = append(metadataList, metadata)
	}

	return metadataList, rows.Err()
}

// DeleteMetadata deletes metadata
func (db *DB) DeleteMetadata(meta Storeable) error {
	stmt, err := db.Prepare("DELETE FROM metadata WHERE id = ?")
	if err != nil {
		return err
	}
	defer stmt.Close()

	_, err = stmt.Exec(meta.ID())
	return err
}
