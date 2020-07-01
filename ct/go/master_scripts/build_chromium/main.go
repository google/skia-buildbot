// Application that builds chromium with or without patches and uploads the build
// to Google Storage.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"time"

	"go.skia.org/infra/ct/go/master_scripts/master_common"
	"go.skia.org/infra/ct/go/util"
	"go.skia.org/infra/go/sklog"
)

var (
	targetPlatform = flag.String("target_platform", util.PLATFORM_ANDROID, "The platform the benchmark will run on (Android / Linux).")
	chromiumHash   = flag.String("chromium_hash", "", "The Chromium commit hash the checkout should be synced to. If not specified then Chromium's ToT hash is used.")
	skiaHash       = flag.String("skia_hash", "", "The Skia commit hash the checkout should be synced to. If not specified then Skia's LKGR hash is used (the hash in Chromium's DEPS file).")
)

func buildChromium() error {
	swarmingClient, err := master_common.Init("build_chromium")
	if err != nil {
		return fmt.Errorf("Could not init: %s", err)
	}

	ctx := context.Background()

	// Finish with glog flush and how long the task took.
	defer util.TimeTrack(time.Now(), "Running build chromium")
	defer sklog.Flush()

	if *chromiumHash == "" {
		return errors.New("Must specify --chromium_hash")
	}
	if *skiaHash == "" {
		return errors.New("Must specify --skia_hash")
	}

	// Create the required chromium build. Differentiate between the master script
	// builds and build_chromium by specifying runID as empty here.
	chromiumBuilds, err := util.TriggerBuildRepoSwarmingTask(ctx, "build_chromium", "", "chromium", "Linux", "", []string{*chromiumHash, *skiaHash}, []string{}, []string{}, true /*singleBuild*/, *master_common.Local, 3*time.Hour, 1*time.Hour, swarmingClient)
	if err != nil {
		return fmt.Errorf("Error encountered when swarming build repo task: %s", err)
	}
	if len(chromiumBuilds) != 1 {
		return fmt.Errorf("Expected 1 build but instead got %d: %v", len(chromiumBuilds), chromiumBuilds)
	}

	return nil
}

func main() {
	retCode := 0
	if err := buildChromium(); err != nil {
		sklog.Errorf("Error while running build chromium: %s", err)
		retCode = 255
	}
	os.Exit(retCode)
}
