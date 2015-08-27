// capture_archives_on_workers is an application that captures archives on all CT
// workers and uploads it to Google Storage. The requester is emailed when the task
// is done.
package main

import (
	"flag"
	"fmt"
	"strings"
	"time"

	"github.com/skia-dev/glog"
	"go.skia.org/infra/ct/go/ctfe/admin_tasks"
	"go.skia.org/infra/ct/go/frontend"
	"go.skia.org/infra/ct/go/util"
	"go.skia.org/infra/go/common"
	skutil "go.skia.org/infra/go/util"
)

var (
	emails        = flag.String("emails", "", "The comma separated email addresses to notify when the task is picked up and completes.")
	gaeTaskID     = flag.Int64("gae_task_id", -1, "The key of the App Engine task. This task will be updated when the task is completed.")
	pagesetType   = flag.String("pageset_type", "", "The type of pagesets to use. Eg: 10k, Mobile10k, All.")
	chromiumBuild = flag.String("chromium_build", "", "The chromium build to use for this capture_archives run.")
	runID         = flag.String("run_id", "", "The unique run id (typically requester + timestamp).")

	taskCompletedSuccessfully = new(bool)
)

func sendEmail(recipients []string) {
	// Send completion email.
	emailSubject := "Capture archives Cluster telemetry task has completed"
	failureHtml := ""
	if !*taskCompletedSuccessfully {
		emailSubject += " with failures"
		failureHtml = util.FailureEmailHtml
	}
	bodyTemplate := `
	The Cluster telemetry queued task to capture archives of %s pagesets has completed.<br/>
	%s
	You can schedule more runs <a href="%s">here</a>.<br/><br/>
	Thanks!
	`
	emailBody := fmt.Sprintf(bodyTemplate, *pagesetType, failureHtml, frontend.AdminTasksWebapp)
	if err := util.SendEmail(recipients, emailSubject, emailBody); err != nil {
		glog.Errorf("Error while sending email: %s", err)
		return
	}
}

func updateWebappTask() {
	if frontend.CtfeV2 {
		vars := admin_tasks.RecreateWebpageArchivesUpdateVars{}
		vars.Id = *gaeTaskID
		vars.SetCompleted(*taskCompletedSuccessfully)
		skutil.LogErr(frontend.UpdateWebappTaskV2(&vars))
		return
	}
	if err := frontend.UpdateWebappTask(*gaeTaskID, frontend.UpdateAdminTasksWebapp, map[string]string{}); err != nil {
		glog.Errorf("Error while updating webapp task: %s", err)
		return
	}
}

func main() {
	defer common.LogPanic()
	common.Init()
	frontend.MustInit()

	// Send start email.
	emailsArr := util.ParseEmails(*emails)
	emailsArr = append(emailsArr, util.CtAdmins...)
	if len(emailsArr) == 0 {
		glog.Error("At least one email address must be specified")
		return
	}
	skutil.LogErr(frontend.UpdateWebappTaskSetStarted(&admin_tasks.RecreateWebpageArchivesUpdateVars{}, *gaeTaskID))
	skutil.LogErr(util.SendTaskStartEmail(emailsArr, "Capture archives", util.GetMasterLogLink(*runID)))
	// Ensure webapp is updated and completion email is sent even if task fails.
	defer updateWebappTask()
	defer sendEmail(emailsArr)
	// Cleanup tmp files after the run.
	defer util.CleanTmpDir()
	// Finish with glog flush and how long the task took.
	defer util.TimeTrack(time.Now(), "Capture archives on Workers")
	defer glog.Flush()

	if *pagesetType == "" {
		glog.Error("Must specify --pageset_type")
		return
	}
	if *chromiumBuild == "" {
		glog.Error("Must specify --chromium_build")
		return
	}

	cmd := []string{
		fmt.Sprintf("cd %s;", util.CtTreeDir),
		"git pull;",
		"make all;",
		// The main command that runs capture_archives on all workers.
		fmt.Sprintf("DISPLAY=:0 capture_archives --worker_num=%s --log_dir=%s --log_id=%s --pageset_type=%s --chromium_build=%s;", util.WORKER_NUM_KEYWORD, util.GLogDir, *runID, *pagesetType, *chromiumBuild),
	}

	_, err := util.SSH(strings.Join(cmd, " "), util.Slaves, util.CAPTURE_ARCHIVES_TIMEOUT)
	if err != nil {
		glog.Errorf("Error while running cmd %s: %s", cmd, err)
		return
	}
	*taskCompletedSuccessfully = true
}
