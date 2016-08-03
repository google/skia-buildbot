// This is just a useful stub, will eventually evolve back into the full web UI.
package main

import (
	"flag"
	"fmt"
	"path/filepath"
	"time"

	"github.com/skia-dev/glog"
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

// buildLib, given a directory that Skia is checked out into, builds libskia.a
// and fiddle_main.o.
func buildLib(checkout, depotTools string) error {
	glog.Info("Starting GNGen")
	if err := buildskia.GNGen(checkout, depotTools, "Release", []string{"is_debug=false"}); err != nil {
		return fmt.Errorf("Failed GN gen: %s", err)
	}

	glog.Info("Building fiddle")
	if msg, err := buildskia.GNNinjaBuild(checkout, depotTools, "Release", "fiddle", true); err != nil {
		return fmt.Errorf("Failed ninja build of fiddle: %q %s", msg, err)
	}
	return nil
}

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
	b := buildskia.New(*fiddleRoot, depotTools, repo, buildLib, 2, time.Hour, true)
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
