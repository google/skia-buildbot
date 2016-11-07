// run_chromium_perf_on_workers is an application that runs the specified telemetry
// benchmark on all CT workers and uploads the results to Google Storage. The
// requester is emailed when the task is done.
package main

import (
	"database/sql"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/skia-dev/glog"
	"go.skia.org/infra/ct/go/ctfe/chromium_perf"
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
	emails                    = flag.String("emails", "", "The comma separated email addresses to notify when the task is picked up and completes.")
	description               = flag.String("description", "", "The description of the run as entered by the requester.")
	gaeTaskID                 = flag.Int64("gae_task_id", -1, "The key of the App Engine task. This task will be updated when the task is completed.")
	pagesetType               = flag.String("pageset_type", "", "The type of pagesets to use. Eg: 10k, Mobile10k, All.")
	benchmarkName             = flag.String("benchmark_name", "", "The telemetry benchmark to run on the workers.")
	benchmarkExtraArgs        = flag.String("benchmark_extra_args", "", "The extra arguments that are passed to the specified benchmark.")
	browserExtraArgsNoPatch   = flag.String("browser_extra_args_nopatch", "", "The extra arguments that are passed to the browser while running the benchmark for the nopatch case.")
	browserExtraArgsWithPatch = flag.String("browser_extra_args_withpatch", "", "The extra arguments that are passed to the browser while running the benchmark for the withpatch case.")
	repeatBenchmark           = flag.Int("repeat_benchmark", 1, "The number of times the benchmark should be repeated. For skpicture_printer benchmark this value is always 1.")
	runInParallel             = flag.Bool("run_in_parallel", false, "Run the benchmark by bringing up multiple chrome instances in parallel.")
	targetPlatform            = flag.String("target_platform", util.PLATFORM_ANDROID, "The platform the benchmark will run on (Android / Linux).")
	runID                     = flag.String("run_id", "", "The unique run id (typically requester + timestamp).")
	varianceThreshold         = flag.Float64("variance_threshold", 0.0, "The variance threshold to use when comparing the resultant CSV files.")
	discardOutliers           = flag.Float64("discard_outliers", 0.0, "The percentage of outliers to discard when comparing the result CSV files.")

	taskCompletedSuccessfully = false

	htmlOutputLink      = util.MASTER_LOGSERVER_LINK
	skiaPatchLink       = util.MASTER_LOGSERVER_LINK
	chromiumPatchLink   = util.MASTER_LOGSERVER_LINK
	catapultPatchLink   = util.MASTER_LOGSERVER_LINK
	benchmarkPatchLink  = util.MASTER_LOGSERVER_LINK
	noPatchOutputLink   = util.MASTER_LOGSERVER_LINK
	withPatchOutputLink = util.MASTER_LOGSERVER_LINK
)

func sendEmail(recipients []string) {
	// Send completion email.
	emailSubject := fmt.Sprintf("Cluster telemetry chromium perf task has completed (%s)", *runID)
	failureHtml := ""
	viewActionMarkup := ""
	var err error

	if taskCompletedSuccessfully {
		if viewActionMarkup, err = email.GetViewActionMarkup(htmlOutputLink, "View Results", "Direct link to the HTML results"); err != nil {
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
	bodyTemplate := `
	The chromium perf %s benchmark task on %s pageset has completed. %s.<br/>
	Run description: %s<br/>
	%s
	The HTML output with differences between the base run and the patch run is <a href='%s'>here</a>.<br/>
	The patch(es) you specified are here:
	<a href='%s'>chromium</a>/<a href='%s'>skia</a>/<a href='%s'>catapult</a>
	<br/><br/>
	You can schedule more runs <a href='%s'>here</a>.
	<br/><br/>
	Thanks!
	`
	emailBody := fmt.Sprintf(bodyTemplate, *benchmarkName, *pagesetType, util.GetSwarmingLogsLink(*runID), *description, failureHtml, htmlOutputLink, chromiumPatchLink, skiaPatchLink, catapultPatchLink, frontend.ChromiumPerfTasksWebapp)
	if err := util.SendEmailWithMarkup(recipients, emailSubject, emailBody, viewActionMarkup); err != nil {
		glog.Errorf("Error while sending email: %s", err)
		return
	}
}

func updateWebappTask() {
	vars := chromium_perf.UpdateVars{}
	vars.Id = *gaeTaskID
	vars.SetCompleted(taskCompletedSuccessfully)
	vars.Results = sql.NullString{String: htmlOutputLink, Valid: true}
	vars.NoPatchRawOutput = sql.NullString{String: noPatchOutputLink, Valid: true}
	vars.WithPatchRawOutput = sql.NullString{String: withPatchOutputLink, Valid: true}
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
	skutil.LogErr(frontend.UpdateWebappTaskSetStarted(&chromium_perf.UpdateVars{}, *gaeTaskID))
	skutil.LogErr(util.SendTaskStartEmail(emailsArr, "Chromium perf", *runID, *description))
	// Ensure webapp is updated and email is sent even if task fails.
	defer updateWebappTask()
	defer sendEmail(emailsArr)
	// Cleanup dirs after run completes.
	defer skutil.RemoveAll(filepath.Join(util.StorageDir, util.ChromiumPerfRunsDir, *runID))
	// Finish with glog flush and how long the task took.
	defer util.TimeTrack(time.Now(), "Running chromium perf task on workers")
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
	if *description == "" {
		glog.Error("Must specify --description")
		return
	}

	// Instantiate GsUtil object.
	gs, err := util.NewGsUtil(nil)
	if err != nil {
		glog.Errorf("Could not instantiate gsutil object: %s", err)
		return
	}
	remoteOutputDir := filepath.Join(util.ChromiumPerfRunsDir, *runID)

	// Copy the patches to Google Storage.
	skiaPatchName := *runID + ".skia.patch"
	chromiumPatchName := *runID + ".chromium.patch"
	catapultPatchName := *runID + ".catapult.patch"
	benchmarkPatchName := *runID + ".benchmark.patch"
	for _, patchName := range []string{skiaPatchName, chromiumPatchName, catapultPatchName, benchmarkPatchName} {
		if err := gs.UploadFile(patchName, os.TempDir(), remoteOutputDir); err != nil {
			glog.Errorf("Could not upload %s to %s: %s", patchName, remoteOutputDir, err)
			return
		}
	}
	skiaPatchLink = util.GS_HTTP_LINK + filepath.Join(util.GSBucketName, remoteOutputDir, skiaPatchName)
	chromiumPatchLink = util.GS_HTTP_LINK + filepath.Join(util.GSBucketName, remoteOutputDir, chromiumPatchName)
	catapultPatchLink = util.GS_HTTP_LINK + filepath.Join(util.GSBucketName, remoteOutputDir, catapultPatchName)
	benchmarkPatchLink = util.GS_HTTP_LINK + filepath.Join(util.GSBucketName, remoteOutputDir, benchmarkPatchName)

	// Create the two required chromium builds (with patch and without the patch).
	chromiumBuilds, err := util.TriggerBuildRepoSwarmingTask(
		"build_chromium", *runID, "chromium", *targetPlatform, []string{},
		[]string{filepath.Join(remoteOutputDir, chromiumPatchName), filepath.Join(remoteOutputDir, skiaPatchName)},
		/*singlebuild*/ false, 3*time.Hour, 1*time.Hour)
	if err != nil {
		glog.Errorf("Error encountered when swarming build repo task: %s", err)
		return
	}
	if len(chromiumBuilds) != 2 {
		glog.Errorf("Expected 2 builds but instead got %d: %v.", len(chromiumBuilds), chromiumBuilds)
		return
	}
	chromiumBuildNoPatch := chromiumBuilds[0]
	chromiumBuildWithPatch := chromiumBuilds[1]

	// Parse out the Chromium and Skia hashes.
	chromiumHash, skiaHash := util.GetHashesFromBuild(chromiumBuildNoPatch)

	// Archive, trigger and collect swarming tasks.
	isolateExtraArgs := map[string]string{
		"CHROMIUM_BUILD_NOPATCH":       chromiumBuildNoPatch,
		"CHROMIUM_BUILD_WITHPATCH":     chromiumBuildWithPatch,
		"RUN_ID":                       *runID,
		"BENCHMARK":                    *benchmarkName,
		"BENCHMARK_ARGS":               *benchmarkExtraArgs,
		"BROWSER_EXTRA_ARGS_NOPATCH":   *browserExtraArgsNoPatch,
		"BROWSER_EXTRA_ARGS_WITHPATCH": *browserExtraArgsWithPatch,
		"REPEAT_BENCHMARK":             strconv.Itoa(*repeatBenchmark),
		"RUN_IN_PARALLEL":              strconv.FormatBool(*runInParallel),
		"TARGET_PLATFORM":              *targetPlatform,
	}
	if err := util.TriggerSwarmingTask(*pagesetType, "chromium_perf", util.CHROMIUM_PERF_ISOLATE, *runID, 12*time.Hour, 1*time.Hour, util.USER_TASKS_PRIORITY, MAX_PAGES_PER_SWARMING_BOT, util.PagesetTypeToInfo[*pagesetType].NumPages, isolateExtraArgs, util.GOLO_WORKER_DIMENSIONS); err != nil {
		glog.Errorf("Error encountered when swarming tasks: %s", err)
		return
	}

	// If "--output-format=csv-pivot-table" was specified then merge all CSV files and upload.
	runIDNoPatch := fmt.Sprintf("%s-nopatch", *runID)
	runIDWithPatch := fmt.Sprintf("%s-withpatch", *runID)
	noOutputSlaves := []string{}
	pathToPyFiles := util.GetPathToPyFiles(false)
	for _, run := range []string{runIDNoPatch, runIDWithPatch} {
		if strings.Contains(*benchmarkExtraArgs, "--output-format=csv-pivot-table") {
			if noOutputSlaves, err = util.MergeUploadCSVFiles(run, pathToPyFiles, gs, util.PagesetTypeToInfo[*pagesetType].NumPages, MAX_PAGES_PER_SWARMING_BOT, true /* handleStrings */); err != nil {
				glog.Errorf("Unable to merge and upload CSV files for %s: %s", run, err)
			}
			// Cleanup created dir after the run completes.
			defer skutil.RemoveAll(filepath.Join(util.StorageDir, util.BenchmarkRunsDir, run))
		}
	}
	totalArchivedWebpages, err := util.GetArchivesNum(gs, *benchmarkExtraArgs, *pagesetType)
	if err != nil {
		glog.Errorf("Error when calculating number of archives: %s", err)
		return
	}

	// Compare the resultant CSV files using csv_comparer.py
	noPatchCSVPath := filepath.Join(util.StorageDir, util.BenchmarkRunsDir, runIDNoPatch, runIDNoPatch+".output")
	withPatchCSVPath := filepath.Join(util.StorageDir, util.BenchmarkRunsDir, runIDWithPatch, runIDWithPatch+".output")
	htmlOutputDir := filepath.Join(util.StorageDir, util.ChromiumPerfRunsDir, *runID, "html")
	skutil.MkdirAll(htmlOutputDir, 0700)
	htmlRemoteDir := filepath.Join(remoteOutputDir, "html")
	htmlOutputLinkBase := util.GS_HTTP_LINK + filepath.Join(util.GSBucketName, htmlRemoteDir) + "/"
	htmlOutputLink = htmlOutputLinkBase + "index.html"
	noPatchOutputLink = util.GS_HTTP_LINK + filepath.Join(util.GSBucketName, util.BenchmarkRunsDir, runIDNoPatch, "consolidated_outputs", runIDNoPatch+".output")
	withPatchOutputLink = util.GS_HTTP_LINK + filepath.Join(util.GSBucketName, util.BenchmarkRunsDir, runIDWithPatch, "consolidated_outputs", runIDWithPatch+".output")
	// Construct path to the csv_comparer python script.
	pathToCsvComparer := filepath.Join(pathToPyFiles, "csv_comparer.py")
	args := []string{
		pathToCsvComparer,
		"--csv_file1=" + noPatchCSVPath,
		"--csv_file2=" + withPatchCSVPath,
		"--output_html=" + htmlOutputDir,
		"--variance_threshold=" + strconv.FormatFloat(*varianceThreshold, 'f', 2, 64),
		"--discard_outliers=" + strconv.FormatFloat(*discardOutliers, 'f', 2, 64),
		"--absolute_url=" + htmlOutputLinkBase,
		"--requester_email=" + *emails,
		"--skia_patch_link=" + skiaPatchLink,
		"--chromium_patch_link=" + chromiumPatchLink,
		"--benchmark_patch_link=" + benchmarkPatchLink,
		"--description=" + *description,
		"--raw_csv_nopatch=" + noPatchOutputLink,
		"--raw_csv_withpatch=" + withPatchOutputLink,
		"--num_repeated=" + strconv.Itoa(*repeatBenchmark),
		"--target_platform=" + *targetPlatform,
		"--browser_args_nopatch=" + *browserExtraArgsNoPatch,
		"--browser_args_withpatch=" + *browserExtraArgsWithPatch,
		"--pageset_type=" + *pagesetType,
		"--chromium_hash=" + chromiumHash,
		"--skia_hash=" + skiaHash,
		"--missing_output_slaves=" + strings.Join(noOutputSlaves, " "),
		"--logs_link_prefix=" + fmt.Sprintf(util.SWARMING_RUN_ID_TASK_LINK_PREFIX_TEMPLATE, *runID, "chromium_perf_"),
		"--total_archives=" + strconv.Itoa(totalArchivedWebpages),
	}
	// TODO(rmistry): Remove the below debugging stmt.
	glog.Errorf("Args of csv_comparer.py: %v", args)
	err = util.ExecuteCmd("python", args, []string{}, util.CSV_COMPARER_TIMEOUT, nil, nil)
	if err != nil {
		glog.Errorf("Error running csv_comparer.py: %s", err)
		return
	}

	// Copy the HTML files to Google Storage.
	if err := gs.UploadDir(htmlOutputDir, htmlRemoteDir, true); err != nil {
		glog.Errorf("Could not upload %s to %s: %s", htmlOutputDir, htmlRemoteDir, err)
		return
	}

	taskCompletedSuccessfully = true
}
