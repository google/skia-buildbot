// Package cockroachdb loads SQL migrations as an http.FileSystem.
package cockroachdb

import (
	"net/http"

	rice "github.com/GeertJohan/go.rice"
	"go.skia.org/infra/go/skerr"
)

// New returns an http.FileSystem with all the migrations for a cockroachdb database.
func New() (http.FileSystem, error) {
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
