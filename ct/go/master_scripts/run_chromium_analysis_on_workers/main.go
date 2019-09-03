// run_chromium_analysis_on_workers is an application that runs the specified
// telemetry benchmark on swarming bots and uploads the results to Google
// Storage. The requester is emailed when the task is done.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"time"

	"go.skia.org/infra/ct/go/ctfe/chromium_analysis"
	"go.skia.org/infra/ct/go/ctfe/task_common"
	"go.skia.org/infra/ct/go/master_scripts/master_common"
	"go.skia.org/infra/ct/go/util"
	"go.skia.org/infra/go/email"
	"go.skia.org/infra/go/sklog"
	skutil "go.skia.org/infra/go/util"
)

const (
	MAX_PAGES_PER_SWARMING_BOT = 100
)

var (
	emails             = flag.String("emails", "", "The comma separated email addresses to notify when the task is picked up and completes.")
	description        = flag.String("description", "", "The description of the run as entered by the requester.")
	taskID             = flag.Int64("task_id", -1, "The key of the CT task in CTFE. The task will be updated when it is started and also when it completes.")
	pagesetType        = flag.String("pageset_type", "", "The type of pagesets to use. Eg: 10k, Mobile10k, All.")
	benchmarkName      = flag.String("benchmark_name", "", "The telemetry benchmark to run on the workers.")
	benchmarkExtraArgs = flag.String("benchmark_extra_args", "", "The extra arguments that are passed to the specified benchmark.")
	browserExtraArgs   = flag.String("browser_extra_args", "", "The extra arguments that are passed to the browser while running the benchmark.")
	runID              = flag.String("run_id", "", "The unique run id (typically requester + timestamp).")
	targetPlatform     = flag.String("target_platform", util.PLATFORM_LINUX, "The platform the benchmark will run on (Android / Linux).")
	runOnGCE           = flag.Bool("run_on_gce", true, "Run on Linux GCE instances. Used only if Linux is used for the target_platform.")
	runInParallel      = flag.Bool("run_in_parallel", true, "Run the benchmark by bringing up multiple chrome instances in parallel.")
	matchStdoutText    = flag.String("match_stdout_txt", "", "Looks for the specified string in the stdout of web page runs. The count of the text's occurence and the lines containing it are added to the CSV of the web page.")
	chromiumHash       = flag.String("chromium_hash", "", "The Chromium full hash the checkout should be synced to before applying patches.")
	taskPriority       = flag.Int("task_priority", util.TASKS_PRIORITY_MEDIUM, "The priority swarming tasks should run at.")
	groupName          = flag.String("group_name", "", "The group name of this run. It will be used as the key when uploading data to ct-perf.skia.org.")

	chromiumPatchGSPath     = flag.String("chromium_patch_gs_path", "", "The location of the Chromium patch in Google storage.")
	skiaPatchGSPath         = flag.String("skia_patch_gs_path", "", "The location of the Skia patch in Google storage.")
	v8PatchGSPath           = flag.String("v8_patch_gs_path", "", "The location of the V8 patch in Google storage.")
	catapultPatchGSPath     = flag.String("catapult_patch_gs_path", "", "The location of the Catapult patch in Google storage.")
	customWebpagesCsvGSPath = flag.String("custom_webpages_csv_gs_path", "", "The location of the custom webpages CSV in Google storage.")

	taskCompletedSuccessfully = false

	chromiumPatchLink  = util.MASTER_LOGSERVER_LINK
	skiaPatchLink      = util.MASTER_LOGSERVER_LINK
	v8PatchLink        = util.MASTER_LOGSERVER_LINK
	catapultPatchLink  = util.MASTER_LOGSERVER_LINK
	customWebpagesLink = util.MASTER_LOGSERVER_LINK
	outputLink         = util.MASTER_LOGSERVER_LINK
)

func sendEmail(recipients []string, gs *util.GcsUtil) {
	// Send completion email.
	emailSubject := fmt.Sprintf("Cluster telemetry chromium analysis task has completed (#%d)", *taskID)
	failureHtml := ""
	viewActionMarkup := ""
	ctPerfHtml := ""
	var err error

	if taskCompletedSuccessfully {
		if viewActionMarkup, err = email.GetViewActionMarkup(outputLink, "View Results", "Direct link to the CSV results"); err != nil {
			sklog.Errorf("Failed to get view action markup: %s", err)
			return
		}
		ctPerfHtml = util.GetCTPerfEmailHtml(*groupName)
	} else {
		emailSubject += " with failures"
		failureHtml = util.GetFailureEmailHtml(*runID)
		if viewActionMarkup, err = email.GetViewActionMarkup(fmt.Sprintf(util.SWARMING_RUN_ID_ALL_TASKS_LINK_TEMPLATE, *runID), "View Failure", "Direct link to the swarming logs"); err != nil {
			sklog.Errorf("Failed to get view action markup: %s", err)
			return
		}
	}

	totalArchivedWebpages, err := util.GetArchivesNum(gs, *benchmarkExtraArgs, *pagesetType)
	if err != nil {
		sklog.Errorf("Error when calculating number of archives: %s", err)
		totalArchivedWebpages = -1
	}
	archivedWebpagesText := ""
	if totalArchivedWebpages != -1 {
		archivedWebpagesText = fmt.Sprintf(" %d WPR archives were used.", totalArchivedWebpages)
	}

	bodyTemplate := `
	The chromium analysis %s benchmark task on %s pageset has completed. %s.<br/>
	Run description: %s<br/>
	%s
	%s
	The CSV output is <a href='%s'>here</a>.%s<br/>
	The patch(es) you specified are here:
	<a href='%s'>chromium</a>/<a href='%s'>skia</a>/<a href='%s'>v8</a>/<a href='%s'>catapult</a>
	<br/>
	Custom webpages (if specified) are <a href='%s'>here</a>.
	<br/><br/>
	You can schedule more runs <a href='%s'>here</a>.
	<br/><br/>
	Thanks!
	`
	emailBody := fmt.Sprintf(bodyTemplate, *benchmarkName, *pagesetType, util.GetSwarmingLogsLink(*runID), *description, failureHtml, ctPerfHtml, outputLink, archivedWebpagesText, chromiumPatchLink, skiaPatchLink, v8PatchLink, catapultPatchLink, customWebpagesLink, master_common.ChromiumAnalysisTasksWebapp)
	if err := util.SendEmailWithMarkup(recipients, emailSubject, emailBody, viewActionMarkup); err != nil {
		sklog.Errorf("Error while sending email: %s", err)
		return
	}
}

func updateTaskInDatastore(ctx context.Context) {
	vars := chromium_analysis.UpdateVars{}
	vars.Id = *taskID
	vars.SetCompleted(taskCompletedSuccessfully)
	vars.RawOutput = outputLink
	skutil.LogErr(task_common.FindAndUpdateTask(ctx, &vars))
}

func runChromiumAnalysisOnWorkers() error {
	master_common.Init("run_chromium_analysis")

	ctx := context.Background()

	// Send start email.
	emailsArr := util.ParseEmails(*emails)
	emailsArr = append(emailsArr, util.CtAdmins...)
	if len(emailsArr) == 0 {
		return errors.New("At least one email address must be specified")
	}
	// Instantiate GcsUtil object.
	gs, err := util.NewGcsUtil(nil)
	if err != nil {
		return fmt.Errorf("Could not instantiate gsutil object: %s", err)
	}

	skutil.LogErr(task_common.UpdateTaskSetStarted(ctx, &chromium_analysis.UpdateVars{}, *taskID, *runID))
	skutil.LogErr(util.SendTaskStartEmail(*taskID, emailsArr, "Chromium analysis", *runID, *description, fmt.Sprintf("Triggered %s benchmark on %s %s pageset.", *benchmarkName, *targetPlatform, *pagesetType)))
	// Ensure task is updated and email is sent even if task fails.
	defer updateTaskInDatastore(ctx)
	defer sendEmail(emailsArr, gs)
	// Cleanup dirs after run completes.
	defer skutil.RemoveAll(filepath.Join(util.StorageDir, util.BenchmarkRunsDir, *runID))
	// Finish with glog flush and how long the task took.
	defer util.TimeTrack(time.Now(), "Running chromium analysis task on workers")
	defer sklog.Flush()

	if *pagesetType == "" {
		return errors.New("Must specify --pageset_type")
	}
	if *benchmarkName == "" {
		return errors.New("Must specify --benchmark_name")
	}
	if *runID == "" {
		return errors.New("Must specify --run_id")
	}

	remoteOutputDir := path.Join(util.ChromiumAnalysisRunsStorageDir, *runID)

	for fileSuffix, patchGSPath := range map[string]string{
		".chromium.patch":      *chromiumPatchGSPath,
		".skia.patch":          *skiaPatchGSPath,
		".v8.patch":            *v8PatchGSPath,
		".catapult.patch":      *catapultPatchGSPath,
		".custom_webpages.csv": *customWebpagesCsvGSPath,
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

	// Copy the patches and custom webpages to Google Storage.
	chromiumPatchName := *runID + ".chromium.patch"
	skiaPatchName := *runID + ".skia.patch"
	v8PatchName := *runID + ".v8.patch"
	catapultPatchName := *runID + ".catapult.patch"
	customWebpagesName := *runID + ".custom_webpages.csv"
	for _, patchName := range []string{chromiumPatchName, v8PatchName, skiaPatchName, catapultPatchName, customWebpagesName} {
		if err := gs.UploadFile(patchName, os.TempDir(), remoteOutputDir); err != nil {
			return fmt.Errorf("Could not upload %s to %s: %s", patchName, remoteOutputDir, err)
		}
	}
	chromiumPatchLink = util.GCS_HTTP_LINK + filepath.Join(util.GCSBucketName, remoteOutputDir, chromiumPatchName)
	skiaPatchLink = util.GCS_HTTP_LINK + filepath.Join(util.GCSBucketName, remoteOutputDir, skiaPatchName)
	v8PatchLink = util.GCS_HTTP_LINK + filepath.Join(util.GCSBucketName, remoteOutputDir, v8PatchName)
	catapultPatchLink = util.GCS_HTTP_LINK + filepath.Join(util.GCSBucketName, remoteOutputDir, catapultPatchName)
	customWebpagesLink = util.GCS_HTTP_LINK + filepath.Join(util.GCSBucketName, remoteOutputDir, customWebpagesName)

	// Find which chromium hash the builds should use.
	if *chromiumHash == "" {
		*chromiumHash, err = util.GetChromiumHash(ctx)
		if err != nil {
			return fmt.Errorf("Could not find the latest chromium hash: %s", err)
		}
	}

	// Trigger both the build repo and isolate telemetry tasks in parallel.
	group := skutil.NewNamedErrGroup()
	var chromiumBuild string
	group.Go("build chromium", func() error {
		chromiumBuilds, err := util.TriggerBuildRepoSwarmingTask(ctx, "build_chromium", *runID, "chromium", *targetPlatform, "", []string{*chromiumHash}, []string{filepath.Join(remoteOutputDir, chromiumPatchName), filepath.Join(remoteOutputDir, skiaPatchName), filepath.Join(remoteOutputDir, v8PatchName)}, []string{}, true /*singleBuild*/, *master_common.Local, 3*time.Hour, 1*time.Hour)
		if err != nil {
			return sklog.FmtErrorf("Error encountered when swarming build repo task: %s", err)
		}
		if len(chromiumBuilds) != 1 {
			return sklog.FmtErrorf("Expected 1 build but instead got %d: %v", len(chromiumBuilds), chromiumBuilds)
		}
		chromiumBuild = chromiumBuilds[0]
		return nil
	})

	// Isolate telemetry.
	isolateDeps := []string{}
	group.Go("isolate telemetry", func() error {
		telemetryIsolatePatches := []string{filepath.Join(remoteOutputDir, chromiumPatchName), filepath.Join(remoteOutputDir, catapultPatchName), filepath.Join(remoteOutputDir, v8PatchName)}
		telemetryHash, err := util.TriggerIsolateTelemetrySwarmingTask(ctx, "isolate_telemetry", *runID, *chromiumHash, "", *targetPlatform, telemetryIsolatePatches, 1*time.Hour, 1*time.Hour, *master_common.Local)
		if err != nil {
			return sklog.FmtErrorf("Error encountered when swarming isolate telemetry task: %s", err)
		}
		if telemetryHash == "" {
			return sklog.FmtErrorf("Found empty telemetry hash!")
		}
		isolateDeps = append(isolateDeps, telemetryHash)
		return nil
	})

	// Wait for chromium build task and isolate telemetry task to complete.
	if err := group.Wait(); err != nil {
		return err
	}

	// Clean up the chromium builds from Google storage after the run completes.
	defer gs.DeleteRemoteDirLogErr(filepath.Join(util.CHROMIUM_BUILDS_DIR_NAME, chromiumBuild))

	// Archive, trigger and collect swarming tasks.
	isolateExtraArgs := map[string]string{
		"CHROMIUM_BUILD":     chromiumBuild,
		"RUN_ID":             *runID,
		"BENCHMARK":          *benchmarkName,
		"BENCHMARK_ARGS":     *benchmarkExtraArgs,
		"BROWSER_EXTRA_ARGS": *browserExtraArgs,
		"RUN_IN_PARALLEL":    strconv.FormatBool(*runInParallel),
		"TARGET_PLATFORM":    *targetPlatform,
		"MATCH_STDOUT_TXT":   *matchStdoutText,
	}

	customWebPagesFilePath := filepath.Join(os.TempDir(), customWebpagesName)
	numPages, err := util.GetNumPages(*pagesetType, customWebPagesFilePath)
	if err != nil {
		return fmt.Errorf("Error encountered when calculating number of pages: %s", err)
	}
	// Calculate the max pages to run per bot.
	maxPagesPerBot := util.GetMaxPagesPerBotValue(*benchmarkExtraArgs, MAX_PAGES_PER_SWARMING_BOT)
	numSlaves, err := util.TriggerSwarmingTask(ctx, *pagesetType, "chromium_analysis", util.CHROMIUM_ANALYSIS_ISOLATE, *runID, "", *targetPlatform, 12*time.Hour, 3*time.Hour, *taskPriority, maxPagesPerBot, numPages, isolateExtraArgs, *runOnGCE, *master_common.Local, util.GetRepeatValue(*benchmarkExtraArgs, 1), isolateDeps)
	if err != nil {
		return fmt.Errorf("Error encountered when swarming tasks: %s", err)
	}

	// Merge all CSV files and upload.
	pathToPyFiles := util.GetPathToPyFiles(*master_common.Local)
	outputCSVLocalPath, noOutputSlaves, err := util.MergeUploadCSVFiles(ctx, *runID, pathToPyFiles, gs, numPages, maxPagesPerBot, true /* handleStrings */, util.GetRepeatValue(*benchmarkExtraArgs, 1))
	if err != nil {
		return fmt.Errorf("Unable to merge and upload CSV files for %s: %s", *runID, err)
	}
	// Cleanup created dir after the run completes.
	defer skutil.RemoveAll(filepath.Join(util.StorageDir, util.BenchmarkRunsDir, *runID))

	// If the number of noOutputSlaves is the same as the total number of triggered slaves then consider the run failed.
	if len(noOutputSlaves) == numSlaves {
		return fmt.Errorf("All %d slaves produced no output", numSlaves)
	}

	// Construct the output link.
	outputLink = util.GCS_HTTP_LINK + filepath.Join(util.GCSBucketName, util.BenchmarkRunsDir, *runID, "consolidated_outputs", *runID+".output")

	// Display the no output slaves.
	for _, noOutputSlave := range noOutputSlaves {
		directLink := fmt.Sprintf(util.SWARMING_RUN_ID_TASK_LINK_PREFIX_TEMPLATE, *runID, "chromium_analysis_"+noOutputSlave)
		fmt.Printf("Missing output from %s\n", directLink)
	}

	if *groupName != "" {
		if err := util.AddCTRunDataToPerf(ctx, *groupName, *runID, outputCSVLocalPath, gs); err != nil {
			return fmt.Errorf("Could not add CT run data to ct-perf.skia.org: %s", err)
		}
	}

	taskCompletedSuccessfully = true
	return nil
}

func main() {
	retCode := 0
	if err := runChromiumAnalysisOnWorkers(); err != nil {
		sklog.Errorf("Error while running chromium analysis on workers: %s", err)
		retCode = 255
	}
	os.Exit(retCode)
}
