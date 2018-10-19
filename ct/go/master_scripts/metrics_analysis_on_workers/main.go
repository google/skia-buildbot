// metrics_analysis_on_workers is an application that runs the
// analysis_metrics_ct benchmark on all CT workers and uploads results to Google
// Storage. The requester is emailed when the task is started and also after
// completion.
//
// Can be tested locally with:
// $ go run go/master_scripts/metrics_analysis_on_workers/main.go --run_id=rmistry-test1 --benchmark_extra_args="--output-format=csv" --logtostderr=true --description=testing --metric_name=loadingMetric --analysis_output_link="https://ct.skia.org/results/cluster-telemetry/tasks/benchmark_runs/rmistry-20180502115012/consolidated_outputs/rmistry-20180502115012.output"
//
package main

import (
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.skia.org/infra/ct/go/ctfe/metrics_analysis"
	"go.skia.org/infra/ct/go/frontend"
	"go.skia.org/infra/ct/go/master_scripts/master_common"
	"go.skia.org/infra/ct/go/util"
	"go.skia.org/infra/go/email"
	"go.skia.org/infra/go/fileutil"
	"go.skia.org/infra/go/sklog"
	skutil "go.skia.org/infra/go/util"
)

const (
	MAX_PAGES_PER_SWARMING_BOT = 50
)

var (
	emails             = flag.String("emails", "", "The comma separated email addresses to notify when the task is picked up and completes.")
	description        = flag.String("description", "", "The description of the run as entered by the requester.")
	taskID             = flag.Int64("task_id", -1, "The key of the CT task in CTFE. The task will be updated when it is started and also when it completes.")
	benchmarkExtraArgs = flag.String("benchmark_extra_args", "", "The extra arguments that are passed to the analysis_metrics_ct benchmark.")
	runID              = flag.String("run_id", "", "The unique run id (typically requester + timestamp).")

	metricName         = flag.String("metric_name", "", "The metric to parse the traces with. Eg: loadingMetric")
	analysisOutputLink = flag.String("analysis_output_link", "", "Cloud trace links will be gathered from this specified CT analysis run Id. If not specified, trace links will be read from ${TMPDIR}/<run_id>.traces.csv")
	valueColumnName    = flag.String("value_column_name", "", "Which column's entries to use as field values when combining CSVs.")

	taskCompletedSuccessfully = false

	chromiumPatchLink = ""
	catapultPatchLink = ""
	tracesLink        = ""
	outputLink        = ""
)

func sendEmail(recipients []string, gs *util.GcsUtil) {
	// Send completion email.
	emailSubject := fmt.Sprintf("Metrics analysis cluster telemetry task has completed (#%d)", *taskID)
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
	The CSV output is <a href='%s'>here</a>.<br/>
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
	vars.Id = *taskID
	vars.SetCompleted(taskCompletedSuccessfully)
	vars.RawOutput = outputLink
	skutil.LogErr(frontend.UpdateWebappTaskV2(&vars))
}

func main() {
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

	skutil.LogErr(frontend.UpdateWebappTaskSetStarted(&metrics_analysis.UpdateVars{}, *taskID, *runID))
	skutil.LogErr(util.SendTaskStartEmail(*taskID, emailsArr, "Metrics analysis", *runID, *description))
	// Ensure webapp is updated and email is sent even if task fails.
	defer updateWebappTask()
	defer sendEmail(emailsArr, gs)
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

	// Use defaults.
	if *valueColumnName == "" {
		*valueColumnName = util.DEFAULT_VALUE_COLUMN_NAME
	}

	tracesFileName := *runID + ".traces.csv"
	tracesFilePath := filepath.Join(os.TempDir(), tracesFileName)
	if *analysisOutputLink != "" {
		if err := extractTracesFromAnalysisRun(tracesFilePath, gs); err != nil {
			sklog.Errorf("Error when extracting traces from run %s to %s: %s", *analysisOutputLink, tracesFilePath, err)
			return
		}
	}
	// Figure out how many traces we are dealing with.
	traces, err := util.GetCustomPages(tracesFilePath)
	if err != nil {
		sklog.Errorf("Could not read %s: %s", tracesFilePath, err)
		return
	}

	// Copy the patches and traces to Google Storage.
	remoteOutputDir := filepath.Join(util.BenchmarkRunsDir, *runID)
	chromiumPatchName := *runID + ".chromium.patch"
	catapultPatchName := *runID + ".catapult.patch"
	for _, patchName := range []string{chromiumPatchName, catapultPatchName, tracesFileName} {
		if err := gs.UploadFile(patchName, os.TempDir(), remoteOutputDir); err != nil {
			sklog.Errorf("Could not upload %s to %s: %s", patchName, remoteOutputDir, err)
			return
		}
	}
	chromiumPatchLink = util.GCS_HTTP_LINK + filepath.Join(util.GCSBucketName, remoteOutputDir, chromiumPatchName)
	catapultPatchLink = util.GCS_HTTP_LINK + filepath.Join(util.GCSBucketName, remoteOutputDir, catapultPatchName)
	tracesLink = util.GCS_HTTP_LINK + filepath.Join(util.GCSBucketName, remoteOutputDir, tracesFileName)

	// Find which chromium hash the workers should use.
	chromiumHash, err := util.GetChromiumHash(ctx)
	if err != nil {
		sklog.Error("Could not find the latest chromium hash")
		return
	}

	// Trigger task to return hash of telemetry isolates.
	telemetryIsolatePatches := []string{filepath.Join(remoteOutputDir, chromiumPatchName), filepath.Join(remoteOutputDir, catapultPatchName)}
	telemetryHash, err := util.TriggerIsolateTelemetrySwarmingTask(ctx, "isolate_telemetry", *runID, chromiumHash, *master_common.ServiceAccountFile, telemetryIsolatePatches, 1*time.Hour, 1*time.Hour, *master_common.Local)
	if err != nil {
		sklog.Errorf("Error encountered when swarming isolate telemetry task: %s", err)
		return
	}
	if telemetryHash == "" {
		sklog.Error("Found empty telemetry hash!")
		return
	}
	isolateDeps := []string{telemetryHash}

	// Calculate the max pages to run per bot.
	maxPagesPerBot := util.GetMaxPagesPerBotValue(*benchmarkExtraArgs, MAX_PAGES_PER_SWARMING_BOT)
	// Archive, trigger and collect swarming tasks.
	isolateExtraArgs := map[string]string{
		"RUN_ID":            *runID,
		"BENCHMARK_ARGS":    *benchmarkExtraArgs,
		"METRIC_NAME":       *metricName,
		"VALUE_COLUMN_NAME": *valueColumnName,
	}
	numSlaves, err := util.TriggerSwarmingTask(ctx, "" /* pagesetType */, "metrics_analysis", util.METRICS_ANALYSIS_ISOLATE, *runID, *master_common.ServiceAccountFile, 12*time.Hour, 3*time.Hour, util.USER_TASKS_PRIORITY, maxPagesPerBot, len(traces), isolateExtraArgs, true /* runOnGCE */, *master_common.Local, util.GetRepeatValue(*benchmarkExtraArgs, 1), isolateDeps)
	if err != nil {
		sklog.Errorf("Error encountered when swarming tasks: %s", err)
		return
	}

	// If "--output-format=csv" is specified then merge all CSV files and upload.
	noOutputSlaves := []string{}
	pathToPyFiles := util.GetPathToPyFiles(*master_common.Local, true /* runOnMaster */)
	if strings.Contains(*benchmarkExtraArgs, "--output-format=csv") {
		if noOutputSlaves, err = util.MergeUploadCSVFiles(ctx, *runID, pathToPyFiles, gs, len(traces), maxPagesPerBot, true /* handleStrings */, util.GetRepeatValue(*benchmarkExtraArgs, 1)); err != nil {
			sklog.Errorf("Unable to merge and upload CSV files for %s: %s", *runID, err)
		}
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

// extractTracesFromAnalysisRuns gathers all traceURLs from the specified analysis
// run and writes to the specified outputPath.
func extractTracesFromAnalysisRun(outputPath string, gs *util.GcsUtil) error {
	// Construct path to the google storage locations and download it locally.
	remoteCsvPath := strings.Split(*analysisOutputLink, util.GCSBucketName+"/")[1]
	localDownloadPath := filepath.Join(util.StorageDir, util.BenchmarkRunsDir, *runID, "downloads")
	localCsvPath := filepath.Join(localDownloadPath, *runID+".csv")
	if err := fileutil.EnsureDirPathExists(localCsvPath); err != nil {
		return fmt.Errorf("Could not create %s: %s", localDownloadPath, err)
	}
	defer skutil.RemoveAll(localDownloadPath)
	if err := gs.DownloadRemoteFile(remoteCsvPath, localCsvPath); err != nil {
		return fmt.Errorf("Error downloading %s to %s: %s", remoteCsvPath, localCsvPath, err)
	}

	headers, values, err := util.GetRowsFromCSV(localCsvPath)
	if err != nil {
		return fmt.Errorf("Could not read %s: %s. Analysis output link: %s", localCsvPath, err, *analysisOutputLink)
	}
	// Gather trace URLs from the CSV.
	traceURLs := []string{}
	for i := range headers {
		if headers[i] == "traceUrls" {
			for j := range values {
				if values[j][i] != "" {
					traceURLs = append(traceURLs, values[j][i])
				}
			}
		}
	}
	if len(traceURLs) == 0 {
		return fmt.Errorf("There were no traceURLs found for the analysis output link: %s", *analysisOutputLink)
	}
	if err := ioutil.WriteFile(outputPath, []byte(strings.Join(traceURLs, ",")), 0644); err != nil {
		return fmt.Errorf("Could not write traceURLs to %s: %s", outputPath, err)
	}
	return nil
}
