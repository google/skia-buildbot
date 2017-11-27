// skia_build is a command line application to trigger or force builds
// of Skia that are done using go/buildskia.
//
// This only builds
package main

import (
	"context"
	"flag"
	"fmt"
	"path/filepath"
	"time"

	"go.skia.org/infra/go/buildskia"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/git/gitinfo"
	"go.skia.org/infra/go/sklog"
)

// flags
var (
	local       = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	head        = flag.Bool("head", false, "Sync to HEAD instead of Skia LKGR.")
	force       = flag.Bool("force", false, "Force a rebuild even if the library has already been checked out.")
	installDeps = flag.Bool("install_deps", false, "Install Skia dependencies")
	workRoot    = flag.String("work_root", "", "Directory location where all the work is done.")
	depotTools  = flag.String("depot_tools", "", "Directory location where depot_tools is installed.")
	useGN       = flag.Bool("use_gn", false, "This application use GN to build.")
)

func main() {
	common.Init()
	if *workRoot == "" {
		sklog.Fatal("The --work_root flag is required.")
	}
	if *depotTools == "" {
		sklog.Fatal("The --depot_tools flag is required.")
	}
	ctx := context.Background()
	repo, err := gitinfo.CloneOrUpdate(ctx, common.REPO_SKIA, filepath.Join(*workRoot, "skia"), true)
	if err != nil {
		sklog.Fatalf("Failed to clone Skia: %s", err)
	}

	b := buildskia.New(ctx, *workRoot, *depotTools, repo, nil, 2, time.Hour, *useGN)
	res, err := b.BuildLatestSkia(ctx, *force, *head, *installDeps)
	if err != nil {
		if err == buildskia.AlreadyExistsErr {
			sklog.Info("Checkout already exists, no work done.")
		} else {
			sklog.Fatalf("Failed to build latest skia: %s", err)
		}
	} else {
		fmt.Printf("Built: %#v", *res)
	}
}
