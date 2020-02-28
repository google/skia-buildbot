package migrations

import (
	"path/filepath"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/cockroachdb" // For support of connection URLs that start with "cochroach://".
	_ "github.com/golang-migrate/migrate/v4/database/sqlite3"     // For support of connection URLs that start with "sqlite3://".
	_ "github.com/golang-migrate/migrate/v4/source/file"          // For support of source URLs that start with "file://".
	"go.skia.org/infra/go/skerr"
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
		return skerr.Wrap(err)
	}
	m, err := migrate.New(source, connectionString)
	if err != nil {
		return skerr.Wrap(err)
	}
	err = m.Up()
	// Don't report an error if the database is already at the right schema.
	if err != nil && err != migrate.ErrNoChange {
		return err
	}
	return nil
}

// Down reverses all the upgrades done in Up(). See Up() for more details.
func Down(migrationsDir, connectionString string) error {
	source, err := sourceFromDirectory(migrationsDir)
	if err != nil {
		return skerr.Wrap(err)
	}
	m, err := migrate.New(source, connectionString)
	if err != nil {
		return skerr.Wrap(err)
	}
	return m.Down()
}
