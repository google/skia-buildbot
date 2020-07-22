package migrations

import (
	"net/http"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/cockroachdb" // For support of connection URLs that start with "cochroach://".
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/golang-migrate/migrate/v4/source/httpfs"
	"go.skia.org/infra/go/skerr"
)

// Up upgrades the schema of the database at the given connectionString
// based on the migrations stored in migrationsDir.
func Up(httpFileSystem http.FileSystem, connectionString string) error {
	source, err := httpfs.New(httpFileSystem, "/")
	if err != nil {
		return skerr.Wrap(err)
	}
	m, err := migrate.NewWithSourceInstance("httpfs", source, connectionString)
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
func Down(httpFileSystem http.FileSystem, connectionString string) error {
	source, err := httpfs.New(httpFileSystem, "/")
	if err != nil {
		return skerr.Wrap(err)
	}
	m, err := migrate.NewWithSourceInstance("httpfs", source, connectionString)
	if err != nil {
		return skerr.Wrap(err)
	}
	return m.Down()
}
