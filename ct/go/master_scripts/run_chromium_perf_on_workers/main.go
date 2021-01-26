// run_chromium_perf_on_workers is an application that runs the specified telemetry
// benchmark on all CT workers and uploads the results to Google Storage. The
// requester is emailed when the task is done.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"go.skia.org/infra/ct/go/master_scripts/master_common"
	"go.skia.org/infra/ct/go/util"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/gitauth"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	skutil "go.skia.org/infra/go/util"
)

const (
	MAX_PAGES_PER_SWARMING_BOT         = 100
	MAX_PAGES_PER_ANDROID_SWARMING_BOT = 20
)

var (
	runRequester              = flag.String("run_requester", "", "The email address of the user that triggered the run.")
	description               = flag.String("description", "", "The description of the run as entered by the requester.")
	pagesetType               = flag.String("pageset_type", "", "The type of pagesets to use. Eg: 10k, Mobile10k, All.")
	benchmarkName             = flag.String("benchmark_name", "", "The telemetry benchmark to run on the workers.")
	benchmarkExtraArgs        = flag.String("benchmark_extra_args", "", "The extra arguments that are passed to the specified benchmark.")
	browserExtraArgsNoPatch   = flag.String("browser_extra_args_nopatch", "", "The extra arguments that are passed to the browser while running the benchmark for the nopatch case.")
	browserExtraArgsWithPatch = flag.String("browser_extra_args_withpatch", "", "The extra arguments that are passed to the browser while running the benchmark for the withpatch case.")
	repeatBenchmark           = flag.Int("repeat_benchmark", 1, "The number of times the benchmark should be repeated.")
	runInParallel             = flag.Bool("run_in_parallel", false, "Run the benchmark by bringing up multiple chrome instances in parallel.")
	targetPlatform            = flag.String("target_platform", util.PLATFORM_ANDROID, "The platform the benchmark will run on (Android / Linux).")
	runOnGCE                  = flag.Bool("run_on_gce", true, "Run on Linux GCE instances.")
	runID                     = flag.String("run_id", "", "The unique run id (typically requester + timestamp).")
	varianceThreshold         = flag.Float64("variance_threshold", 0.0, "The variance threshold to use when comparing the resultant CSV files.")
	discardOutliers           = flag.Float64("discard_outliers", 0.0, "The percentage of outliers to discard when comparing the result CSV files.")
	chromiumHash              = flag.String("chromium_hash", "", "The Chromium full hash the checkout should be synced to before applying patches.")
	taskPriority              = flag.Int("task_priority", util.TASKS_PRIORITY_MEDIUM, "The priority swarming tasks should run at.")
	groupName                 = flag.String("group_name", "", "The group name of this run. It will be used as the key when uploading data to ct-perf.skia.org.")
	valueColumnName           = flag.String("value_column_name", "", "Which column's entries to use as field values when combining CSVs.")

	chromiumPatchGSPath          = flag.String("chromium_patch_gs_path", "", "The location of the Chromium patch in Google storage.")
	skiaPatchGSPath              = flag.String("skia_patch_gs_path", "", "The location of the Skia patch in Google storage.")
	v8PatchGSPath                = flag.String("v8_patch_gs_path", "", "The location of the V8 patch in Google storage.")
	catapultPatchGSPath          = flag.String("catapult_patch_gs_path", "", "The location of the Catapult patch in Google storage.")
	chromiumBaseBuildPatchGSPath = flag.String("chromium_base_build_patch_gs_path", "", "The location of the Chromium base build patch in Google storage.")
	customWebpagesCsvGSPath      = flag.String("custom_webpages_csv_gs_path", "", "The location of the custom webpages CSV in Google storage.")
)

func runChromiumPerfOnWorkers() error {
	swarmingClient, err := master_common.Init("run_chromium_perf")
	if err != nil {
		return fmt.Errorf("Could not init: %s", err)
	}

	ctx := context.Background()

	// Cleanup dirs after run completes.
	defer skutil.RemoveAll(filepath.Join(util.StorageDir, util.ChromiumPerfRunsDir, *runID))
	// Finish with glog flush and how long the task took.
	defer util.TimeTrack(time.Now(), "Running chromium perf task on workers")
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
	if *description == "" {
		return errors.New("Must specify --description")
	}

	// Use defaults.
	if *valueColumnName == "" {
		*valueColumnName = util.DEFAULT_VALUE_COLUMN_NAME
	}

	// Instantiate GcsUtil object.
	gs, err := util.NewGcsUtil(nil)
	if err != nil {
		return fmt.Errorf("Could not instantiate gsutil object: %s", err)
	}
	remoteOutputDir := util.GetPerfRemoteDir(*runID)

	// Find git exec.
	gitExec, err := git.Executable(ctx)
	if err != nil {
		return skerr.Wrap(err)
	}

	// TODO(rmistry): Fix the below.
	//
	// This is silly, we do the following here:
	// * Downloads patches from the location specified by CTFE.
	// * Locally "fixes" them (adds an extra newline at the end).
	// * Uses the locally downloaded the files to check if all patches are
	//   empty to determine if we should use a single build.
	// * Re-uploads them in a location expected by the builder tasks.
	//
	// What we should be doing instead is using the CTFE locations and passing them
	// around to all the worker scripts that need them by being explicit. Eg:
	// "skiaPatchGSPath". rmistry@ started doing this but it made the effort of
	// making CT interruptible more difficult. Should tackle this as a separate
	// effort here and in all other similar master scripts.
	//
	for fileSuffix, patchGSPath := range map[string]string{
		".chromium.patch":            *chromiumPatchGSPath,
		".skia.patch":                *skiaPatchGSPath,
		".v8.patch":                  *v8PatchGSPath,
		".catapult.patch":            *catapultPatchGSPath,
		".chromium_base_build.patch": *chromiumBaseBuildPatchGSPath,
		".custom_webpages.csv":       *customWebpagesCsvGSPath,
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
	skiaPatchName := *runID + ".skia.patch"
	chromiumPatchName := *runID + ".chromium.patch"
	v8PatchName := *runID + ".v8.patch"
	catapultPatchName := *runID + ".catapult.patch"
	chromiumPatchNameBaseBuild := *runID + ".chromium_base_build.patch"
	customWebpagesName := *runID + ".custom_webpages.csv"
	for _, patchName := range []string{skiaPatchName, chromiumPatchName, v8PatchName, catapultPatchName, chromiumPatchNameBaseBuild, customWebpagesName} {
		if err := gs.UploadFile(patchName, os.TempDir(), remoteOutputDir); err != nil {
			return fmt.Errorf("Could not upload %s to %s: %s", patchName, remoteOutputDir, err)
		}
	}
	skiaPatchLink := util.GCS_HTTP_LINK + filepath.Join(util.GCSBucketName, remoteOutputDir, skiaPatchName)
	chromiumPatchLink := util.GCS_HTTP_LINK + filepath.Join(util.GCSBucketName, remoteOutputDir, chromiumPatchName)

	// Check if the patches have any content to decide if we need one or two chromium builds.
	localPatches := []string{filepath.Join(os.TempDir(), chromiumPatchName), filepath.Join(os.TempDir(), skiaPatchName), filepath.Join(os.TempDir(), v8PatchName), filepath.Join(os.TempDir(), chromiumPatchNameBaseBuild)}
	remotePatches := []string{filepath.Join(remoteOutputDir, chromiumPatchName), filepath.Join(remoteOutputDir, skiaPatchName), filepath.Join(remoteOutputDir, v8PatchName), filepath.Join(remoteOutputDir, chromiumPatchNameBaseBuild)}

	// Find which chromium hash the builds should use.
	if *chromiumHash == "" {
		*chromiumHash, err = util.GetChromiumHash(ctx, gitExec)
		if err != nil {
			return fmt.Errorf("Could not find the latest chromium hash: %s", err)
		}
	}

	// Trigger both the build repo and isolate telemetry tasks in parallel.
	group := skutil.NewNamedErrGroup()
	var chromiumBuildNoPatch, chromiumBuildWithPatch string
	group.Go("build chromium", func() error {
		if util.PatchesAreEmpty(localPatches) {
			// Create only one chromium build.
			chromiumBuilds, err := util.TriggerBuildRepoSwarmingTask(
				ctx, "build_chromium", *runID, "chromium", *targetPlatform, "", []string{*chromiumHash}, remotePatches, []string{},
				true, *master_common.Local, 3*time.Hour, 1*time.Hour, swarmingClient)
			if err != nil {
				return skerr.Fmt("Error encountered when swarming build repo task: %s", err)
			}
			if len(chromiumBuilds) != 1 {
				return skerr.Fmt("Expected 1 build but instead got %d: %v.", len(chromiumBuilds), chromiumBuilds)
			}
			chromiumBuildNoPatch = chromiumBuilds[0]
			chromiumBuildWithPatch = chromiumBuilds[0]

		} else {
			// Create the two required chromium builds (with patch and without the patch).
			chromiumBuilds, err := util.TriggerBuildRepoSwarmingTask(
				ctx, "build_chromium", *runID, "chromium", *targetPlatform, "", []string{*chromiumHash}, remotePatches, []string{},
				false, *master_common.Local, 3*time.Hour, 1*time.Hour, swarmingClient)
			if err != nil {
				return skerr.Fmt("Error encountered when swarming build repo task: %s", err)
			}
			if len(chromiumBuilds) != 2 {
				return skerr.Fmt("Expected 2 builds but instead got %d: %v.", len(chromiumBuilds), chromiumBuilds)
			}
			chromiumBuildNoPatch = chromiumBuilds[0]
			chromiumBuildWithPatch = chromiumBuilds[1]
		}
		return nil
	})

	// Isolate telemetry.
	isolateDeps := []string{}
	group.Go("isolate telemetry", func() error {
		telemetryIsolatePatches := []string{filepath.Join(remoteOutputDir, chromiumPatchName), filepath.Join(remoteOutputDir, catapultPatchName), filepath.Join(remoteOutputDir, v8PatchName)}
		telemetryHash, err := util.TriggerIsolateTelemetrySwarmingTask(ctx, "isolate_telemetry", *runID, *chromiumHash, "", *targetPlatform, telemetryIsolatePatches, 1*time.Hour, 1*time.Hour, *master_common.Local, swarmingClient)
		if err != nil {
			return fmt.Errorf("Error encountered when swarming isolate telemetry task: %s", err)
		}
		if telemetryHash == "" {
			return fmt.Errorf("Found empty telemetry hash!")
		}
		isolateDeps = append(isolateDeps, telemetryHash)
		return nil
	})

	// Wait for chromium build task and isolate telemetry task to complete.
	if err := group.Wait(); err != nil {
		return err
	}

	baseCmd := []string{
		"luci-auth",
		"context",
		"--",
		"bin/run_chromium_perf",
		"-logtostderr",
		"--chromium_build_nopatch=" + chromiumBuildNoPatch,
		"--chromium_build_withpatch=" + chromiumBuildWithPatch,
		"--run_id=" + *runID,
		"--benchmark_name=" + *benchmarkName,
		"--benchmark_extra_args=" + *benchmarkExtraArgs,
		"--browser_extra_args_nopatch=" + *browserExtraArgsNoPatch,
		"--browser_extra_args_withpatch=" + *browserExtraArgsWithPatch,
		"--repeat_benchmark=" + strconv.Itoa(*repeatBenchmark),
		"--run_in_parallel=" + strconv.FormatBool(*runInParallel),
		"--target_platform=" + *targetPlatform,
		"--value_column_name=" + *valueColumnName,
	}
	customWebPagesFilePath := filepath.Join(os.TempDir(), customWebpagesName)
	numPages, err := util.GetNumPages(*pagesetType, customWebPagesFilePath)
	if err != nil {
		return fmt.Errorf("Error encountered when calculating number of pages: %s", err)
	}
	// Determine hard timeout according to the number of times benchmark should be repeated.
	// Cap it off at the max allowable hours.
	var hardTimeout = time.Duration(skutil.MinInt(12**repeatBenchmark, util.MAX_SWARMING_HARD_TIMEOUT_HOURS)) * time.Hour
	// Calculate the max pages to run per bot.
	defaultMaxPagesPerSwarmingBot := MAX_PAGES_PER_SWARMING_BOT
	if *targetPlatform == util.PLATFORM_ANDROID {
		defaultMaxPagesPerSwarmingBot = MAX_PAGES_PER_ANDROID_SWARMING_BOT
	}
	maxPagesPerBot := util.GetMaxPagesPerBotValue(*benchmarkExtraArgs, defaultMaxPagesPerSwarmingBot)
	numWorkers, err := util.TriggerSwarmingTask(ctx, *pagesetType, "chromium_perf", util.CHROMIUM_PERF_ISOLATE, *runID, "", *targetPlatform, hardTimeout, 1*time.Hour, *taskPriority, maxPagesPerBot, numPages, *runOnGCE, *master_common.Local, util.GetRepeatValue(*benchmarkExtraArgs, *repeatBenchmark), baseCmd, isolateDeps, swarmingClient)
	if err != nil {
		return fmt.Errorf("Error encountered when swarming tasks: %s", err)
	}

	// If "--output-format=csv" is specified then merge all CSV files and upload.
	runIDNoPatch := fmt.Sprintf("%s-nopatch", *runID)
	runIDWithPatch := fmt.Sprintf("%s-withpatch", *runID)
	pathToPyFiles, err := util.GetPathToPyFiles(*master_common.Local)
	if err != nil {
		return fmt.Errorf("Could not get path to py files: %s", err)
	}
	var noOutputWorkers []string

	// Nopatch CSV file processing.
	noPatchCSVLocalPath, noOutputWorkers, err := util.MergeUploadCSVFiles(ctx, runIDNoPatch, pathToPyFiles, gs, numPages, maxPagesPerBot, true /* handleStrings */, util.GetRepeatValue(*benchmarkExtraArgs, *repeatBenchmark))
	if err != nil {
		return fmt.Errorf("Unable to merge and upload CSV files for %s: %s", runIDNoPatch, err)
	}
	// Cleanup created dir after the run completes.
	defer skutil.RemoveAll(filepath.Join(util.StorageDir, util.BenchmarkRunsDir, runIDNoPatch))
	// If the number of noOutputWorkers is the same as the total number of triggered workers then consider the run failed.
	if len(noOutputWorkers) == numWorkers {
		return fmt.Errorf("All %d workers produced no output for nopatch run", numWorkers)
	}

	// Withpatch CSV file processing.
	withPatchCSVLocalPath, noOutputWorkers, err := util.MergeUploadCSVFiles(ctx, runIDWithPatch, pathToPyFiles, gs, numPages, maxPagesPerBot, true /* handleStrings */, util.GetRepeatValue(*benchmarkExtraArgs, *repeatBenchmark))
	if err != nil {
		return fmt.Errorf("Unable to merge and upload CSV files for %s: %s", runIDWithPatch, err)
	}
	// Cleanup created dir after the run completes.
	defer skutil.RemoveAll(filepath.Join(util.StorageDir, util.BenchmarkRunsDir, runIDWithPatch))
	// If the number of noOutputWorkers is the same as the total number of triggered workers then consider the run failed.
	if len(noOutputWorkers) == numWorkers {
		return fmt.Errorf("All %d workers produced no output for withpatch run", numWorkers)
	}

	totalArchivedWebpages, err := util.GetArchivesNum(gs, *benchmarkExtraArgs, *pagesetType)
	if err != nil {
		return fmt.Errorf("Error when calculating number of archives: %s", err)
	}

	// Compare the resultant CSV files using csv_comparer.py
	_, skiaHash := util.GetHashesFromBuild(chromiumBuildNoPatch)
	htmlOutputDir := filepath.Join(util.StorageDir, util.ChromiumPerfRunsDir, *runID, "html")
	util.MkdirAll(htmlOutputDir, 0700)
	htmlRemoteDir := util.GetPerfRemoteHTMLDir(*runID)
	htmlOutputLinkBase := util.GetPerfOutputLinkBase(*runID)
	noPatchOutputLink := util.GetPerfNoPatchOutputLink(*runID)
	withPatchOutputLink := util.GetPerfWithPatchOutputLink(*runID)
	// Construct path to the csv_comparer python script.
	pathToCsvComparer := filepath.Join(pathToPyFiles, "csv_comparer.py")
	args := []string{
		pathToCsvComparer,
		"--csv_file1=" + noPatchCSVLocalPath,
		"--csv_file2=" + withPatchCSVLocalPath,
		"--output_html=" + htmlOutputDir,
		"--variance_threshold=" + strconv.FormatFloat(*varianceThreshold, 'f', 2, 64),
		"--discard_outliers=" + strconv.FormatFloat(*discardOutliers, 'f', 2, 64),
		"--absolute_url=" + htmlOutputLinkBase,
		"--requester_email=" + *runRequester,
		"--skia_patch_link=" + skiaPatchLink,
		"--chromium_patch_link=" + chromiumPatchLink,
		"--description=" + *description,
		"--raw_csv_nopatch=" + noPatchOutputLink,
		"--raw_csv_withpatch=" + withPatchOutputLink,
		"--num_repeated=" + strconv.Itoa(*repeatBenchmark),
		"--target_platform=" + *targetPlatform,
		"--browser_args_nopatch=" + *browserExtraArgsNoPatch,
		"--browser_args_withpatch=" + *browserExtraArgsWithPatch,
		"--pageset_type=" + *pagesetType,
		"--chromium_hash=" + *chromiumHash,
		"--skia_hash=" + skiaHash,
		"--missing_output_workers=" + strings.Join(noOutputWorkers, " "),
		"--logs_link_prefix=" + fmt.Sprintf(util.SWARMING_RUN_ID_TASK_LINK_PREFIX_TEMPLATE, *runID, "chromium_perf_"),
		"--total_archives=" + strconv.Itoa(totalArchivedWebpages),
	}
	err = util.ExecuteCmd(ctx, util.BINARY_PYTHON, args, []string{}, util.CSV_COMPARER_TIMEOUT, nil, nil)
	if err != nil {
		return fmt.Errorf("Error running csv_comparer.py: %s", err)
	}

	// Copy the HTML files to Google Storage.
	if err := gs.UploadDir(htmlOutputDir, htmlRemoteDir, true); err != nil {
		return fmt.Errorf("Could not upload %s to %s: %s", htmlOutputDir, htmlRemoteDir, err)
	}

	if *groupName != "" {
		// Start the gitauth package because we will need to commit to CT Perf's synthetic repo.
		ts, err := auth.NewDefaultTokenSource(*master_common.Local, auth.SCOPE_USERINFO_EMAIL, auth.SCOPE_GERRIT)
		if err != nil {
			return err
		}
		if _, err := gitauth.New(ts, filepath.Join(os.TempDir(), "gitcookies"), true, util.CT_SERVICE_ACCOUNT); err != nil {
			return fmt.Errorf("Failed to create git cookie updater: %s", err)
		}

		if err := util.AddCTRunDataToPerf(ctx, *groupName, *runID, withPatchCSVLocalPath, gitExec, gs); err != nil {
			return fmt.Errorf("Could not add CT run data to ct-perf.skia.org: %s", err)
		}
	}

	return nil
}

func main() {
	retCode := 0
	if err := runChromiumPerfOnWorkers(); err != nil {
		sklog.Errorf("Error while running chromium perf on workers: %s", err)
		retCode = 255
	}
	os.Exit(retCode)
}
