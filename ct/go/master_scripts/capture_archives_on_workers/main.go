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
	"skia.googlesource.com/buildbot.git/ct/go/util"
	"skia.googlesource.com/buildbot.git/go/common"
)

var (
	emails        = flag.String("emails", "", "The comma separated email addresses to notify when the task is picked up and completes.")
	gaeTaskID     = flag.Int("gae_task_id", -1, "The key of the App Engine task. This task will be updated when the task is completed.")
	pagesetType   = flag.String("pageset_type", "", "The type of pagesets to use. Eg: 10k, Mobile10k, All.")
	chromiumBuild = flag.String("chromium_build", "", "The chromium build to use for this capture_archives run.")

	taskCompletedSuccessfully = new(bool)
)

func sendEmail() {
	// Send completion email.
	emailsArr := util.ParseEmails(*emails)
	if len(emailsArr) == 0 {
		glog.Error("At least one email address must be specified")
		return
	}
	emailSubject := "Capture archives Cluster telemetry task has completed"
	if !*taskCompletedSuccessfully {
		emailSubject += " with failures"
	}
	// TODO(rmistry): Add a link to the master logs here and maybe a table with
	// links to logs of the 100 slaves.
	bodyTemplate := `
	The Cluster telemetry queued task to capture archives of %s pagesets has completed.<br/>
	You can schedule more runs <a href="%s">here</a>.<br/><br/>
	Thanks!
	`
	emailBody := fmt.Sprintf(bodyTemplate, *pagesetType, util.AdminTasksWebapp)
	if err := util.SendEmail(emailsArr, emailSubject, emailBody); err != nil {
		glog.Errorf("Error while sending email: %s", err)
		return
	}
}

func updateWebappTask() {
	if err := util.UpdateWebappTask(*gaeTaskID, util.UpdateAdminTasksWebapp, map[string]string{}); err != nil {
		glog.Errorf("Error while updating webapp task: %s", err)
		return
	}
}

func main() {
	common.Init()
	// Ensure webapp is updated and email is sent even if task fails.
	defer updateWebappTask()
	defer sendEmail()
	defer util.TimeTrack(time.Now(), "Creating Pagesets on Workers")
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
		"make worker_scripts;",
		// The main command that runs capture_archives on all workers.
		fmt.Sprintf("DISPLAY=:0 capture_archives --worker_num=%s --log_dir=%s --pageset_type=%s --chromium_build=%s;", util.WORKER_NUM_KEYWORD, util.GLogDir, *pagesetType, *chromiumBuild),
	}

	// Setting a 5 day timeout since it may take a while to capture 1M archives.
	if _, err := util.SSH(strings.Join(cmd, " "), util.Slaves, 5*24*time.Hour); err != nil {
		glog.Errorf("Error while running cmd %s: %s", cmd, err)
		return
	}
	*taskCompletedSuccessfully = true
}
