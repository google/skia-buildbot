//go:generate statik -src=../../../../migrations/cockroachdb
//go:generate gofmt -s -w .
package cockroachdb

import (
	"net/http"

	"github.com/rakyll/statik/fs"

	_ "go.skia.org/infra/perf/go/sql/migrations/cockroachdb/statik" // Import the embedded directory.
)

// New returns an http.FileSystem with all the migrations for a cockroachdb database.
func New() (http.FileSystem, error) {
	return fs.New()
}
