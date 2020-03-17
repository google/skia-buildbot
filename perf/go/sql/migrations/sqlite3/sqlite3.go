//go:generate statik -src=../../../../migrations/sqlite3
//go:generate gofmt -s -w .
package sqlite3

import (
	"net/http"

	"github.com/rakyll/statik/fs"

	_ "go.skia.org/infra/perf/go/sql/migrations/sqlite3/statik" // Import the embedded directory.
)

// New returns an http.FileSystem with all the migrations for an sqlite3 database.
func New() (http.FileSystem, error) {
	return fs.New()
}
