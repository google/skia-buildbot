// Package dist loads the WebPack output as an http.FileSystem.
package dist

import (
	"net/http"

	rice "github.com/GeertJohan/go.rice"
	"go.skia.org/infra/go/skerr"
)

// New returns an http.FileSystem with all the source files in ./dist.
func New() (http.FileSystem, error) {
	conf := rice.Config{
		LocateOrder: []rice.LocateMethod{rice.LocateFS, rice.LocateEmbedded},
	}
	// Directory is infra/perf/dist.
	box, err := conf.FindBox("../../dist")
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return box.HTTPBox(), nil
}
