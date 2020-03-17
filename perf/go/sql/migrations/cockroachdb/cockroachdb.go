//go:generate statik -src=../../../../migrations/cockroachdb
package cockroachdb

import (
	"net/http"

	"github.com/rakyll/statik/fs"

	_ "go.skia.org/infra/perf/go/sql/migrations/cockroachdb/statik"
)

func New() (http.FileSystem, error) {
	return fs.New()
}
