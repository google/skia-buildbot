// metrics_analysis_on_workers is an application that runs the
// analysis_metrics_ct benchmark on all CT workers and uploads results to Google
// Storage. The requester is emailed when the task is started and also after
// completion.
//
// Can be tested locally with:
// $ go run go/master_scripts/metrics_analysis_on_workers/main.go --run_id=rmistry-test1 --benchmark_extra_args="--output-format=csv" --description=testing --metric_name=loadingMetric --analysis_output_link="https://ct.skia.org/results/cluster-telemetry/tasks/benchmark_runs/rmistry-20180502115012/consolidated_outputs/rmistry-20180502115012.output"
//
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.skia.org/infra/ct/go/master_scripts/master_common"
	"go.skia.org/infra/ct/go/util"
	"go.skia.org/infra/go/fileutil"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	skutil "go.skia.org/infra/go/util"
)

const (
	MAX_PAGES_PER_SWARMING_BOT = 50
)

var (
	benchmarkExtraArgs = flag.String("benchmark_extra_args", "", "The extra arguments that are passed to the analysis_metrics_ct benchmark.")
	runID              = flag.String("run_id", "", "The unique run id (typically requester + timestamp).")

	metricName         = flag.String("metric_name", "", "The metric to parse the traces with. Eg: loadingMetric")
	analysisOutputLink = flag.String("analysis_output_link", "", "Cloud trace links will be gathered from this specified CT analysis run Id. If not specified, trace links will be read from ${TMPDIR}/<run_id>.traces.csv")
	valueColumnName    = flag.String("value_column_name", "", "Which column's entries to use as field values when combining CSVs.")
	taskPriority       = flag.Int("task_priority", util.TASKS_PRIORITY_MEDIUM, "The priority swarming tasks should run at.")

	chromiumPatchGSPath   = flag.String("chromium_patch_gs_path", "", "The location of the Chromium patch in Google storage.")
	catapultPatchGSPath   = flag.String("catapult_patch_gs_path", "", "The location of the Catapult patch in Google storage.")
	customTracesCsvGSPath = flag.String("custom_traces_csv_gs_path", "", "The location of the custom traces CSV in Google storage.")
)

func metricsAnalysisOnWorkers() error {
	swarmingClient, casClient, err := master_common.Init("run_metrics_analysis")
	if err != nil {
		return fmt.Errorf("Could not init: %s", err)
	}

	ctx := context.Background()

	// Instantiate GcsUtil object.
	gs, err := util.NewGcsUtil(nil)
	if err != nil {
		return fmt.Errorf("Could not instantiate gsutil object: %s", err)
	}

	// Cleanup dirs after run completes.
	defer skutil.RemoveAll(filepath.Join(util.StorageDir, util.BenchmarkRunsDir, *runID))
	// Finish with glog flush and how long the task took.
	defer util.TimeTrack(time.Now(), "Running metrics analysis task on workers")
	defer sklog.Flush()

	if *runID == "" {
		return errors.New("Must specify --run_id")
	}
	if *metricName == "" {
		return errors.New("Must specify --metric_name")
	}

	// Use defaults.
	if *valueColumnName == "" {
		*valueColumnName = util.DEFAULT_VALUE_COLUMN_NAME
	}

	for fileSuffix, patchGSPath := range map[string]string{
		".chromium.patch": *chromiumPatchGSPath,
		".catapult.patch": *catapultPatchGSPath,
		".traces.csv":     *customTracesCsvGSPath,
	} {
		patch, err := util.GetPatchFromStorage(patchGSPath)
		if err != nil {
			return fmt.Errorf("Could not download patch %s from Google storage: %s", patchGSPath, err)
		}
		// Add an extra newline at the end because git sometimes rejects patches due to
		// missing newlines.
		patch = patch + "\n"
		patchPath := filepath.Join(os.TempDir(), *runID+fileSuffix)
		if err := ioutil.WriteFile(patchPath, []byte(patch), 0666); err != nil {
			return fmt.Errorf("Could not write patch %s to %s: %s", patch, patchPath, err)
		}
		defer skutil.Remove(patchPath)
	}

	tracesFileName := *runID + ".traces.csv"
	tracesFilePath := filepath.Join(os.TempDir(), tracesFileName)
	if *analysisOutputLink != "" {
		if err := extractTracesFromAnalysisRun(tracesFilePath, gs); err != nil {
			return fmt.Errorf("Error when extracting traces from run %s to %s: %s", *analysisOutputLink, tracesFilePath, err)
		}
	}
	// Figure out how many traces we are dealing with.
	traces, err := util.GetCustomPages(tracesFilePath)
	if err != nil {
		return fmt.Errorf("Could not read %s: %s", tracesFilePath, err)
	}

	// Copy the patches and traces to Google Storage.
	remoteOutputDir := filepath.Join(util.BenchmarkRunsDir, *runID)
	chromiumPatchName := *runID + ".chromium.patch"
	catapultPatchName := *runID + ".catapult.patch"
	for _, patchName := range []string{chromiumPatchName, catapultPatchName, tracesFileName} {
		if err := gs.UploadFile(patchName, os.TempDir(), remoteOutputDir); err != nil {
			return fmt.Errorf("Could not upload %s to %s: %s", patchName, remoteOutputDir, err)
		}
	}

	// Find which chromium hash the workers should use.
	gitExec, err := git.Executable(ctx)
	if err != nil {
		return skerr.Wrap(err)
	}
	chromiumHash, err := util.GetChromiumHash(ctx, gitExec)
	if err != nil {
		return fmt.Errorf("Could not find the latest chromium hash: %s", err)
	}

	// Trigger task to return hash of telemetry isolates.
	telemetryIsolatePatches := []string{filepath.Join(remoteOutputDir, chromiumPatchName), filepath.Join(remoteOutputDir, catapultPatchName)}
	telemetryHash, err := util.TriggerIsolateTelemetrySwarmingTask(ctx, "isolate_telemetry", *runID, chromiumHash, "", util.PLATFORM_LINUX, telemetryIsolatePatches, 1*time.Hour, 1*time.Hour, *master_common.Local, swarmingClient, casClient)
	if err != nil {
		return fmt.Errorf("Error encountered when swarming isolate telemetry task: %s", err)
	}
	if telemetryHash == "" {
		return errors.New("Found empty telemetry hash!")
	}

	// Calculate the max pages to run per bot.
	maxPagesPerBot := util.GetMaxPagesPerBotValue(*benchmarkExtraArgs, MAX_PAGES_PER_SWARMING_BOT)
	// Archive, trigger and collect swarming tasks.
	baseCmd := []string{
		"luci-auth",
		"context",
		"--",
		"bin/metrics_analysis",
		"-logtostderr",
		"--run_id=" + *runID,
		"--benchmark_extra_args=" + *benchmarkExtraArgs,
		"--metric_name=" + *metricName,
		"--value_column_name=" + *valueColumnName,
	}
	casSpec := util.CasMetricsAnalysis()
	casSpec.IncludeDigests = append(casSpec.IncludeDigests, telemetryHash)
	numWorkers, err := util.TriggerSwarmingTask(ctx, "" /* pagesetType */, "metrics_analysis", *runID, util.PLATFORM_LINUX, casSpec, 12*time.Hour, 3*time.Hour, *taskPriority, maxPagesPerBot, len(traces), true /* runOnGCE */, *master_common.Local, util.GetRepeatValue(*benchmarkExtraArgs, 1), baseCmd, swarmingClient, casClient)
	if err != nil {
		return fmt.Errorf("Error encountered when swarming tasks: %s", err)
	}

	// If "--output-format=csv" is specified then merge all CSV files and upload.
	noOutputWorkers := []string{}
	pathToPyFiles, err := util.GetPathToPyFiles(*master_common.Local)
	if err != nil {
		return fmt.Errorf("Could not get path to py files: %s", err)
	}
	if strings.Contains(*benchmarkExtraArgs, "--output-format=csv") {
		if _, noOutputWorkers, err = util.MergeUploadCSVFiles(ctx, *runID, pathToPyFiles, gs, len(traces), maxPagesPerBot, true /* handleStrings */, util.GetRepeatValue(*benchmarkExtraArgs, 1)); err != nil {
			sklog.Errorf("Unable to merge and upload CSV files for %s: %s", *runID, err)
		}
	}
	// If the number of noOutputWorkers is the same as the total number of triggered workers then consider the run failed.
	if len(noOutputWorkers) == numWorkers {
		return fmt.Errorf("All %d workers produced no output", numWorkers)
	}

	// Display the no output workers.
	for _, noOutputWorker := range noOutputWorkers {
		directLink := fmt.Sprintf(util.SWARMING_RUN_ID_TASK_LINK_PREFIX_TEMPLATE, *runID, "metrics_analysis_"+noOutputWorker)
		fmt.Printf("Missing output from %s\n", directLink)
	}

	return nil
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

func main() {
	retCode := 0
	if err := metricsAnalysisOnWorkers(); err != nil {
		sklog.Errorf("Error while running metrics analysis on workers: %s", err)
		retCode = 255
	}
	os.Exit(retCode)
}
