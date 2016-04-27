// This is just a useful stub, will eventually evolve back into the full web UI.
package main

import (
	"flag"
	"fmt"

	"github.com/skia-dev/glog"
	"go.skia.org/infra/fiddle/go/builder"
	"go.skia.org/infra/go/common"
)

// flags
var (
	local       = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	head        = flag.Bool("head", false, "Sync to HEAD instead of Skia LKGR.")
	force       = flag.Bool("force", false, "Force a rebuild even if the library has already been checked out.")
	installDeps = flag.Bool("install_deps", false, "Install Skia dependencies")
	fiddleRoot  = flag.String("fiddle_root", "", "Directory location where all the work is done.")
	depotTools  = flag.String("depot_tools", "", "Directory location where depot_tools is installed.")
	milestone   = flag.Bool("milestone", false, "Also create a milestone mNN branch build.")
)

func main() {
	common.Init()
	if *fiddleRoot == "" {
		glog.Fatal("The --fiddle_root flag is required.")
	}
	if *depotTools == "" {
		glog.Fatal("The --depot_tools flag is required.")
	}
	b := builder.New(*fiddleRoot, *depotTools, nil)
	res, err := b.BuildLatestSkia(*force, *head, *installDeps)
	if err != nil {
		if err == builder.AlreadyExistsErr {
			glog.Info("Checkout already exists, no work done.")
		} else {
			glog.Fatalf("Failed to build latest skia: %s", err)
		}
	} else {
		fmt.Printf("Built: %#v", *res)
	}

	// Now do the same thing for the last chrome branch.
	if *milestone {
		name, res, err := b.BuildLatestSkiaChromeBranch(false)
		if err != nil {
			glog.Fatalf("Failed to build latest skia branch: %s", err)
		}
		fmt.Printf("Built: %s %#v", name, *res)
	}
}
