package db

import (
	"encoding/json"

	badger "github.com/dgraph-io/badger/v4"
	"github.com/marianozunino/drop/internal/config"
	"github.com/marianozunino/drop/internal/model"
)

type DB struct {
	*badger.DB
}

type Storeable interface {
	ID() string
}

// NewDB creates a new BadgerDB
func NewDB(config *config.Config) (*DB, error) {
	db, err := badger.Open(badger.DefaultOptions(config.BadgerPath))
	if err != nil {
		return nil, err
	}

	return &DB{db}, nil
}

// Close closes the BadgerDB
func (db *DB) Close() error {
	return db.DB.Close()
}

// StoreMetadata stores metadata in BadgerDB
func (db *DB) StoreMetadata(metadata Storeable) error {
	// Serialize metadata to JSON
	value, err := json.Marshal(metadata)
	if err != nil {
		return err
	}

	// Use token as key
	key := []byte(metadata.ID())

	// Write to BadgerDB
	return db.Update(func(txn *badger.Txn) error {
		return txn.Set(key, value)
	})
}

// GetMetadataByToken retrieves metadata from BadgerDB
func (db *DB) GetMetadataByID(ID string) (model.FileMetadata, error) {
	var metadata model.FileMetadata
	key := []byte(ID)

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

// ListAllMetadata lists all metadata
func (db *DB) ListAllMetadata() ([]model.FileMetadata, error) {
	var metadataList []model.FileMetadata

	err := db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()

		for it.Rewind(); it.Valid(); it.Next() {
			item := it.Item()
			var metadata model.FileMetadata

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

// DeleteMetadata deletes metadata
func (db *DB) DeleteMetadata(meta Storeable) any {
	return db.Update(func(txn *badger.Txn) error {
		return txn.Delete([]byte(meta.ID()))
	})
}
