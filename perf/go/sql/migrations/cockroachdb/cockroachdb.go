// Package cockroachdb loads SQL migrations as an http.FileSystem.
package cockroachdb

import (
	"net/http"
	"os"
	"path/filepath"

	"go.skia.org/infra/go/repo_root"
	"go.skia.org/infra/go/skerr"
)

// New returns an http.FileSystem with all the migrations for a cockroachdb database.
func New() (http.FileSystem, error) {
	// For some reason, the path resolution does not work under "bazel test ..."
	// unless we change into the workspace's root directory.
	workspaceRoot, err := repo_root.Get()
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	if err := os.Chdir(workspaceRoot); err != nil {
		return nil, skerr.Wrap(err)
	}
	// Directory is infra/perf/migrations.
	return http.Dir(filepath.Join(workspaceRoot, "perf/migrations/cockroachdb")), nil
}
