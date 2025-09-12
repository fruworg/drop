package testutil

import (
	"fmt"
	"path/filepath"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/sqlite3"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

// RunTestMigrations runs database migrations for tests
func RunTestMigrations(dbPath string) error {
	// Get the absolute path to the migrations directory from the project root
	// We need to go up from the test directory to the project root
	projectRoot, err := filepath.Abs("../..")
	if err != nil {
		return err
	}
	migrationsPath := filepath.Join(projectRoot, "internal", "migration", "migrations")

	// Create migrator
	m, err := migrate.New(
		fmt.Sprintf("file://%s", migrationsPath),
		fmt.Sprintf("sqlite3://%s", dbPath),
	)
	if err != nil {
		return err
	}
	defer m.Close()

	// Run migrations
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return err
	}

	return nil
}

// RunTestMigrationsFromProjectRoot runs database migrations for tests from project root
func RunTestMigrationsFromProjectRoot(dbPath string) error {
	// Get the absolute path to the migrations directory from the project root
	projectRoot, err := filepath.Abs(".")
	if err != nil {
		return err
	}
	migrationsPath := filepath.Join(projectRoot, "internal", "migration", "migrations")

	// Create migrator
	m, err := migrate.New(
		fmt.Sprintf("file://%s", migrationsPath),
		fmt.Sprintf("sqlite3://%s", dbPath),
	)
	if err != nil {
		return err
	}
	defer m.Close()

	// Run migrations
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return err
	}

	return nil
}
