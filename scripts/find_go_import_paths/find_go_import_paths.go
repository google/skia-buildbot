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
	allPkgs, err := imports.LoadAllPackageData(context.Background())
	if err != nil {
		sklog.Fatal(err)
	}
	if *startPkg != "" {
		allPkgs = map[string]*imports.Package{
			*startPkg: allPkgs[*startPkg],
		}
	}
	allPaths := map[string]bool{}
	for name, _ := range allPkgs {
		paths, err := imports.FindImportPaths(context.Background(), name, *findPkg)
		if err != nil {
			sklog.Fatal(err)
		}
		for _, path := range paths {
			str := strings.Join(path, " <- ")
			allPaths[str] = true
		}
	}
	for path, _ := range allPaths {
		fmt.Println(path)
	}
}
