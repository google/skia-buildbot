// run_chromium_analysis_on_workers is an application that runs the specified
// telemetry benchmark on swarming bots and uploads the results to Google
// Storage. The requester is emailed when the task is done.
package main

import (
	"database/sql"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/skia-dev/glog"
	"go.skia.org/infra/ct/go/ctfe/chromium_analysis"
	"go.skia.org/infra/ct/go/frontend"
	"go.skia.org/infra/ct/go/master_scripts/master_common"
	"go.skia.org/infra/ct/go/util"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/email"
	skutil "go.skia.org/infra/go/util"
)

const (
	MAX_PAGES_PER_SWARMING_BOT = 100
)

var (
	emails             = flag.String("emails", "", "The comma separated email addresses to notify when the task is picked up and completes.")
	description        = flag.String("description", "", "The description of the run as entered by the requester.")
	gaeTaskID          = flag.Int64("gae_task_id", -1, "The key of the task. This task will be updated when the task is started and completed.")
	pagesetType        = flag.String("pageset_type", "", "The type of pagesets to use. Eg: 10k, Mobile10k, All.")
	benchmarkName      = flag.String("benchmark_name", "", "The telemetry benchmark to run on the workers.")
	benchmarkExtraArgs = flag.String("benchmark_extra_args", "", "The extra arguments that are passed to the specified benchmark.")
	browserExtraArgs   = flag.String("browser_extra_args", "", "The extra arguments that are passed to the browser while running the benchmark.")
	runID              = flag.String("run_id", "", "The unique run id (typically requester + timestamp).")

	taskCompletedSuccessfully = false

	chromiumPatchLink  = util.MASTER_LOGSERVER_LINK
	catapultPatchLink  = util.MASTER_LOGSERVER_LINK
	benchmarkPatchLink = util.MASTER_LOGSERVER_LINK
	outputLink         = util.MASTER_LOGSERVER_LINK
)

func sendEmail(recipients []string, gs *util.GsUtil) {
	// Send completion email.
	emailSubject := fmt.Sprintf("Cluster telemetry chromium analysis task has completed (%s)", *runID)
	failureHtml := ""
	viewActionMarkup := ""
	var err error

	if taskCompletedSuccessfully {
		if viewActionMarkup, err = email.GetViewActionMarkup(outputLink, "View Results", "Direct link to the CSV results"); err != nil {
			glog.Errorf("Failed to get view action markup: %s", err)
			return
		}
	} else {
		emailSubject += " with failures"
		failureHtml = util.GetFailureEmailHtml(*runID)
		if viewActionMarkup, err = email.GetViewActionMarkup(util.GetMasterLogLink(*runID), "View Failure", "Direct link to the master log"); err != nil {
			glog.Errorf("Failed to get view action markup: %s", err)
			return
		}
	}

	totalArchivedWebpages, err := util.GetArchivesNum(gs, *benchmarkExtraArgs, *pagesetType)
	if err != nil {
		glog.Errorf("Error when calculating number of archives: %s", err)
	}
	archivedWebpagesText := ""
	if totalArchivedWebpages != -1 {
		archivedWebpagesText = fmt.Sprintf(" %d WPR archives were used.", totalArchivedWebpages)
	}

	bodyTemplate := `
	The chromium analysis %s benchmark task on %s pageset has completed.<br/>
	Run description: %s<br/>
	%s
	The CSV output is <a href='%s'>here</a>.%s<br/>
	The patch(es) you specified are here:
	<a href='%s'>chromium</a>/<a href='%s'>catapult</a>/<a href='%s'>telemetry</a>
	<br/><br/>
	You can schedule more runs <a href='%s'>here</a>.
	<br/><br/>
	Thanks!
	`
	emailBody := fmt.Sprintf(bodyTemplate, *benchmarkName, *pagesetType, *description, failureHtml, outputLink, archivedWebpagesText, chromiumPatchLink, catapultPatchLink, benchmarkPatchLink, frontend.ChromiumAnalysisTasksWebapp)
	if err := util.SendEmailWithMarkup(recipients, emailSubject, emailBody, viewActionMarkup); err != nil {
		glog.Errorf("Error while sending email: %s", err)
		return
	}
}

func updateWebappTask() {
	vars := chromium_analysis.UpdateVars{}
	vars.Id = *gaeTaskID
	vars.SetCompleted(taskCompletedSuccessfully)
	vars.RawOutput = sql.NullString{String: outputLink, Valid: true}
	skutil.LogErr(frontend.UpdateWebappTaskV2(&vars))
}

func main() {
	defer common.LogPanic()
	master_common.Init()

	// Send start email.
	emailsArr := util.ParseEmails(*emails)
	emailsArr = append(emailsArr, util.CtAdmins...)
	if len(emailsArr) == 0 {
		glog.Error("At least one email address must be specified")
		return
	}
	// Instantiate GsUtil object.
	gs, err := util.NewGsUtil(nil)
	if err != nil {
		glog.Errorf("Could not instantiate gsutil object: %s", err)
		return
	}

	skutil.LogErr(frontend.UpdateWebappTaskSetStarted(&chromium_analysis.UpdateVars{}, *gaeTaskID))
	skutil.LogErr(util.SendTaskStartEmail(emailsArr, "Chromium analysis", *runID, *description))
	// Ensure webapp is updated and email is sent even if task fails.
	defer updateWebappTask()
	defer sendEmail(emailsArr, gs)
	// Cleanup dirs after run completes.
	defer skutil.RemoveAll(filepath.Join(util.StorageDir, util.BenchmarkRunsDir, *runID))
	// Finish with glog flush and how long the task took.
	defer util.TimeTrack(time.Now(), "Running chromium analysis task on workers")
	defer glog.Flush()

	if *pagesetType == "" {
		glog.Error("Must specify --pageset_type")
		return
	}
	if *benchmarkName == "" {
		glog.Error("Must specify --benchmark_name")
		return
	}
	if *runID == "" {
		glog.Error("Must specify --run_id")
		return
	}

	remoteOutputDir := filepath.Join(util.ChromiumAnalysisRunsDir, *runID)

	// Copy the patches to Google Storage.
	chromiumPatchName := *runID + ".chromium.patch"
	catapultPatchName := *runID + ".catapult.patch"
	benchmarkPatchName := *runID + ".benchmark.patch"
	for _, patchName := range []string{chromiumPatchName, catapultPatchName, benchmarkPatchName} {
		if err := gs.UploadFile(patchName, os.TempDir(), remoteOutputDir); err != nil {
			glog.Errorf("Could not upload %s to %s: %s", patchName, remoteOutputDir, err)
			return
		}
	}
	chromiumPatchLink = util.GS_HTTP_LINK + filepath.Join(util.GSBucketName, remoteOutputDir, chromiumPatchName)
	catapultPatchLink = util.GS_HTTP_LINK + filepath.Join(util.GSBucketName, remoteOutputDir, catapultPatchName)
	benchmarkPatchLink = util.GS_HTTP_LINK + filepath.Join(util.GSBucketName, remoteOutputDir, benchmarkPatchName)

	// Create the required chromium build.
	chromiumBuilds, err := util.TriggerBuildRepoSwarmingTask("build_chromium", *runID, "chromium", "Linux", []string{}, []string{filepath.Join(remoteOutputDir, chromiumPatchName)}, true /*singleBuild*/, 3*time.Hour, 1*time.Hour)
	if err != nil {
		glog.Errorf("Error encountered when swarming build repo task: %s", err)
		return
	}
	if len(chromiumBuilds) != 1 {
		glog.Errorf("Expected 1 build but instead got %d: %v", len(chromiumBuilds), chromiumBuilds)
		return
	}
	chromiumBuild := chromiumBuilds[0]

	// Archive, trigger and collect swarming tasks.
	isolateExtraArgs := map[string]string{
		"CHROMIUM_BUILD":     chromiumBuild,
		"RUN_ID":             *runID,
		"BENCHMARK":          *benchmarkName,
		"BENCHMARK_ARGS":     *benchmarkExtraArgs,
		"BROWSER_EXTRA_ARGS": *browserExtraArgs,
	}
	if err := util.TriggerSwarmingTask(*pagesetType, "chromium_analysis", util.CHROMIUM_ANALYSIS_ISOLATE, *runID, 3*time.Hour, 1*time.Hour, util.USER_TASKS_PRIORITY, MAX_PAGES_PER_SWARMING_BOT, isolateExtraArgs, util.GCE_WORKER_DIMENSIONS); err != nil {
		glog.Errorf("Error encountered when swarming tasks: %s", err)
		return
	}

	// If "--output-format=csv-pivot-table" was specified then merge all CSV files and upload.
	noOutputSlaves := []string{}
	pathToPyFiles := util.GetPathToPyFiles(false)
	if strings.Contains(*benchmarkExtraArgs, "--output-format=csv-pivot-table") {
		if noOutputSlaves, err = util.MergeUploadCSVFiles(*runID, pathToPyFiles, gs, util.PagesetTypeToInfo[*pagesetType].NumPages, MAX_PAGES_PER_SWARMING_BOT, true /* handleStrings */); err != nil {
			glog.Errorf("Unable to merge and upload CSV files for %s: %s", *runID, err)
		}
		// Cleanup created dir after the run completes.
		defer skutil.RemoveAll(filepath.Join(util.StorageDir, util.BenchmarkRunsDir, *runID))
	}

	// Construct the output link.
	outputLink = util.GS_HTTP_LINK + filepath.Join(util.GSBucketName, util.BenchmarkRunsDir, *runID, "consolidated_outputs", *runID+".output")

	// Display the no output slaves.
	for _, noOutputSlave := range noOutputSlaves {
		directLink := fmt.Sprintf(util.SWARMING_RUN_ID_TASK_LINK_PREFIX_TEMPLATE, *runID, "chromium_analysis_"+noOutputSlave)
		fmt.Printf("Missing output from %s\n", directLink)
	}

	taskCompletedSuccessfully = true
}
