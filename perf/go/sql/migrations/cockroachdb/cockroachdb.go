// Package cockroachdb loads SQL migrations as an http.FileSystem.
package cockroachdb

import (
	"net/http"
	"path/filepath"
	"runtime"
)

// New returns an http.FileSystem with all the migrations for a cockroachdb database.
func New() (http.FileSystem, error) {
	// Directory is infra/perf/migrations.
	_, filename, _, _ := runtime.Caller(1)
	return http.Dir(filepath.Join(filepath.Dir(filename), "../../../migrations/cockroachdb")), nil
}
