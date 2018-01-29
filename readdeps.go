package main

import (
	"flag"
	"path/filepath"

	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/status/go/depsmap"
)

var (
	parentHash = flag.String("parent_hash", "HEAD", "Read DEPS file at this revision.")
	workdir    = flag.String("workdir", ".", "Working directory.")
)

func main() {
	common.Init()

	wd, err := filepath.Abs(*workdir)
	if err != nil {
		sklog.Fatal(err)
	}
	d, err := depsmap.New(common.REPO_SKIA_INTERNAL_TEST, wd)
	if err != nil {
		sklog.Fatal(err)
	}
	rev, err := d.Lookup(*parentHash, common.REPO_SKIA)
	if err != nil {
		sklog.Fatal(err)
	}
	sklog.Infof("Got: %s", rev)
}
