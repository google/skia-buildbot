// capture_skps_on_workers is an application that captures SKPs of the
// specified patchset type on all CT workers and uploads the results to Google
// Storage. The requester is emailed when the task is done.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"strings"
	"text/template"
	"time"

	"github.com/skia-dev/glog"
	"go.skia.org/infra/ct/go/ctfe/capture_skps"
	"go.skia.org/infra/ct/go/frontend"
	"go.skia.org/infra/ct/go/util"
	"go.skia.org/infra/go/common"
	skutil "go.skia.org/infra/go/util"
)

var (
	emails         = flag.String("emails", "", "The comma separated email addresses to notify when the task is picked up and completes.")
	gaeTaskID      = flag.Int64("gae_task_id", -1, "The key of the App Engine task. This task will be updated when the task is completed.")
	pagesetType    = flag.String("pageset_type", "", "The type of pagesets to use. Eg: 10k, Mobile10k, All.")
	chromiumBuild  = flag.String("chromium_build", "", "The chromium build to use for this capture SKPs run.")
	targetPlatform = flag.String("target_platform", util.PLATFORM_LINUX, "The platform the benchmark will run on (Android / Linux).")
	runID          = flag.String("run_id", "", "The unique run id (typically requester + timestamp).")

	taskCompletedSuccessfully = false
	outputRemoteLink          = util.MASTER_LOGSERVER_LINK
)

func sendEmail(recipients []string) {
	// Send completion email.
	emailSubject := fmt.Sprintf("Capture SKPs cluster telemetry task has completed (%s)", *runID)
	failureHtml := ""
	if !taskCompletedSuccessfully {
		emailSubject += " with failures"
		failureHtml = util.GetFailureEmailHtml(*runID)
	}
	bodyTemplate := `
	The Capture SKPs task on %s pageset has completed.<br/>
	%s
	The output of your script is available <a href='%s'>here</a>.<br/><br/>
	You can schedule more runs <a href="%s">here</a>.<br/><br/>
	Thanks!
	`
	emailBody := fmt.Sprintf(bodyTemplate, *pagesetType, failureHtml, outputRemoteLink, frontend.CaptureSKPsTasksWebapp)
	if err := util.SendEmail(recipients, emailSubject, emailBody); err != nil {
		glog.Errorf("Error while sending email: %s", err)
		return
	}
}

func updateWebappTask() {
	if frontend.CtfeV2 {
		vars := capture_skps.UpdateVars{}
		vars.Id = *gaeTaskID
		vars.SetCompleted(taskCompletedSuccessfully)
		skutil.LogErr(frontend.UpdateWebappTaskV2(&vars))
		return
	}
	extraData := map[string]string{
		"output_link": outputRemoteLink,
	}
	if err := frontend.UpdateWebappTask(*gaeTaskID, frontend.UpdateCaptureSKPsTasksWebapp, extraData); err != nil {
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
	skutil.LogErr(frontend.UpdateWebappTaskSetStarted(&capture_skps.UpdateVars{}, *gaeTaskID))
	skutil.LogErr(util.SendTaskStartEmail(emailsArr, "Capture SKPs", util.GetMasterLogLink(*runID)))
	// Ensure webapp is updated and completion email is sent even if task
	// fails.
	defer updateWebappTask()
	defer sendEmail(emailsArr)

	// Cleanup tmp files after the run.
	defer util.CleanTmpDir()
	// Finish with glog flush and how long the task took.
	defer util.TimeTrack(time.Now(), "Running capture skps task on workers")
	defer glog.Flush()

	if *pagesetType == "" {
		glog.Error("Must specify --pageset_type")
		return
	}
	if *chromiumBuild == "" {
		glog.Error("Must specify --chromium_build")
		return
	}
	if *runID == "" {
		glog.Error("Must specify --run_id")
		return
	}

	// Run the capture_skps script on all workers.
	captureSKPsCmdTemplate := "DISPLAY=:0 capture_skps --worker_num={{.WorkerNum}} --log_dir={{.LogDir}} --log_id={{.RunID}} " +
		"--pageset_type={{.PagesetType}} --chromium_build={{.ChromiumBuild}} --run_id={{.RunID}} " +
		"--target_platform={{.TargetPlatform}};"
	captureSKPsTemplateParsed := template.Must(template.New("capture_skps_cmd").Parse(captureSKPsCmdTemplate))
	captureSKPsCmdBytes := new(bytes.Buffer)
	if err := captureSKPsTemplateParsed.Execute(captureSKPsCmdBytes, struct {
		WorkerNum      string
		LogDir         string
		PagesetType    string
		ChromiumBuild  string
		RunID          string
		TargetPlatform string
	}{
		WorkerNum:      util.WORKER_NUM_KEYWORD,
		LogDir:         util.GLogDir,
		PagesetType:    *pagesetType,
		ChromiumBuild:  *chromiumBuild,
		RunID:          *runID,
		TargetPlatform: *targetPlatform,
	}); err != nil {
		glog.Errorf("Failed to execute template: %s", err)
		return
	}
	cmd := []string{
		fmt.Sprintf("cd %s;", util.CtTreeDir),
		"git pull;",
		"make all;",
		// The main command that runs capture_skps on all workers.
		captureSKPsCmdBytes.String(),
	}

	_, err := util.SSH(strings.Join(cmd, " "), util.Slaves, util.CAPTURE_SKPS_TIMEOUT)
	if err != nil {
		glog.Errorf("Error while running cmd %s: %s", cmd, err)
		return
	}

	taskCompletedSuccessfully = true
}
