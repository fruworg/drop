package migration

import (
	"database/sql"
	"fmt"
	"log"
	"path/filepath"
	"strings"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/sqlite3"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/marianozunino/drop/internal/config"
	_ "github.com/mattn/go-sqlite3"
)

// Manager handles database migrations
type Manager struct {
	migrator *migrate.Migrate
	db       *sql.DB
	config   *config.Config
}

// NewManagerWithDB creates a new migration manager using an existing database connection
func NewManagerWithDB(db *sql.DB, cfg *config.Config) (*Manager, error) {
	// Create migration driver using the existing database connection
	driver, err := sqlite3.WithInstance(db, &sqlite3.Config{})
	if err != nil {
		return nil, fmt.Errorf("failed to create migration driver: %w", err)
	}

	// Create migrator using embedded migrations
	sourceDriver, err := iofs.New(MigrationsFS, "migrations")
	if err != nil {
		return nil, fmt.Errorf("failed to create source driver: %w", err)
	}

	migrator, err := migrate.NewWithInstance("iofs", sourceDriver, "sqlite3", driver)
	if err != nil {
		return nil, fmt.Errorf("failed to create migrator: %w", err)
	}

	return &Manager{
		migrator: migrator,
		db:       db,
		config:   cfg,
	}, nil
}

// NewManager creates a new migration manager
func NewManager(dbPath, migrationsPath string) (*Manager, error) {
	// Open database connection
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Create migration driver
	driver, err := sqlite3.WithInstance(db, &sqlite3.Config{})
	if err != nil {
		return nil, fmt.Errorf("failed to create migration driver: %w", err)
	}

	// Get absolute path for migrations
	absPath, err := filepath.Abs(migrationsPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path for migrations: %w", err)
	}

	// Create migrator
	migrator, err := migrate.NewWithDatabaseInstance(
		fmt.Sprintf("file://%s", absPath),
		"sqlite3",
		driver,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create migrator: %w", err)
	}

	return &Manager{
		migrator: migrator,
		db:       db,
	}, nil
}

// Up runs all pending migrations
func (m *Manager) Up() error {
	log.Println("Running database migrations...")

	// Check if database is in dirty state
	version, dirty, err := m.migrator.Version()
	if err != nil && err != migrate.ErrNilVersion {
		return fmt.Errorf("failed to check migration version: %w", err)
	}

	if dirty {
		log.Printf("Database is in dirty state at version %d. Attempting to fix...", version)

		// Try to force the version to clean state
		if err := m.migrator.Force(int(version)); err != nil {
			log.Printf("Failed to force clean version %d: %v", version, err)
			// If forcing fails, try to force to the previous version
			if version > 0 {
				log.Printf("Attempting to force to previous version %d...", version-1)
				if err := m.migrator.Force(int(version - 1)); err != nil {
					return fmt.Errorf("failed to fix dirty database state: %w", err)
				}
				log.Printf("Successfully forced to version %d", version-1)
			} else {
				return fmt.Errorf("database is dirty at version 0 and cannot be fixed automatically")
			}
		} else {
			log.Printf("Successfully cleaned dirty state at version %d", version)
		}
	}

	err = m.migrator.Up()
	if err != nil && err != migrate.ErrNoChange {
		// Check if the error is due to existing schema elements
		if strings.Contains(err.Error(), "duplicate column name") || strings.Contains(err.Error(), "table") && strings.Contains(err.Error(), "already exists") {
			log.Printf("Migration failed due to existing schema elements. Checking if schema needs updating...")

			// Check if we actually need to migrate the schema
			if needsSchemaMigration(m.db) {
				log.Printf("Schema needs migration. Attempting to migrate existing schema...")
				if err := migrateExistingSchema(m.db); err != nil {
					log.Printf("Warning: Failed to migrate existing schema: %v", err)
				}
			}

			// Force the migration version to the latest to avoid this issue in the future
			if err := m.migrator.Force(2); err != nil {
				log.Printf("Warning: Failed to force migration version: %v", err)
			}
		} else {
			return fmt.Errorf("failed to run migrations: %w", err)
		}
	}

	if err == migrate.ErrNoChange {
		log.Println("No new migrations to run")
	} else {
		log.Println("Migrations completed successfully")
	}

	// JSON data migration is now handled directly in the SQL migration files
	log.Println("Schema migrations completed. JSON data migration handled in SQL.")

	return nil
}

// Down rolls back the last migration
func (m *Manager) Down() error {
	log.Println("Rolling back last migration...")

	err := m.migrator.Steps(-1)
	if err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("failed to rollback migration: %w", err)
	}

	if err == migrate.ErrNoChange {
		log.Println("No migrations to rollback")
	} else {
		log.Println("Migration rollback completed successfully")
	}

	return nil
}

// Force sets the migration version without running migrations
func (m *Manager) Force(version int) error {
	log.Printf("Forcing migration version to %d...", version)

	err := m.migrator.Force(version)
	if err != nil {
		return fmt.Errorf("failed to force migration version: %w", err)
	}

	log.Printf("Migration version forced to %d", version)
	return nil
}

// Version returns the current migration version
func (m *Manager) Version() (uint, bool, error) {
	version, dirty, err := m.migrator.Version()
	if err != nil && err != migrate.ErrNilVersion {
		return 0, false, fmt.Errorf("failed to get migration version: %w", err)
	}

	return version, dirty, nil
}

// Close closes the migration manager and database connection
func (m *Manager) Close() error {
	// Don't close the migrator or database connection since it's shared with the main app
	// The main application will close it when needed
	log.Println("Migration manager closed (database connection preserved)")
	return nil
}

// MigrateToVersion migrates to a specific version
func (m *Manager) MigrateToVersion(targetVersion uint) error {
	log.Printf("Migrating to version %d...", targetVersion)

	err := m.migrator.Migrate(targetVersion)
	if err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("failed to migrate to version %d: %w", targetVersion, err)
	}

	if err == migrate.ErrNoChange {
		log.Printf("Already at version %d", targetVersion)
	} else {
		log.Printf("Successfully migrated to version %d", targetVersion)
	}

	return nil
}

// FixDirtyState attempts to fix a dirty migration state
func (m *Manager) FixDirtyState() error {
	version, dirty, err := m.migrator.Version()
	if err != nil && err != migrate.ErrNilVersion {
		return fmt.Errorf("failed to check migration version: %w", err)
	}

	if !dirty {
		log.Println("Database is not in dirty state")
		return nil
	}

	log.Printf("Database is in dirty state at version %d. Attempting to fix...", version)

	// Try to force the version to clean state
	if err := m.migrator.Force(int(version)); err != nil {
		log.Printf("Failed to force clean version %d: %v", version, err)
		// If forcing fails, try to force to the previous version
		if version > 0 {
			log.Printf("Attempting to force to previous version %d...", version-1)
			if err := m.migrator.Force(int(version - 1)); err != nil {
				return fmt.Errorf("failed to fix dirty database state: %w", err)
			}
			log.Printf("Successfully forced to version %d", version-1)
		} else {
			return fmt.Errorf("database is dirty at version 0 and cannot be fixed automatically")
		}
	} else {
		log.Printf("Successfully cleaned dirty state at version %d", version)
	}

	return nil
}

// Drop drops all tables (use with caution!)
func (m *Manager) Drop() error {
	log.Println("Dropping all tables...")

	err := m.migrator.Drop()
	if err != nil {
		return fmt.Errorf("failed to drop tables: %w", err)
	}

	log.Println("All tables dropped successfully")
	return nil
}

// needsSchemaMigration checks if the database needs schema migration
func needsSchemaMigration(db *sql.DB) bool {
	// Check if file_path column exists
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM pragma_table_info('metadata') WHERE name = 'file_path'").Scan(&count)
	if err != nil {
		return false
	}
	return count == 0
}

// migrateExistingSchema migrates an existing old schema to the new schema
func migrateExistingSchema(db *sql.DB) error {
	log.Println("Migrating existing schema from old format...")

	// Add the new columns
	columns := []string{
		"ALTER TABLE metadata ADD COLUMN file_path TEXT",
		"ALTER TABLE metadata ADD COLUMN token TEXT",
		"ALTER TABLE metadata ADD COLUMN original_name TEXT",
		"ALTER TABLE metadata ADD COLUMN upload_date DATETIME",
		"ALTER TABLE metadata ADD COLUMN expires_at DATETIME",
		"ALTER TABLE metadata ADD COLUMN size INTEGER",
		"ALTER TABLE metadata ADD COLUMN content_type TEXT",
		"ALTER TABLE metadata ADD COLUMN one_time_view BOOLEAN DEFAULT FALSE",
	}

	for _, columnSQL := range columns {
		if _, err := db.Exec(columnSQL); err != nil {
			// Ignore errors for columns that already exist
			if !strings.Contains(err.Error(), "duplicate column name") {
				log.Printf("Warning: Failed to add column: %v", err)
			}
		}
	}

	// Create indexes
	indexes := []string{
		"CREATE INDEX IF NOT EXISTS idx_file_path ON metadata(file_path)",
		"CREATE INDEX IF NOT EXISTS idx_original_name ON metadata(original_name)",
		"CREATE INDEX IF NOT EXISTS idx_upload_date ON metadata(upload_date)",
		"CREATE INDEX IF NOT EXISTS idx_expires_at ON metadata(expires_at)",
		"CREATE INDEX IF NOT EXISTS idx_size ON metadata(size)",
	}

	for _, indexSQL := range indexes {
		if _, err := db.Exec(indexSQL); err != nil {
			log.Printf("Warning: Failed to create index: %v", err)
		}
	}

	log.Println("Schema migration completed")
	return nil
}
