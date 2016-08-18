// This is just a useful stub, will eventually evolve back into the full web UI.
package main

import (
	"flag"
	"fmt"
	"path/filepath"
	"time"

	"github.com/skia-dev/glog"
	"go.skia.org/infra/fiddle/go/buildlib"
	"go.skia.org/infra/go/buildskia"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/gitinfo"
)

// flags
var (
	local       = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	head        = flag.Bool("head", false, "Sync to HEAD instead of Skia LKGR.")
	force       = flag.Bool("force", false, "Force a rebuild even if the library has already been checked out.")
	installDeps = flag.Bool("install_deps", false, "Install Skia dependencies")
	fiddleRoot  = flag.String("fiddle_root", "", "Directory location where all the work is done.")
)

func main() {
	common.Init()
	if *fiddleRoot == "" {
		glog.Fatal("The --fiddle_root flag is required.")
	}
	depotTools := filepath.Join(*fiddleRoot, "depot_tools")
	repo, err := gitinfo.CloneOrUpdate(common.REPO_SKIA, filepath.Join(*fiddleRoot, "skia"), true)
	if err != nil {
		glog.Fatalf("Failed to clone Skia: %s", err)
	}
	b := buildskia.New(*fiddleRoot, depotTools, repo, buildlib.BuildLib, 2, time.Hour, true)
	res, err := b.BuildLatestSkia(*force, *head, *installDeps)
	if err != nil {
		if err == buildskia.AlreadyExistsErr {
			glog.Info("Checkout already exists, no work done.")
		} else {
			glog.Fatalf("Failed to build latest skia: %s", err)
		}
	} else {
		fmt.Printf("Built: %#v", *res)
	}
}
