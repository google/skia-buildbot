// Package cockroachdb loads SQL migrations as an http.FileSystem.
package cockroachdb

import (
	"net/http"
	"os"

	rice "github.com/GeertJohan/go.rice"
	"go.skia.org/infra/go/repo_root"
	"go.skia.org/infra/go/skerr"
)

// New returns an http.FileSystem with all the migrations for a cockroachdb database.
func New() (http.FileSystem, error) {
	// For some reason, go.rice's path resolution algorithm does not work under "bazel test ..."
	// unless we change into the workspace's root directory.
	workspaceRoot, err := repo_root.Get()
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	if err := os.Chdir(workspaceRoot); err != nil {
		return nil, skerr.Wrap(err)
	}

	conf := rice.Config{
		LocateOrder: []rice.LocateMethod{rice.LocateFS, rice.LocateEmbedded},
	}
	// Directory is infra/perf/migrations.
	box, err := conf.FindBox("../../../../migrations/cockroachdb")
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return box.HTTPBox(), nil
}
