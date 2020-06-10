// create_pagesets_on_workers is an application that creates pagesets on all CT
// workers and uploads it to Google Storage. The requester is emailed when the task
// is done.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"go.skia.org/infra/ct/go/master_scripts/master_common"
	"go.skia.org/infra/ct/go/util"
	"go.skia.org/infra/go/sklog"
	skutil "go.skia.org/infra/go/util"
)

const (
	// TODO(rmistry): Change back to 1000 once swarming can handle >10k pending tasks.
	MAX_PAGES_PER_SWARMING_BOT = 50000
)

var (
	pagesetType = flag.String("pageset_type", "", "The type of pagesets to create from the Alexa CSV list. Eg: 10k, Mobile10k, All.")
	runOnGCE    = flag.Bool("run_on_gce", true, "Run on Linux GCE instances.")
	runID       = flag.String("run_id", "", "The unique run id (typically requester + timestamp).")
)

func createPagesetsOnWorkers() error {
	swarmingClient, err := master_common.Init("create_pagesets")
	if err != nil {
		return fmt.Errorf("Could not init: %s", err)
	}

	ctx := context.Background()

	// Finish with glog flush and how long the task took.
	defer util.TimeTrack(time.Now(), "Creating Pagesets on Workers")
	defer sklog.Flush()

	if *pagesetType == "" {
		return errors.New("Must specify --pageset_type")
	}

	// Empty the remote dir before the workers upload to it.
	gs, err := util.NewGcsUtil(nil)
	if err != nil {
		return err
	}
	gsBaseDir := filepath.Join(util.SWARMING_DIR_NAME, util.PAGESETS_DIR_NAME, *pagesetType)
	skutil.LogErr(gs.DeleteRemoteDir(gsBaseDir))

	// Archive, trigger and collect swarming tasks.
	baseCmd := []string{
		"luci-auth",
		"context",
		"--",
		"bin/create_pagesets",
		"-logtostderr",
	}
	if _, err := util.TriggerSwarmingTask(ctx, *pagesetType, "create_pagesets", util.CREATE_PAGESETS_ISOLATE, *runID, "", util.PLATFORM_LINUX, 5*time.Hour, 1*time.Hour, util.TASKS_PRIORITY_LOW, MAX_PAGES_PER_SWARMING_BOT, util.PagesetTypeToInfo[*pagesetType].NumPages, *runOnGCE, *master_common.Local, 1, baseCmd, []string{} /* isolateDeps */, swarmingClient); err != nil {
		return fmt.Errorf("Error encountered when swarming tasks: %s", err)
	}

	return nil
}

func main() {
	retCode := 0
	if err := createPagesetsOnWorkers(); err != nil {
		sklog.Errorf("Error while creating pagesets on workers: %s", err)
		retCode = 255
	}
	os.Exit(retCode)
}
