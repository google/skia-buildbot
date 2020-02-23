package migrations

import (
	"path/filepath"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/cockroachdb"
	_ "github.com/golang-migrate/migrate/v4/database/sqlite3"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

func sourceFromDirectory(dir string) (string, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	return "file://" + abs, nil
}

// Source is:
// 		"cockroachdb://cockroach:@localhost:26257/example?sslmode=disable"
// or
//      "sqlite3:///tmp/test.db"
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
