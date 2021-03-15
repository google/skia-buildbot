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
	maxPagesPerSwarmingBot        = 100
	maxPagesPerAndroidSwarmingBot = 20
)

var (
	pagesetType          = flag.String("pageset_type", "", "The type of pagesets to use. Eg: 10k, Mobile10k, All.")
	benchmarkName        = flag.String("benchmark_name", "", "The telemetry benchmark to run on the workers.")
	benchmarkExtraArgs   = flag.String("benchmark_extra_args", "", "The extra arguments that are passed to the specified benchmark.")
	browserExtraArgs     = flag.String("browser_extra_args", "", "The extra arguments that are passed to the browser while running the benchmark.")
	runID                = flag.String("run_id", "", "The unique run id (typically requester + timestamp).")
	targetPlatform       = flag.String("target_platform", util.PLATFORM_LINUX, "The platform the benchmark will run on (Android / Linux).")
	runOnGCE             = flag.Bool("run_on_gce", true, "Run on Linux GCE instances. Used only if Linux is used for the target_platform.")
	runInParallel        = flag.Bool("run_in_parallel", true, "Run the benchmark by bringing up multiple chrome instances in parallel.")
	matchStdoutText      = flag.String("match_stdout_txt", "", "Looks for the specified string in the stdout of web page runs. The count of the text's occurence and the lines containing it are added to the CSV of the web page.")
	chromiumHash         = flag.String("chromium_hash", "", "The Chromium full hash the checkout should be synced to before applying patches.")
	apkGsPath            = flag.String("apk_gs_path", "", "GS path to a custom APK to use instead of building one from scratch. Eg: gs://chrome-unsigned/android-B0urB0N/79.0.3922.0/arm_64/ChromeModern.apk")
	chromeBuildGsPath    = flag.String("chrome_build_gs_path", "", "GS path to a custom chrome build to use instead of building one from scratch. Eg: gs://chromium-browser-snapshots/Linux_x64/805044/chrome-linux.zip")
	telemetryIsolateHash = flag.String("telemetry_isolate_hash", "", "User specified telemetry isolate hash to download and use from isolate server. If specified the \"Isolate Telemetry\" task will be skipped.")
	taskPriority         = flag.Int("task_priority", util.TASKS_PRIORITY_MEDIUM, "The priority swarming tasks should run at.")
	groupName            = flag.String("group_name", "", "The group name of this run. It will be used as the key when uploading data to ct-perf.skia.org.")
	valueColumnName      = flag.String("value_column_name", "", "Which column's entries to use as field values when combining CSVs.")

	chromiumPatchGSPath     = flag.String("chromium_patch_gs_path", "", "The location of the Chromium patch in Google storage.")
	skiaPatchGSPath         = flag.String("skia_patch_gs_path", "", "The location of the Skia patch in Google storage.")
	v8PatchGSPath           = flag.String("v8_patch_gs_path", "", "The location of the V8 patch in Google storage.")
	catapultPatchGSPath     = flag.String("catapult_patch_gs_path", "", "The location of the Catapult patch in Google storage.")
	customWebpagesCsvGSPath = flag.String("custom_webpages_csv_gs_path", "", "The location of the custom webpages CSV in Google storage.")
)

func runChromiumAnalysisOnWorkers() error {
	swarmingClient, casClient, err := master_common.Init("run_chromium_analysis")
	if err != nil {
		return fmt.Errorf("Could not init: %s", err)
	}

	ctx := context.Background()

	// Instantiate GcsUtil object.
	gs, err := util.NewGcsUtil(nil)
	if err != nil {
		return fmt.Errorf("Could not instantiate gsutil object: %s", err)
	}

	// Find git exec.
	gitExec, err := git.Executable(ctx)
	if err != nil {
		return skerr.Wrap(err)
	}

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
	if *apkGsPath != "" && !strings.HasPrefix(*apkGsPath, "gs://") {
		return errors.New("apkGsPath must start with gs://")
	}
	if *chromeBuildGsPath != "" && !strings.HasPrefix(*chromeBuildGsPath, "gs://") {
		return errors.New("chromeBuildGsPath must start with gs://")
	}

	// Use defaults.
	if *valueColumnName == "" {
		*valueColumnName = util.DEFAULT_VALUE_COLUMN_NAME
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

	// Find which chromium hash the builds should use.
	if *chromiumHash == "" {
		*chromiumHash, err = util.GetChromiumHash(ctx, gitExec)
		if err != nil {
			return fmt.Errorf("Could not find the latest chromium hash: %s", err)
		}
	}

	// Trigger both the build repo and isolate telemetry tasks in parallel.
	group := skutil.NewNamedErrGroup()
	var chromiumBuild string
	if *apkGsPath != "" || *chromeBuildGsPath != "" {
		// Do not trigger chromium build if a custom APK or chrome build is specified.
		chromiumBuild = ""
	} else {
		group.Go("build chromium", func() error {
			chromiumBuilds, err := util.TriggerBuildRepoSwarmingTask(ctx, "build_chromium", *runID, "chromium", *targetPlatform, "", []string{*chromiumHash}, []string{filepath.Join(remoteOutputDir, chromiumPatchName), filepath.Join(remoteOutputDir, skiaPatchName), filepath.Join(remoteOutputDir, v8PatchName)}, []string{}, true /*singleBuild*/, *master_common.Local, 3*time.Hour, 1*time.Hour, swarmingClient, casClient)
			if err != nil {
				return skerr.Fmt("Error encountered when swarming build repo task: %s", err)
			}
			if len(chromiumBuilds) != 1 {
				return skerr.Fmt("Expected 1 build but instead got %d: %v", len(chromiumBuilds), chromiumBuilds)
			}
			chromiumBuild = chromiumBuilds[0]
			return nil
		})
	}

	// Isolate telemetry.
	casDeps := []string{}
	if *telemetryIsolateHash != "" {
		casDeps = append(casDeps, *telemetryIsolateHash)
	} else {
		group.Go("isolate telemetry", func() error {
			telemetryIsolatePatches := []string{filepath.Join(remoteOutputDir, chromiumPatchName), filepath.Join(remoteOutputDir, catapultPatchName), filepath.Join(remoteOutputDir, v8PatchName)}
			telemetryHash, err := util.TriggerIsolateTelemetrySwarmingTask(ctx, "isolate_telemetry", *runID, *chromiumHash, "", *targetPlatform, telemetryIsolatePatches, 1*time.Hour, 1*time.Hour, *master_common.Local, swarmingClient, casClient)
			if err != nil {
				return skerr.Fmt("Error encountered when swarming isolate telemetry task: %s", err)
			}
			if telemetryHash == "" {
				return skerr.Fmt("Found empty telemetry hash!")
			}
			casDeps = append(casDeps, telemetryHash)
			return nil
		})
	}

	// Wait for chromium build task and isolate telemetry task to complete.
	if err := group.Wait(); err != nil {
		return err
	}

	if chromiumBuild != "" {
		// If a chromium build was created then delete it from Google storage after the run completes.
		defer gs.DeleteRemoteDirLogErr(filepath.Join(util.CHROMIUM_BUILDS_DIR_NAME, chromiumBuild))
	}

	// Archive, trigger and collect swarming tasks.
	baseCmd := []string{
		"luci-auth",
		"context",
		"--",
		"bin/run_chromium_analysis",
		"-logtostderr",
		"--chromium_build=" + chromiumBuild,
		"--run_id=" + *runID,
		"--apk_gs_path=" + *apkGsPath,
		"--chrome_build_gs_path=" + *chromeBuildGsPath,
		"--benchmark_name=" + *benchmarkName,
		"--benchmark_extra_args=" + *benchmarkExtraArgs,
		"--browser_extra_args=" + *browserExtraArgs,
		"--run_in_parallel=" + strconv.FormatBool(*runInParallel),
		"--target_platform=" + *targetPlatform,
		"--match_stdout_txt=" + *matchStdoutText,
		"--value_column_name=" + *valueColumnName,
	}

	customWebPagesFilePath := filepath.Join(os.TempDir(), customWebpagesName)
	numPages, err := util.GetNumPages(*pagesetType, customWebPagesFilePath)
	if err != nil {
		return fmt.Errorf("Error encountered when calculating number of pages: %s", err)
	}
	// Calculate the max pages to run per bot.
	defaultMaxPagesPerSwarmingBot := maxPagesPerSwarmingBot
	if *targetPlatform == util.PLATFORM_ANDROID {
		defaultMaxPagesPerSwarmingBot = maxPagesPerAndroidSwarmingBot
	}
	maxPagesPerBot := util.GetMaxPagesPerBotValue(*benchmarkExtraArgs, defaultMaxPagesPerSwarmingBot)
	casSpec := util.CasChromiumAnalysisLinux()
	if *targetPlatform == util.PLATFORM_WINDOWS {
		casSpec = util.CasChromiumAnalysisWin()
	}
	casSpec.IncludeDigests = append(casSpec.IncludeDigests, casDeps...)
	numWorkers, err := util.TriggerSwarmingTask(ctx, *pagesetType, "chromium_analysis", *runID, *targetPlatform, casSpec, 12*time.Hour, 3*time.Hour, *taskPriority, maxPagesPerBot, numPages, *runOnGCE, *master_common.Local, util.GetRepeatValue(*benchmarkExtraArgs, 1), baseCmd, swarmingClient, casClient)
	if err != nil {
		return fmt.Errorf("Error encountered when swarming tasks: %s", err)
	}

	// Merge all CSV files and upload.
	pathToPyFiles, err := util.GetPathToPyFiles(*master_common.Local)
	if err != nil {
		return fmt.Errorf("Could not get path to py files: %s", err)
	}
	outputCSVLocalPath, noOutputWorkers, err := util.MergeUploadCSVFiles(ctx, *runID, pathToPyFiles, gs, numPages, maxPagesPerBot, true /* handleStrings */, util.GetRepeatValue(*benchmarkExtraArgs, 1))
	if err != nil {
		return fmt.Errorf("Unable to merge and upload CSV files for %s: %s", *runID, err)
	}
	// Cleanup created dir after the run completes.
	defer skutil.RemoveAll(filepath.Join(util.StorageDir, util.BenchmarkRunsDir, *runID))

	// If the number of noOutputWorkers is the same as the total number of triggered workers then consider the run failed.
	if len(noOutputWorkers) == numWorkers {
		return fmt.Errorf("All %d workers produced no output", numWorkers)
	}

	// Display the no output workers.
	for _, noOutputWorker := range noOutputWorkers {
		directLink := fmt.Sprintf(util.SWARMING_RUN_ID_TASK_LINK_PREFIX_TEMPLATE, *runID, "chromium_analysis_"+noOutputWorker)
		fmt.Printf("Missing output from %s\n", directLink)
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

		if err := util.AddCTRunDataToPerf(ctx, *groupName, *runID, outputCSVLocalPath, gitExec, gs); err != nil {
			return fmt.Errorf("Could not add CT run data to ct-perf.skia.org: %s", err)
		}
	}

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
