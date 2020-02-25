package migrations

import (
	"path/filepath"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/cockroachdb"
	_ "github.com/golang-migrate/migrate/v4/database/sqlite3"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

// sourceFromDirectory takes a source directory and prepends file:// to it so it
// looks like a URL that the migrate library expects.
func sourceFromDirectory(dir string) (string, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	return "file://" + abs, nil
}

// Up upgrades the schema of the database at the given connectionString
// based on the migrations stored in migrationsDir.
func Up(migrationsDir, connectionString string) error {
	source, err := sourceFromDirectory(migrationsDir)
	if err != nil {
		return err
	}
	m, err := migrate.New(source, connectionString)
	if err != nil {
		return err
	}
	return m.Up()
}

// Down reverses all the upgrades done in Up(). See Up() for more details.
func Down(migrationsDir, connectionString string) error {
	source, err := sourceFromDirectory(migrationsDir)
	if err != nil {
		return err
	}
	m, err := migrate.New(source, connectionString)
	if err != nil {
		return err
	}
	return m.Down()
}
