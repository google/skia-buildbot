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

	"go.skia.org/infra/ct/go/ctfe/chromium_builds"
	"go.skia.org/infra/ct/go/ctfe/task_common"
	"go.skia.org/infra/ct/go/master_scripts/master_common"
	"go.skia.org/infra/ct/go/util"
	"go.skia.org/infra/go/sklog"
	skutil "go.skia.org/infra/go/util"
)

var (
	emails         = flag.String("emails", "", "The comma separated email addresses to notify when the task is picked up and completes.")
	taskID         = flag.Int64("task_id", -1, "The key of the CT task in CTFE. The task will be updated when it is started and also when it completes.")
	runID          = flag.String("run_id", "", "The unique run id (typically requester + timestamp).")
	targetPlatform = flag.String("target_platform", util.PLATFORM_ANDROID, "The platform the benchmark will run on (Android / Linux).")
	chromiumHash   = flag.String("chromium_hash", "", "The Chromium commit hash the checkout should be synced to. If not specified then Chromium's ToT hash is used.")
	skiaHash       = flag.String("skia_hash", "", "The Skia commit hash the checkout should be synced to. If not specified then Skia's LKGR hash is used (the hash in Chromium's DEPS file).")

	taskCompletedSuccessfully = false
	chromiumBuildTimestamp    = ""
)

func sendEmail(recipients []string) {
	emailSubject := "Chromium build task has completed"
	failureHtml := ""
	if !taskCompletedSuccessfully {
		emailSubject += " with failures"
		failureHtml = util.GetFailureEmailHtml(*runID)
	}
	bodyTemplate := `
	The Cluster telemetry queued task to create a new chromium build has completed. %s.<br/>
	%s
	You can schedule more runs <a href="%s">here</a>.<br/><br/>
	Thanks!
	`
	emailBody := fmt.Sprintf(bodyTemplate, util.GetSwarmingLogsLink(*runID), failureHtml, master_common.ChromiumBuildTasksWebapp)
	if err := util.SendEmail(recipients, emailSubject, emailBody); err != nil {
		sklog.Errorf("Error while sending email: %s", err)
		return
	}
}

func updateTaskInDatastore(ctx context.Context) {
	vars := chromium_builds.UpdateVars{}
	vars.Id = *taskID
	vars.SetCompleted(taskCompletedSuccessfully)
	skutil.LogErr(task_common.FindAndUpdateTask(ctx, &vars))
}

func buildChromium() error {
	master_common.Init("build_chromium")

	ctx := context.Background()

	// Send start email.
	emailsArr := util.ParseEmails(*emails)
	emailsArr = append(emailsArr, util.CtAdmins...)
	if len(emailsArr) == 0 {
		return errors.New("At least one email address must be specified")
	}
	skutil.LogErr(task_common.UpdateTaskSetStarted(ctx, &chromium_builds.UpdateVars{}, *taskID, *runID))
	skutil.LogErr(util.SendTaskStartEmail(*taskID, emailsArr, "Build chromium", *runID, "", ""))
	// Ensure task is updated and completion email is sent even if task fails.
	defer updateTaskInDatastore(ctx)
	defer sendEmail(emailsArr)
	// Finish with glog flush and how long the task took.
	defer util.TimeTrack(time.Now(), "Running build chromium")
	defer sklog.Flush()

	if *chromiumHash == "" {
		return errors.New("Must specify --chromium_hash")
	}
	if *skiaHash == "" {
		return errors.New("Must specify --skia_hash")
	}

	// Create the required chromium build.
	// Note: chromium_builds.CreateChromiumBuildOnSwarming specifies the
	//       "-DSK_WHITELIST_SERIALIZED_TYPEFACES" flag only when *runID is empty.
	//       Since builds created by this master script will be consumed only by the
	//       capture_skps tasks (which require that flag) specify runID as empty here.
	chromiumBuilds, err := util.TriggerBuildRepoSwarmingTask(ctx, "build_chromium", "", "chromium", "Linux", "", []string{*chromiumHash, *skiaHash}, []string{}, []string{}, true /*singleBuild*/, *master_common.Local, 3*time.Hour, 1*time.Hour)
	if err != nil {
		return fmt.Errorf("Error encountered when swarming build repo task: %s", err)
	}
	if len(chromiumBuilds) != 1 {
		return fmt.Errorf("Expected 1 build but instead got %d: %v", len(chromiumBuilds), chromiumBuilds)
	}

	taskCompletedSuccessfully = true
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
