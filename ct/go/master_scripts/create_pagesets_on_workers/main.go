// create_pagesets_on_workers is an application that creates pagesets on all CT
// workers and uploads it to Google Storage. The requester is emailed when the task
// is done.
package main

import (
	"flag"
	"fmt"
	"path/filepath"
	"time"

	"go.skia.org/infra/ct/go/ctfe/admin_tasks"
	"go.skia.org/infra/ct/go/frontend"
	"go.skia.org/infra/ct/go/master_scripts/master_common"
	"go.skia.org/infra/ct/go/util"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/sklog"
	skutil "go.skia.org/infra/go/util"
)

const (
	// TODO(rmistry): Change back to 1000 once swarming can handle >10k pending tasks.
	MAX_PAGES_PER_SWARMING_BOT = 50000
)

var (
	emails      = flag.String("emails", "", "The comma separated email addresses to notify when the task is picked up and completes.")
	gaeTaskID   = flag.Int64("gae_task_id", -1, "The key of the App Engine task. This task will be updated when the task is completed.")
	pagesetType = flag.String("pageset_type", "", "The type of pagesets to create from the Alexa CSV list. Eg: 10k, Mobile10k, All.")
	runID       = flag.String("run_id", "", "The unique run id (typically requester + timestamp).")

	taskCompletedSuccessfully = new(bool)
)

func sendEmail(recipients []string) {
	// Send completion email.
	emailSubject := "Create pagesets Cluster telemetry task has completed"
	failureHtml := ""
	if !*taskCompletedSuccessfully {
		emailSubject += " with failures"
		failureHtml = util.GetFailureEmailHtml(*runID)
	}
	bodyTemplate := `
	The Cluster telemetry queued task to create %s pagesets has completed. %s.<br/>
	%s
	You can schedule more runs <a href="%s">here</a>.<br/><br/>
	Thanks!
	`
	emailBody := fmt.Sprintf(bodyTemplate, *pagesetType, util.GetSwarmingLogsLink(*runID), failureHtml, frontend.AdminTasksWebapp)
	if err := util.SendEmail(recipients, emailSubject, emailBody); err != nil {
		sklog.Errorf("Error while sending email: %s", err)
		return
	}
}

func updateWebappTask() {
	vars := admin_tasks.RecreatePageSetsUpdateVars{}
	vars.Id = *gaeTaskID
	vars.SetCompleted(*taskCompletedSuccessfully)
	skutil.LogErr(frontend.UpdateWebappTaskV2(&vars))
}

func main() {
	defer common.LogPanic()
	master_common.Init()

	// Send start email.
	emailsArr := util.ParseEmails(*emails)
	emailsArr = append(emailsArr, util.CtAdmins...)
	if len(emailsArr) == 0 {
		sklog.Error("At least one email address must be specified")
		return
	}
	skutil.LogErr(frontend.UpdateWebappTaskSetStarted(&admin_tasks.RecreatePageSetsUpdateVars{}, *gaeTaskID))
	skutil.LogErr(util.SendTaskStartEmail(emailsArr, "Creating pagesets", *runID, ""))
	// Ensure webapp is updated and completion email is sent even if task fails.
	defer updateWebappTask()
	defer sendEmail(emailsArr)
	// Finish with glog flush and how long the task took.
	defer util.TimeTrack(time.Now(), "Creating Pagesets on Workers")
	defer sklog.Flush()

	if *pagesetType == "" {
		sklog.Error("Must specify --pageset_type")
		return
	}

	// Empty the remote dir before the workers upload to it.
	gs, err := util.NewGsUtil(nil)
	if err != nil {
		sklog.Error(err)
		return
	}
	gsBaseDir := filepath.Join(util.SWARMING_DIR_NAME, util.PAGESETS_DIR_NAME, *pagesetType)
	skutil.LogErr(gs.DeleteRemoteDir(gsBaseDir))

	// Archive, trigger and collect swarming tasks.
	if err := util.TriggerSwarmingTask(*pagesetType, "create_pagesets", util.CREATE_PAGESETS_ISOLATE, *runID, 5*time.Hour, 1*time.Hour, util.ADMIN_TASKS_PRIORITY, MAX_PAGES_PER_SWARMING_BOT, util.PagesetTypeToInfo[*pagesetType].NumPages, map[string]string{}, util.GCE_WORKER_DIMENSIONS); err != nil {
		sklog.Errorf("Error encountered when swarming tasks: %s", err)
		return
	}

	*taskCompletedSuccessfully = true
}
