//go:generate statik -src=../../../../migrations/sqlite
package sqlite

import (
	"net/http"

	"github.com/rakyll/statik/fs"

	_ "go.skia.org/infra/perf/go/sql/migrations/sqlite/statik"
)

func New() (http.FileSystem, error) {
	return fs.New()
}
