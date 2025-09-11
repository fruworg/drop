package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/sqlite3"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

func main() {
	var (
		dbPath  = flag.String("db", "./data/dump.db", "Database path")
		action  = flag.String("action", "up", "Migration action: up, down, force")
		steps   = flag.Int("steps", 0, "Number of steps to migrate (0 = all)")
		version = flag.Int("version", 0, "Version to force to")
	)
	flag.Parse()

	// Ensure database directory exists (only if not an absolute path)
	if !filepath.IsAbs(*dbPath) {
		if err := os.MkdirAll("./data", 0755); err != nil {
			log.Fatalf("Failed to create data directory: %v", err)
		}
	} else {
		// For absolute paths, ensure the parent directory exists
		parentDir := filepath.Dir(*dbPath)
		if err := os.MkdirAll(parentDir, 0755); err != nil {
			log.Fatalf("Failed to create database directory: %v", err)
		}
	}

	// Create database file if it doesn't exist
	if _, err := os.Stat(*dbPath); os.IsNotExist(err) {
		file, err := os.Create(*dbPath)
		if err != nil {
			log.Fatalf("Failed to create database file: %v", err)
		}
		file.Close()
	}

	// Create migrator
	m, err := migrate.New(
		"file://internal/migration/migrations",
		fmt.Sprintf("sqlite3://%s", *dbPath),
	)
	if err != nil {
		log.Fatalf("Failed to create migrator: %v", err)
	}
	defer m.Close()

	// Execute migration action
	switch *action {
	case "up":
		if *steps > 0 {
			err = m.Steps(*steps)
		} else {
			err = m.Up()
		}
		if err != nil && err != migrate.ErrNoChange {
			log.Fatalf("Migration failed: %v", err)
		}
		log.Println("Migrations completed successfully")

	case "down":
		if *steps > 0 {
			err = m.Steps(-*steps)
		} else {
			err = m.Down()
		}
		if err != nil && err != migrate.ErrNoChange {
			log.Fatalf("Migration failed: %v", err)
		}
		log.Println("Migrations rolled back successfully")

	case "force":
		if *version == 0 {
			log.Fatal("Version must be specified for force action")
		}
		err = m.Force(*version)
		if err != nil {
			log.Fatalf("Force migration failed: %v", err)
		}
		log.Printf("Database version forced to %d", *version)

	default:
		log.Fatalf("Unknown action: %s", *action)
	}
}
