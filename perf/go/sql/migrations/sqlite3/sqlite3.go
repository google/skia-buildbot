//go:generate statik -src=../../../../migrations/sqlite3
package sqlite3

import (
	"net/http"

	"github.com/rakyll/statik/fs"

	_ "go.skia.org/infra/perf/go/sql/migrations/sqlite/statik"
)

func New() (http.FileSystem, error) {
	return fs.New()
}
