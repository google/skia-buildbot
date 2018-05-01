// metrics_analysis_on_workers is an application that runs the
// analysis_metrics_ct benchmark on all CT workers and uploads results to Google
// Storage. The requester is emailed when the task is started and also after
// completion.
//
// Can be tested locally with:
// $ go run go/master_scripts/metrics_analysis_on_workers/main.go --run_id=rmistry-test1 --benchmark_extra_args="--output-format=csv" --logtostderr=true --description=testing --local --metric_name=loadingMetric
//
package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.skia.org/infra/ct/go/ctfe/metrics_analysis"
	"go.skia.org/infra/ct/go/frontend"
	"go.skia.org/infra/ct/go/master_scripts/master_common"
	"go.skia.org/infra/ct/go/util"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/email"
	"go.skia.org/infra/go/sklog"
	skutil "go.skia.org/infra/go/util"
)

const (
	// TODO(rmistry): What is the sweet spot here?
	MAX_PAGES_PER_SWARMING_BOT = 200
)

var (
	emails             = flag.String("emails", "", "The comma separated email addresses to notify when the task is picked up and completes.")
	description        = flag.String("description", "", "The description of the run as entered by the requester.")
	gaeTaskID          = flag.Int64("gae_task_id", -1, "The key of the App Engine task. This task will be updated when the task is completed.")
	benchmarkExtraArgs = flag.String("benchmark_extra_args", "", "The extra arguments that are passed to the analysis_metrics_ct benchmark.")
	runID              = flag.String("run_id", "", "The unique run id (typically requester + timestamp).")

	metricName    = flag.String("metric_name", "", "The metric to parse the traces with. Eg: loadingMetric")
	analysisRunId = flag.Int("analysis_run_id", -1, "Cloud trace links will be gathered from this specified CT analysis run Id.")

	taskCompletedSuccessfully = false

	chromiumPatchLink = ""
	catapultPatchLink = ""
	tracesLink        = ""
	outputLink        = ""
)

func sendEmail(recipients []string, gs *util.GcsUtil) {
	// Send completion email.
	emailSubject := fmt.Sprintf("Metrics analysis cluster telemetry task has completed (%s)", *runID)
	failureHtml := ""
	viewActionMarkup := ""
	var err error

	if taskCompletedSuccessfully {
		if viewActionMarkup, err = email.GetViewActionMarkup(outputLink, "View Results", "Direct link to the CSV results"); err != nil {
			sklog.Errorf("Failed to get view action markup: %s", err)
			return
		}
	} else {
		emailSubject += " with failures"
		failureHtml = util.GetFailureEmailHtml(*runID)
		if viewActionMarkup, err = email.GetViewActionMarkup(fmt.Sprintf(util.SWARMING_RUN_ID_ALL_TASKS_LINK_TEMPLATE, *runID), "View Failure", "Direct link to the swarming logs"); err != nil {
			sklog.Errorf("Failed to get view action markup: %s", err)
			return
		}
	}

	bodyTemplate := `
	The metrics analysis task has completed. %s.<br/>
	Run description: %s<br/>
	%s
	The CSV output is <a href='%s'>here</a>.%s<br/>
	The patch(es) you specified are here:
	<a href='%s'>chromium</a>/<a href='%s'>catapult</a>
	<br/>
	Traces used for this run are <a href='%s'>here</a>.
	<br/><br/>
	You can schedule more runs <a href='%s'>here</a>.
	<br/><br/>
	Thanks!
	`
	emailBody := fmt.Sprintf(bodyTemplate, util.GetSwarmingLogsLink(*runID), *description, failureHtml, outputLink, chromiumPatchLink, catapultPatchLink, tracesLink, frontend.MetricsAnalysisTasksWebapp)
	if err := util.SendEmailWithMarkup(recipients, emailSubject, emailBody, viewActionMarkup); err != nil {
		sklog.Errorf("Error while sending email: %s", err)
		return
	}
}

func updateWebappTask() {
	vars := metrics_analysis.UpdateVars{}
	vars.Id = *gaeTaskID
	vars.SetCompleted(taskCompletedSuccessfully)
	vars.RawOutput = sql.NullString{String: outputLink, Valid: true}
	skutil.LogErr(frontend.UpdateWebappTaskV2(&vars))
}

func main() {
	defer common.LogPanic()
	master_common.Init("run_metrics_analysis")

	ctx := context.Background()

	// Send start email.
	emailsArr := util.ParseEmails(*emails)
	emailsArr = append(emailsArr, util.CtAdmins...)
	if len(emailsArr) == 0 {
		sklog.Error("At least one email address must be specified")
		return
	}
	// Instantiate GcsUtil object.
	gs, err := util.NewGcsUtil(nil)
	if err != nil {
		sklog.Errorf("Could not instantiate gsutil object: %s", err)
		return
	}

	skutil.LogErr(frontend.UpdateWebappTaskSetStarted(&metrics_analysis.UpdateVars{}, *gaeTaskID, *runID))
	if !*master_common.Local {
		skutil.LogErr(util.SendTaskStartEmail(emailsArr, "Metrics analysis", *runID, *description))
		// Ensure webapp is updated and email is sent even if task fails.
		defer updateWebappTask()
		defer sendEmail(emailsArr, gs)
	}
	// Cleanup dirs after run completes.
	defer skutil.RemoveAll(filepath.Join(util.StorageDir, util.BenchmarkRunsDir, *runID))
	// Finish with glog flush and how long the task took.
	defer util.TimeTrack(time.Now(), "Running metrics analysis task on workers")
	defer sklog.Flush()

	if *runID == "" {
		sklog.Error("Must specify --run_id")
		return
	}
	if *metricName == "" {
		sklog.Error("Must specify --metric_name")
		return
	}

	remoteOutputDir := filepath.Join(util.MetricsAnalysisRunsDir, *runID)

	// Copy the patches and custom webpages to Google Storage.
	chromiumPatchName := *runID + ".chromium.patch"
	catapultPatchName := *runID + ".catapult.patch"
	tracesFileName := *runID + ".traces.csv"
	for _, patchName := range []string{chromiumPatchName, catapultPatchName} {
		if err := gs.UploadFile(patchName, os.TempDir(), remoteOutputDir); err != nil {
			sklog.Errorf("Could not upload %s to %s: %s", patchName, remoteOutputDir, err)
			return
		}
	}

	// If the tracesFile does not exist then see if an CT analysis run ID has been provided.
	// TODO(rmistry):

	fmt.Println("Do your work here!")
	sklog.Fatal("Do your work here!!")

	chromiumPatchLink = util.GCS_HTTP_LINK + filepath.Join(util.GCSBucketName, remoteOutputDir, chromiumPatchName)
	catapultPatchLink = util.GCS_HTTP_LINK + filepath.Join(util.GCSBucketName, remoteOutputDir, catapultPatchName)
	tracesLink = util.GCS_HTTP_LINK + filepath.Join(util.GCSBucketName, remoteOutputDir, tracesFileName)

	// TODO(rmistry): If analysis ID is specified then after verifying that it is correct
	//                download the output CSV and parse all traceURLs from it, use yuzus for examples of multiple traces..
	// TODO(rmistry): figure out the num pages per bot stuff based on the number of traceURLs from whichever source you get it from.

	// Find which chromium hash the workers should use.
	chromiumHash, err := util.GetChromiumHash(ctx)
	if err != nil {
		sklog.Error("Could not find the latest chromium hash")
		return
	}
	numPages := 3

	// Archive, trigger and collect swarming tasks.
	isolateExtraArgs := map[string]string{
		"RUN_ID":         *runID,
		"BENCHMARK_ARGS": *benchmarkExtraArgs,
		"METRIC_NAME":    *metricName,
		"CHROMIUM_HASH":  chromiumHash,
	}

	fmt.Println("THESE ARE THE ISOLATE EXTRA ARGS::::")
	fmt.Println(isolateExtraArgs)

	//customWebPagesFilePath := filepath.Join(os.TempDir(), customWebpagesName)
	//numPages, err := util.GetNumPages(*pagesetType, customWebPagesFilePath)
	//if err != nil {
	//	sklog.Errorf("Error encountered when calculating number of pages: %s", err)
	//	return
	//}
	// Calculate the max pages to run per bot.
	numSlaves, err := util.TriggerSwarmingTask(ctx, "" /* pagesetType */, "metrics_analysis", util.METRICS_ANALYSIS_ISOLATE, *runID, 12*time.Hour, 1*time.Hour, util.USER_TASKS_PRIORITY, MAX_PAGES_PER_SWARMING_BOT, numPages, isolateExtraArgs, true /* runOnGCE */, util.GetRepeatValue(*benchmarkExtraArgs, 1))
	if err != nil {
		sklog.Errorf("Error encountered when swarming tasks: %s", err)
		return
	}

	// If "--output-format=csv" is specified then merge all CSV files and upload.
	noOutputSlaves := []string{}
	pathToPyFiles := util.GetPathToPyFiles(false)
	if strings.Contains(*benchmarkExtraArgs, "--output-format=csv") {
		if noOutputSlaves, err = util.MergeUploadCSVFiles(ctx, *runID, pathToPyFiles, gs, numPages, MAX_PAGES_PER_SWARMING_BOT, true /* handleStrings */, 1 /* repeatValue */); err != nil {
			sklog.Errorf("Unable to merge and upload CSV files for %s: %s", *runID, err)
		}
		// Cleanup created dir after the run completes.
		defer skutil.RemoveAll(filepath.Join(util.StorageDir, util.BenchmarkRunsDir, *runID))
	}
	// If the number of noOutputSlaves is the same as the total number of triggered slaves then consider the run failed.
	if len(noOutputSlaves) == numSlaves {
		sklog.Errorf("All %d slaves produced no output", numSlaves)
		return
	}

	// Construct the output link.
	outputLink = util.GCS_HTTP_LINK + filepath.Join(util.GCSBucketName, util.BenchmarkRunsDir, *runID, "consolidated_outputs", *runID+".output")

	// Display the no output slaves.
	for _, noOutputSlave := range noOutputSlaves {
		directLink := fmt.Sprintf(util.SWARMING_RUN_ID_TASK_LINK_PREFIX_TEMPLATE, *runID, "metrics_analysis_"+noOutputSlave)
		fmt.Printf("Missing output from %s\n", directLink)
	}

	taskCompletedSuccessfully = true
}
