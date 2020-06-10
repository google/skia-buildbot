// capture_skps_on_workers is an application that captures SKPs of the
// specified patchset type on all CT workers and uploads the results to Google
// Storage. The requester is emailed when the task is done.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"go.skia.org/infra/ct/go/master_scripts/master_common"
	"go.skia.org/infra/ct/go/util"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	skutil "go.skia.org/infra/go/util"
)

const (
	MAX_PAGES_PER_SWARMING_BOT_CAPTURE_SKPS = 100
)

var (
	pagesetType    = flag.String("pageset_type", "", "The type of pagesets to use. Eg: 10k, Mobile10k, All.")
	chromiumBuild  = flag.String("chromium_build", "", "The chromium build to use for this capture SKPs run.")
	targetPlatform = flag.String("target_platform", util.PLATFORM_LINUX, "The platform the benchmark will run on (Android / Linux).")
	runOnGCE       = flag.Bool("run_on_gce", true, "Run on Linux GCE instances.")
	runID          = flag.String("run_id", "", "The unique run id (typically requester + timestamp).")
)

func captureSKPsOnWorkers() error {
	swarmingClient, err := master_common.Init("capture_skps")
	if err != nil {
		return fmt.Errorf("Could not init: %s", err)
	}

	ctx := context.Background()

	// Finish with glog flush and how long the task took.
	defer util.TimeTrack(time.Now(), "Running capture skps task on workers")
	defer sklog.Flush()

	if *pagesetType == "" {
		return errors.New("Must specify --pageset_type")
	}
	if *chromiumBuild == "" {
		return errors.New("Must specify --chromium_build")
	}
	if *runID == "" {
		return errors.New("Must specify --run_id")
	}

	// Empty the remote dir before the workers upload to it.
	gs, err := util.NewGcsUtil(nil)
	if err != nil {
		return err
	}
	skpGCSBaseDir := filepath.Join(util.SWARMING_DIR_NAME, util.SKPS_DIR_NAME, *pagesetType, *chromiumBuild)
	skutil.LogErr(gs.DeleteRemoteDir(skpGCSBaseDir))

	// Trigger both the skpinfo build and isolate telemetry tasks in parallel.
	group := skutil.NewNamedErrGroup()
	var skpinfoRemotePath string
	group.Go("build skpinfo", func() error {
		cipdPackage, err := util.GetCipdPackageFromAsset("clang_linux")
		if err != nil {
			return fmt.Errorf("Could not get cipd package for clang_linux: %s", err)
		}
		remoteDirNames, err := util.TriggerBuildRepoSwarmingTask(
			ctx, "build_skpinfo", *runID, "skiaSKPInfo", util.PLATFORM_LINUX, "", []string{}, []string{}, []string{cipdPackage}, true, *master_common.Local, 3*time.Hour, 1*time.Hour, swarmingClient)
		if err != nil {
			return fmt.Errorf("Error encountered when swarming build skpinfo task: %s", err)
		}
		skpinfoRemoteDirName := remoteDirNames[0]
		skpinfoRemotePath = path.Join(util.BINARIES_DIR_NAME, skpinfoRemoteDirName, util.BINARY_SKPINFO)
		return nil
	})

	// Isolate telemetry
	isolateDeps := []string{}
	group.Go("isolate telemetry", func() error {
		tokens := strings.Split(*chromiumBuild, "-")
		chromiumHash := tokens[0]
		telemetryHash, err := util.TriggerIsolateTelemetrySwarmingTask(ctx, "isolate_telemetry", *runID, chromiumHash, "", *targetPlatform, []string{}, 1*time.Hour, 1*time.Hour, *master_common.Local, swarmingClient)
		if err != nil {
			return skerr.Fmt("Error encountered when swarming isolate telemetry task: %s", err)
		}
		if telemetryHash == "" {
			return skerr.Fmt("Found empty telemetry hash!")
		}
		isolateDeps = append(isolateDeps, telemetryHash)
		return nil
	})

	// Wait for skpinfo build task and isolate telemetry task to complete.
	if err := group.Wait(); err != nil {
		return err
	}

	// Archive, trigger and collect swarming tasks.
	baseCmd := []string{
		"luci-auth",
		"context",
		"--",
		"bin/capture_skps",
		"-logtostderr",
		"--chromium_build=" + *chromiumBuild,
		"--skpinfo_remote_path=" + skpinfoRemotePath,
		"--run_id=" + *runID,
	}
	if _, err := util.TriggerSwarmingTask(ctx, *pagesetType, "capture_skps", util.CAPTURE_SKPS_ISOLATE, *runID, "", *targetPlatform, 3*time.Hour, 1*time.Hour, util.TASKS_PRIORITY_LOW, MAX_PAGES_PER_SWARMING_BOT_CAPTURE_SKPS, util.PagesetTypeToInfo[*pagesetType].NumPages, *runOnGCE, *master_common.Local, 1, baseCmd, isolateDeps, swarmingClient); err != nil {
		return fmt.Errorf("Error encountered when swarming tasks: %s", err)
	}

	return nil
}

func main() {
	retCode := 0
	if err := captureSKPsOnWorkers(); err != nil {
		sklog.Errorf("Error while running capture skps on workers: %s", err)
		retCode = 255
	}
	os.Exit(retCode)
}
