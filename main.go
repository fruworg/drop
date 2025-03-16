package main

import (
	"encoding/json"
	"log"
	"time"

	badger "github.com/dgraph-io/badger/v4"
)

type FileMetadata struct {
	Token        string    `json:"token"`
	OriginalName string    `json:"original_name,omitempty"`
	UploadDate   time.Time `json:"upload_date"`
	ExpiresAt    time.Time `json:"expires_at,omitempty"`
	Size         int64     `json:"size"`
	ContentType  string    `json:"content_type,omitempty"`
}

func main() {
	// Open BadgerDB
	db, err := badger.Open(badger.DefaultOptions("/tmp/badger"))
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// Example metadata
	metadata := FileMetadata{
		Token:        "abc123",
		OriginalName: "document.pdf",
		UploadDate:   time.Now(),
		ExpiresAt:    time.Now().Add(24 * time.Hour),
		Size:         1024,
		ContentType:  "application/pdf",
	}

	// Store metadata
	err = storeMetadata(db, metadata)
	if err != nil {
		log.Fatal(err)
	}

	// Retrieve metadata
	retrievedMetadata, err := getMetadata(db, "abc123")
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("Retrieved metadata: %+v", retrievedMetadata)
}

// Store metadata in BadgerDB
func storeMetadata(db *badger.DB, metadata FileMetadata) error {
	// Serialize metadata to JSON
	value, err := json.Marshal(metadata)
	if err != nil {
		return err
	}

	// Use token as key
	key := []byte(metadata.Token)

	// Write to BadgerDB
	return db.Update(func(txn *badger.Txn) error {
		return txn.Set(key, value)
	})
}

// Retrieve metadata from BadgerDB
func getMetadata(db *badger.DB, token string) (FileMetadata, error) {
	var metadata FileMetadata
	key := []byte(token)

	err := db.View(func(txn *badger.Txn) error {
		item, err := txn.Get(key)
		if err != nil {
			return err
		}

		return item.Value(func(val []byte) error {
			return json.Unmarshal(val, &metadata)
		})
	})

	return metadata, err
}

// List all metadata
func listAllMetadata(db *badger.DB) ([]FileMetadata, error) {
	var metadataList []FileMetadata

	err := db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()

		for it.Rewind(); it.Valid(); it.Next() {
			item := it.Item()
			var metadata FileMetadata

			err := item.Value(func(val []byte) error {
				return json.Unmarshal(val, &metadata)
			})
			if err != nil {
				return err
			}

			metadataList = append(metadataList, metadata)
		}
		return nil
	})

	return metadataList, err
}
