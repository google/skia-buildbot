// skia_build is a command line application to trigger or force builds
// of Skia that are done using go/buildskia.
//
// This only builds
package main

import (
	"flag"
	"fmt"
	"time"

	"github.com/skia-dev/glog"
	"go.skia.org/infra/go/buildskia"
	"go.skia.org/infra/go/common"
)

// flags
var (
	local       = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	head        = flag.Bool("head", false, "Sync to HEAD instead of Skia LKGR.")
	force       = flag.Bool("force", false, "Force a rebuild even if the library has already been checked out.")
	installDeps = flag.Bool("install_deps", false, "Install Skia dependencies")
	workRoot    = flag.String("work_root", "", "Directory location where all the work is done.")
	depotTools  = flag.String("depot_tools", "", "Directory location where depot_tools is installed.")
)

func main() {
	common.Init()
	if *workRoot == "" {
		glog.Fatal("The --work_root flag is required.")
	}
	if *depotTools == "" {
		glog.Fatal("The --depot_tools flag is required.")
	}
	b := buildskia.New(*workRoot, *depotTools, nil, nil, 2, time.Hour)
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
