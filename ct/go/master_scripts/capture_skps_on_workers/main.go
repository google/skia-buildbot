// capture_skps_on_workers is an application that captures SKPs of the
// specified patchset type on all CT workers and uploads the results to Google
// Storage. The requester is emailed when the task is done.
package main

import (
	"context"
	"flag"
	"fmt"
	"path"
	"path/filepath"
	"strings"
	"time"

	"go.skia.org/infra/ct/go/ctfe/capture_skps"
	"go.skia.org/infra/ct/go/frontend"
	"go.skia.org/infra/ct/go/master_scripts/master_common"
	"go.skia.org/infra/ct/go/util"
	"go.skia.org/infra/go/sklog"
	skutil "go.skia.org/infra/go/util"
)

const (
	MAX_PAGES_PER_SWARMING_BOT_CAPTURE_SKPS = 100
)

var (
	emails         = flag.String("emails", "", "The comma separated email addresses to notify when the task is picked up and completes.")
	description    = flag.String("description", "", "The description of the run as entered by the requester.")
	taskID         = flag.Int64("task_id", -1, "The key of the CT task in CTFE. The task will be updated when it is started and also when it completes.")
	pagesetType    = flag.String("pageset_type", "", "The type of pagesets to use. Eg: 10k, Mobile10k, All.")
	chromiumBuild  = flag.String("chromium_build", "", "The chromium build to use for this capture SKPs run.")
	targetPlatform = flag.String("target_platform", util.PLATFORM_LINUX, "The platform the benchmark will run on (Android / Linux).")
	runOnGCE       = flag.Bool("run_on_gce", true, "Run on Linux GCE instances.")
	runID          = flag.String("run_id", "", "The unique run id (typically requester + timestamp).")

	taskCompletedSuccessfully = false
)

func sendEmail(recipients []string) {
	// Send completion email.
	emailSubject := fmt.Sprintf("Capture SKPs cluster telemetry task has completed (#%d)", *taskID)
	failureHtml := ""
	if !taskCompletedSuccessfully {
		emailSubject += " with failures"
		failureHtml = util.GetFailureEmailHtml(*runID)
	}
	bodyTemplate := `
	The Capture SKPs task on %s pageset has completed. %s.<br/>
	Run description: %s<br/>
	%s
	You can schedule more runs <a href="%s">here</a>.<br/><br/>
	Thanks!
	`
	emailBody := fmt.Sprintf(bodyTemplate, *pagesetType, util.GetSwarmingLogsLink(*runID), *description, failureHtml, frontend.CaptureSKPsTasksWebapp)
	if err := util.SendEmail(recipients, emailSubject, emailBody); err != nil {
		sklog.Errorf("Error while sending email: %s", err)
		return
	}
}

func updateWebappTask() {
	vars := capture_skps.UpdateVars{}
	vars.Id = *taskID
	vars.SetCompleted(taskCompletedSuccessfully)
	skutil.LogErr(frontend.UpdateWebappTaskV2(&vars))
}

func main() {
	master_common.Init("capture_skps")

	ctx := context.Background()

	// Send start email.
	emailsArr := util.ParseEmails(*emails)
	emailsArr = append(emailsArr, util.CtAdmins...)
	if len(emailsArr) == 0 {
		sklog.Error("At least one email address must be specified")
		return
	}
	skutil.LogErr(frontend.UpdateWebappTaskSetStarted(&capture_skps.UpdateVars{}, *taskID, *runID))
	skutil.LogErr(util.SendTaskStartEmail(*taskID, emailsArr, "Capture SKPs", *runID, *description))
	// Ensure webapp is updated and completion email is sent even if task
	// fails.
	defer updateWebappTask()
	defer sendEmail(emailsArr)

	// Finish with glog flush and how long the task took.
	defer util.TimeTrack(time.Now(), "Running capture skps task on workers")
	defer sklog.Flush()

	if *pagesetType == "" {
		sklog.Error("Must specify --pageset_type")
		return
	}
	if *chromiumBuild == "" {
		sklog.Error("Must specify --chromium_build")
		return
	}
	if *runID == "" {
		sklog.Error("Must specify --run_id")
		return
	}

	// Empty the remote dir before the workers upload to it.
	gs, err := util.NewGcsUtil(nil)
	if err != nil {
		sklog.Error(err)
		return
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
			ctx, "build_skpinfo", *runID, "skiaSKPInfo", util.PLATFORM_LINUX, []string{}, []string{}, []string{cipdPackage}, true, *master_common.Local, 3*time.Hour, 1*time.Hour)
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
		telemetryHash, err := util.TriggerIsolateTelemetrySwarmingTask(ctx, "isolate_telemetry", *runID, chromiumHash, []string{}, 1*time.Hour, 1*time.Hour, *master_common.Local)
		if err != nil {
			return sklog.FmtErrorf("Error encountered when swarming isolate telemetry task: %s", err)
		}
		if telemetryHash == "" {
			return sklog.FmtErrorf("Found empty telemetry hash!")
		}
		isolateDeps = append(isolateDeps, telemetryHash)
		return nil
	})

	// Wait for skpinfo build task and isolate telemetry task to complete.
	if err := group.Wait(); err != nil {
		sklog.Error(err)
		return
	}

	// Archive, trigger and collect swarming tasks.
	isolateExtraArgs := map[string]string{
		"CHROMIUM_BUILD":      *chromiumBuild,
		"RUN_ID":              *runID,
		"SKPINFO_REMOTE_PATH": skpinfoRemotePath,
	}
	if _, err := util.TriggerSwarmingTask(ctx, *pagesetType, "capture_skps", util.CAPTURE_SKPS_ISOLATE, *runID, 3*time.Hour, 1*time.Hour, util.ADMIN_TASKS_PRIORITY, MAX_PAGES_PER_SWARMING_BOT_CAPTURE_SKPS, util.PagesetTypeToInfo[*pagesetType].NumPages, isolateExtraArgs, *runOnGCE, *master_common.Local, 1, isolateDeps); err != nil {
		sklog.Errorf("Error encountered when swarming tasks: %s", err)
		return
	}

	taskCompletedSuccessfully = true
}
