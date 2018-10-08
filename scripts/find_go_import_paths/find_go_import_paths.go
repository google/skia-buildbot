package main

import (
	"context"
	"flag"
	"fmt"
	"strings"

	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/imports"
	"go.skia.org/infra/go/sklog"
)

var (
	startPkg = flag.String("start_pkg", "", "Start with this package.")
	findPkg  = flag.String("find_pkg", "", "Find importers of this package.")
)

func main() {
	common.Init()
	// Pre-load data for all packages in this repo.
	if _, err := imports.LoadAllPackageData(context.Background()); err != nil {
		sklog.Fatal(err)
	}
	paths, err := imports.FindImportPaths(context.Background(), *startPkg, *findPkg)
	if err != nil {
		sklog.Fatal(err)
	}
	for _, path := range paths {
		str := strings.Join(path, " <- ")
		fmt.Println(str)
	}
}
