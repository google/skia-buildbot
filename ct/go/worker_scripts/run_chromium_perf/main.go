// run_chromium_perf is an application that runs the specified benchmark over CT's
// webpage archives.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"sync"
	"time"

	"github.com/skia-dev/glog"

	"strings"

	"go.skia.org/infra/ct/go/adb"
	"go.skia.org/infra/ct/go/util"
	"go.skia.org/infra/ct/go/worker_scripts/worker_common"
	"go.skia.org/infra/go/common"
	skutil "go.skia.org/infra/go/util"
)

const (
	// The number of goroutines that will run in parallel to run benchmarks.
	WORKER_POOL_SIZE = 10
)

var (
	startRange                = flag.Int("start_range", 1, "The number this worker will run benchmarks from.")
	num                       = flag.Int("num", 100, "The total number of benchmarks to run starting from the start_range.")
	pagesetType               = flag.String("pageset_type", util.PAGESET_TYPE_MOBILE_10k, "The type of pagesets to create from the Alexa CSV list. Eg: 10k, Mobile10k, All.")
	chromiumBuildNoPatch      = flag.String("chromium_build_nopatch", "", "The chromium build to use for the nopatch run.")
	chromiumBuildWithPatch    = flag.String("chromium_build_withpatch", "", "The chromium build to use for the withpatch run.")
	runID                     = flag.String("run_id", "", "The unique run id (typically requester + timestamp).")
	benchmarkName             = flag.String("benchmark_name", "", "The telemetry benchmark to run on this worker.")
	benchmarkExtraArgs        = flag.String("benchmark_extra_args", "", "The extra arguments that are passed to the specified benchmark.")
	browserExtraArgsNoPatch   = flag.String("browser_extra_args_nopatch", "", "The extra arguments that are passed to the browser while running the benchmark during the nopatch run.")
	browserExtraArgsWithPatch = flag.String("browser_extra_args_withpatch", "", "The extra arguments that are passed to the browser while running the benchmark during the withpatch run.")
	repeatBenchmark           = flag.Int("repeat_benchmark", 3, "The number of times the benchmark should be repeated. For skpicture_printer benchmark this value is always 1.")
	runInParallel             = flag.Bool("run_in_parallel", false, "Run the benchmark by bringing up multiple chrome instances in parallel.")
	targetPlatform            = flag.String("target_platform", util.PLATFORM_ANDROID, "The platform the benchmark will run on (Android / Linux).")
	chromeCleanerTimer        = flag.Duration("cleaner_timer", 15*time.Minute, "How often all chrome processes will be killed on this slave.")
)

func main() {
	defer common.LogPanic()
	worker_common.Init()
	defer util.TimeTrack(time.Now(), "Running Chromium Perf")
	defer glog.Flush()

	// Validate required arguments.
	if *chromiumBuildNoPatch == "" {
		glog.Error("Must specify --chromium_build_nopatch")
		return
	}
	if *chromiumBuildWithPatch == "" {
		glog.Error("Must specify --chromium_build_withpatch")
		return
	}
	if *runID == "" {
		glog.Error("Must specify --run_id")
		return
	}
	if *benchmarkName == "" {
		glog.Error("Must specify --benchmark_name")
		return
	}

	// Reset the local chromium checkout.
	if err := util.ResetCheckout(util.ChromiumSrcDir); err != nil {
		glog.Errorf("Could not reset %s: %s", util.ChromiumSrcDir, err)
		return
	}
	// Sync the local chromium checkout.
	if err := util.SyncDir(util.ChromiumSrcDir); err != nil {
		glog.Errorf("Could not gclient sync %s: %s", util.ChromiumSrcDir, err)
		return
	}

	if *targetPlatform == util.PLATFORM_ANDROID {
		if err := adb.VerifyLocalDevice(); err != nil {
			// Android device missing or offline.
			glog.Errorf("Could not find Android device: %s", err)
			return
		}
		// TODO(rmistry): Could this be a problem in swarming?
		// Kill adb server to make sure we start from a clean slate.
		skutil.LogErr(util.ExecuteCmd(util.BINARY_ADB, []string{"kill-server"}, []string{},
			util.ADB_ROOT_TIMEOUT, nil, nil))
		// Make sure adb shell is running as root.
		skutil.LogErr(util.ExecuteCmd(util.BINARY_ADB, []string{"root"}, []string{},
			util.ADB_ROOT_TIMEOUT, nil, nil))
	}

	// Instantiate GsUtil object.
	gs, err := util.NewGsUtil(nil)
	if err != nil {
		glog.Error(err)
		return
	}

	// Download the benchmark patch for this run from Google storage.
	benchmarkPatchName := *runID + ".benchmark.patch"
	tmpDir, err := ioutil.TempDir("", "patches")
	if err != nil {
		glog.Errorf("Could not create a temp dir: %s", err)
		return
	}
	defer skutil.RemoveAll(tmpDir)
	benchmarkPatchLocalPath := filepath.Join(tmpDir, benchmarkPatchName)
	remoteDir := filepath.Join(util.ChromiumPerfRunsDir, *runID)
	benchmarkPatchRemotePath := filepath.Join(remoteDir, benchmarkPatchName)
	respBody, err := gs.GetRemoteFileContents(benchmarkPatchRemotePath)
	if err != nil {
		glog.Errorf("Could not fetch %s: %s", benchmarkPatchRemotePath, err)
		return
	}
	defer skutil.Close(respBody)
	buf := new(bytes.Buffer)
	if _, err := buf.ReadFrom(respBody); err != nil {
		glog.Errorf("Could not read from %s: %s", benchmarkPatchRemotePath, err)
		return
	}
	if err := ioutil.WriteFile(benchmarkPatchLocalPath, buf.Bytes(), 0666); err != nil {
		glog.Errorf("Unable to create file %s: %s", benchmarkPatchLocalPath, err)
		return
	}
	// Apply benchmark patch to the local chromium checkout.
	if buf.Len() > 10 {
		if err := util.ApplyPatch(benchmarkPatchLocalPath, util.ChromiumSrcDir); err != nil {
			glog.Errorf("Could not apply Telemetry's patch in %s: %s", util.ChromiumSrcDir, err)
			return
		}
	}

	// Download the specified chromium builds.
	for _, chromiumBuild := range []string{*chromiumBuildNoPatch, *chromiumBuildWithPatch} {
		if err := gs.DownloadChromiumBuild(chromiumBuild); err != nil {
			glog.Error(err)
			return
		}
		//Delete the chromium build to save space when we are done.
		defer skutil.RemoveAll(filepath.Join(util.ChromiumBuildsDir, chromiumBuild))
	}

	chromiumBinaryNoPatch := filepath.Join(util.ChromiumBuildsDir, *chromiumBuildNoPatch, util.BINARY_CHROME)
	chromiumBinaryWithPatch := filepath.Join(util.ChromiumBuildsDir, *chromiumBuildWithPatch, util.BINARY_CHROME)

	// Download pagesets if they do not exist locally.
	pathToPagesets := filepath.Join(util.PagesetsDir, *pagesetType)
	if _, err := gs.DownloadSwarmingArtifacts(pathToPagesets, util.PAGESETS_DIR_NAME, *pagesetType, *startRange, *num); err != nil {
		glog.Error(err)
		return
	}
	defer skutil.RemoveAll(pathToPagesets)

	// Download archives if they do not exist locally.
	pathToArchives := filepath.Join(util.WebArchivesDir, *pagesetType)
	if _, err := gs.DownloadSwarmingArtifacts(pathToArchives, util.WEB_ARCHIVES_DIR_NAME, *pagesetType, *startRange, *num); err != nil {
		glog.Error(err)
		return
	}
	defer skutil.RemoveAll(pathToArchives)

	// Establish nopatch output paths.
	runIDNoPatch := fmt.Sprintf("%s-nopatch", *runID)
	localOutputDirNoPatch := filepath.Join(util.StorageDir, util.BenchmarkRunsDir, runIDNoPatch)
	skutil.RemoveAll(localOutputDirNoPatch)
	skutil.MkdirAll(localOutputDirNoPatch, 0700)
	defer skutil.RemoveAll(localOutputDirNoPatch)
	remoteDirNoPatch := filepath.Join(util.BenchmarkRunsDir, runIDNoPatch)

	// Establish withpatch output paths.
	runIDWithPatch := fmt.Sprintf("%s-withpatch", *runID)
	localOutputDirWithPatch := filepath.Join(util.StorageDir, util.BenchmarkRunsDir, runIDWithPatch)
	skutil.RemoveAll(localOutputDirWithPatch)
	skutil.MkdirAll(localOutputDirWithPatch, 0700)
	defer skutil.RemoveAll(localOutputDirWithPatch)
	remoteDirWithPatch := filepath.Join(util.BenchmarkRunsDir, runIDWithPatch)

	// Construct path to the ct_run_benchmark python script.
	pathToPyFiles := util.GetPathToPyFiles(!*worker_common.Local)

	fileInfos, err := ioutil.ReadDir(pathToPagesets)
	if err != nil {
		glog.Errorf("Unable to read the pagesets dir %s: %s", pathToPagesets, err)
		return
	}

	numWorkers := WORKER_POOL_SIZE
	if *targetPlatform == util.PLATFORM_ANDROID || !*runInParallel {
		// Do not run page sets in parallel if the target platform is Android.
		// This is because the nopatch/withpatch APK needs to be installed prior to
		// each run and this will interfere with the parallel runs. Instead of trying
		// to find a complicated solution to this, it makes sense for Android to
		// continue to be serial because it will help guard against
		// crashes/flakiness/inconsistencies which are more prevalent in mobile runs.
		numWorkers = 1
		glog.Infoln("===== Going to run the task serially =====")
	} else {
		glog.Infoln("===== Going to run the task with parallel chrome processes =====")
	}

	// Create channel that contains all pageset file names. This channel will
	// be consumed by the worker pool.
	pagesetRequests := util.GetClosedChannelOfPagesets(fileInfos)

	var wg sync.WaitGroup
	// Use a RWMutex for the chromeProcessesCleaner goroutine to communicate to
	// the workers (acting as "readers") when it wants to be the "writer" and
	// kill all zombie chrome processes.
	var mutex sync.RWMutex

	// Loop through workers in the worker pool.
	for i := 0; i < numWorkers; i++ {
		// Increment the WaitGroup counter.
		wg.Add(1)

		// Create and run a goroutine closure that captures SKPs.
		go func() {
			// Decrement the WaitGroup counter when the goroutine completes.
			defer wg.Done()

			for pagesetName := range pagesetRequests {

				mutex.RLock()

				if err := util.RunBenchmark(pagesetName, pathToPagesets, pathToPyFiles, localOutputDirNoPatch, *chromiumBuildNoPatch, chromiumBinaryNoPatch, runIDNoPatch, *browserExtraArgsNoPatch, *benchmarkName, *targetPlatform, *benchmarkExtraArgs, *pagesetType, *repeatBenchmark); err != nil {
					glog.Errorf("Error while running nopatch benchmark: %s", err)
					return
				}
				if err := util.RunBenchmark(pagesetName, pathToPagesets, pathToPyFiles, localOutputDirWithPatch, *chromiumBuildWithPatch, chromiumBinaryWithPatch, runIDWithPatch, *browserExtraArgsWithPatch, *benchmarkName, *targetPlatform, *benchmarkExtraArgs, *pagesetType, *repeatBenchmark); err != nil {
					glog.Errorf("Error while running withpatch benchmark: %s", err)
					return
				}
				mutex.RUnlock()
			}
		}()
	}

	if !*worker_common.Local {
		// Start the cleaner.
		go util.ChromeProcessesCleaner(&mutex, *chromeCleanerTimer)
	}

	// Wait for all spawned goroutines to complete.
	wg.Wait()

	// If "--output-format=csv-pivot-table" was specified then merge all CSV files and upload.
	if strings.Contains(*benchmarkExtraArgs, "--output-format=csv-pivot-table") {
		if err := util.MergeUploadCSVFilesOnWorkers(localOutputDirNoPatch, pathToPyFiles, runIDNoPatch, remoteDirNoPatch, gs, *startRange); err != nil {
			glog.Errorf("Error while processing withpatch CSV files: %s", err)
			return
		}
		if err := util.MergeUploadCSVFilesOnWorkers(localOutputDirWithPatch, pathToPyFiles, runIDWithPatch, remoteDirWithPatch, gs, *startRange); err != nil {
			glog.Errorf("Error while processing withpatch CSV files: %s", err)
			return
		}
	}
}
