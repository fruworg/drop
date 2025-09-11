package migration

import (
	"embed"
	"io/fs"
)

//go:embed migrations/*.sql
var MigrationsFS embed.FS

// GetMigrationsFS returns the embedded filesystem containing migration files
func GetMigrationsFS() fs.FS {
	return MigrationsFS
}
