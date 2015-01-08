// Application that builds chromium with or without patches and uploads the build
// to Google Storage.
package main

import (
	"flag"
	"time"

	"github.com/skia-dev/glog"

	"skia.googlesource.com/buildbot.git/ct/go/util"
	"skia.googlesource.com/buildbot.git/go/common"
)

var (
	runID          = flag.String("run_id", "", "The unique run id (typically requester + timestamp).")
	targetPlatform = flag.String("target_platform", util.PLATFORM_ANDROID, "The platform the benchmark will run on (Android / Linux).")
	applyPatches   = flag.Bool("apply_patches", false, "If true looks for Chromium/Blink/Skia patches in temp dir and runs once with the patches and once without.")
	chromiumHash   = flag.String("chromium_hash", "", "The Chromium commit hash the checkout should be synced to. If not specified then Chromium's ToT hash is used.")
	skiaHash       = flag.String("skia_hash", "", "The Skia commit hash the checkout should be synced to. If not specified then Skia's LKGR hash is used (the hash in Chromium's DEPS file).")
)

func main() {
	common.Init()
	defer util.TimeTrack(time.Now(), "Running build chromium")
	defer glog.Flush()

	if _, _, err := util.CreateChromiumBuild(*runID, *targetPlatform, *chromiumHash, *skiaHash, *applyPatches); err != nil {
		glog.Errorf("Error while creating the Chromium build: %s", err)
		return
	}
}
