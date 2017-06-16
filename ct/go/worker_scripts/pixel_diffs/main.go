// Application that does Pixel diff from CT's webpage archives.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"go.skia.org/infra/go/sklog"

	"go.skia.org/infra/ct/go/util"
	"go.skia.org/infra/ct/go/worker_scripts/worker_common"
	"go.skia.org/infra/go/common"
	skutil "go.skia.org/infra/go/util"
)

const (
	// The number of goroutines that will run in parallel to run pixel diff.
	WORKER_POOL_SIZE = 10
)

var (
	startRange                = flag.Int("start_range", 1, "The number this worker will capture SKPs from.")
	num                       = flag.Int("num", 100, "The total number of SKPs to capture starting from the start_range.")
	pagesetType               = flag.String("pageset_type", util.PAGESET_TYPE_MOBILE_10k, "The type of pagesets to create SKPs from. Eg: 10k, Mobile10k, All.")
	chromiumBuildNoPatch      = flag.String("chromium_build_nopatch", "", "The chromium build to use for the nopatch run.")
	chromiumBuildWithPatch    = flag.String("chromium_build_withpatch", "", "The chromium build to use for the withpatch run.")
	runID                     = flag.String("run_id", "", "The unique run id (typically requester + timestamp).")
	benchmarkExtraArgs        = flag.String("benchmark_extra_args", "", "The extra arguments that are passed to the specified benchmark.")
	browserExtraArgsNoPatch   = flag.String("browser_extra_args_nopatch", "", "The extra arguments that are passed to the browser while running the benchmark during the nopatch run.")
	browserExtraArgsWithPatch = flag.String("browser_extra_args_withpatch", "", "The extra arguments that are passed to the browser while running the benchmark during the withpatch run.")
	chromeCleanerTimer        = flag.Duration("cleaner_timer", 30*time.Minute, "How often all chrome processes will be killed on this slave.")
)

func captureSkps() error {
	defer common.LogPanic()
	worker_common.Init()
	if !*worker_common.Local {
		//defer util.CleanTmpDir()
	}
	defer util.TimeTrack(time.Now(), "Pixel diffing")
	defer sklog.Flush()

	// Validate required arguments.
	if *chromiumBuildNoPatch == "" {
		return errors.New("Must specify --chromium_build_nopatch")
	}
	if *chromiumBuildWithPatch == "" {
		return errors.New("Must specify --chromium_build_withpatch")
	}
	if *runID == "" {
		return errors.New("Must specify --run_id")
	}

	// Reset the local chromium checkout.
	if err := util.ResetChromiumCheckout(util.ChromiumSrcDir); err != nil {
		return fmt.Errorf("Could not reset %s: %s", util.ChromiumSrcDir, err)
	}
	// Sync the local chromium checkout.
	if err := util.SyncDir(util.ChromiumSrcDir, map[string]string{}, []string{}); err != nil {
		return fmt.Errorf("Could not gclient sync %s: %s", util.ChromiumSrcDir, err)
	}

	// Instantiate GcsUtil object.
	gs, err := util.NewGcsUtil(nil)
	if err != nil {
		return err
	}

	// Download the custom webpages for this run from Google storage.
	customWebpagesName := *runID + ".custom_webpages.csv"
	if _, err := util.DownloadPatch(filepath.Join(tmpDir, customWebpagesName), filepath.Join(remotePatchesDir, customWebpagesName), gs); err != nil {
		return fmt.Errorf("Could not download %s: %s", customWebpagesName, err)
	}
	customWebpages, err := util.GetCustomPages(filepath.Join(tmpDir, customWebpagesName))
	if err != nil {
		return fmt.Errorf("Could not read custom webpages file %s: %s", customWebpagesName, err)
	}
	if len(customWebpages) > 0 {
		startIndex := *startRange - 1
		endIndex := skutil.MinInt(*startRange+*num, len(customWebpages))
		customWebpages = customWebpages[startIndex:endIndex]
	}

	chromiumBuilds := []string{*chromiumBuildNoPatch}
	// No point downloading the same build twice. Download only if builds are different.
	if *chromiumBuildNoPatch != *chromiumBuildWithPatch {
		chromiumBuilds = append(chromiumBuilds, *chromiumBuildWithPatch)
	}
	//// Download the specified chromium builds.
	//for _, chromiumBuild := range chromiumBuilds {
	//	if err := gs.DownloadChromiumBuild(chromiumBuild); err != nil {
	//		return err
	//	}
	//	//Delete the chromium build to save space when we are done.
	//	//defer skutil.RemoveAll(filepath.Join(util.ChromiumBuildsDir, chromiumBuild))
	//}

	chromiumBinaryNoPatch := filepath.Join(util.ChromiumBuildsDir, *chromiumBuildNoPatch, util.BINARY_CHROME)
	chromiumBinaryWithPatch := filepath.Join(util.ChromiumBuildsDir, *chromiumBuildWithPatch, util.BINARY_CHROME)

	var pathToPagesets string
	if len(customWebpages) > 0 {
		pathToPagesets = filepath.Join(util.PagesetsDir, "custom")
		if err := util.CreateCustomPagesets(customWebpages, pathToPagesets); err != nil {
			return err
		}
	} else {
		// Download pagesets if they do not exist locally.
		pathToPagesets = filepath.Join(util.PagesetsDir, *pagesetType)
		if _, err := gs.DownloadSwarmingArtifacts(pathToPagesets, util.PAGESETS_DIR_NAME, *pagesetType, *startRange, *num); err != nil {
			return err
		}
	}
	defer skutil.RemoveAll(pathToPagesets)

	if !strings.Contains(*benchmarkExtraArgs, util.USE_LIVE_SITES_FLAGS) {
		// Download archives if they do not exist locally.
		pathToArchives := filepath.Join(util.WebArchivesDir, *pagesetType)
		if _, err := gs.DownloadSwarmingArtifacts(pathToArchives, util.WEB_ARCHIVES_DIR_NAME, *pagesetType, *startRange, *num); err != nil {
			return err
		}
		defer skutil.RemoveAll(pathToArchives)
	}

	// Establish nopatch output paths.
	runIDNoPatch := fmt.Sprintf("%s-nopatch", *runID)
	localOutputDirNoPatch := filepath.Join(util.StorageDir, util.BenchmarkRunsDir, runIDNoPatch)
	skutil.RemoveAll(localOutputDirNoPatch)
	skutil.MkdirAll(localOutputDirNoPatch, 0700)
	//defer skutil.RemoveAll(localOutputDirNoPatch)
	remoteDirNoPatch := filepath.Join(util.BenchmarkRunsDir, runIDNoPatch)

	// Establish withpatch output paths.
	runIDWithPatch := fmt.Sprintf("%s-withpatch", *runID)
	localOutputDirWithPatch := filepath.Join(util.StorageDir, util.BenchmarkRunsDir, runIDWithPatch)
	skutil.RemoveAll(localOutputDirWithPatch)
	skutil.MkdirAll(localOutputDirWithPatch, 0700)
	//defer skutil.RemoveAll(localOutputDirWithPatch)
	remoteDirWithPatch := filepath.Join(util.BenchmarkRunsDir, runIDWithPatch)

	// Construct path to the ct_run_benchmark python script.
	pathToPyFiles := util.GetPathToPyFiles(!*worker_common.Local)

	fileInfos, err := ioutil.ReadDir(pathToPagesets)
	if err != nil {
		return fmt.Errorf("Unable to read the pagesets dir %s: %s", pathToPagesets, err)
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
		sklog.Infoln("===== Going to run the task serially =====")
	} else {
		sklog.Infoln("===== Going to run the task with parallel chrome processes =====")
	}

	// Create channel that contains all pageset file names. This channel will
	// be consumed by the worker pool.
	pagesetRequests := util.GetClosedChannelOfPagesets(fileInfos)

	var wg sync.WaitGroup
	// Use a RWMutex for the chromeProcessesCleaner goroutine to communicate to
	// the workers (acting as "readers") when it wants to be the "writer" and
	// kill all zombie chrome processes.
	var mutex sync.RWMutex

	timeoutTracker := util.TimeoutTracker{}

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
				noPatchErr := util.RunBenchmark(pagesetName, pathToPagesets, pathToPyFiles, localOutputDirNoPatch, *chromiumBuildNoPatch, chromiumBinaryNoPatch, runIDNoPatch, *browserExtraArgsNoPatch, *benchmarkName, *targetPlatform, *benchmarkExtraArgs, *pagesetType, *repeatBenchmark)
				if noPatchErr != nil && exec.IsTimeout(noPatchErr) {
					timeoutTracker.Increment()
				} else {
					timeoutTracker.Reset()
				}
				withPatchErr := util.RunBenchmark(pagesetName, pathToPagesets, pathToPyFiles, localOutputDirWithPatch, *chromiumBuildWithPatch, chromiumBinaryWithPatch, runIDWithPatch, *browserExtraArgsWithPatch, *benchmarkName, *targetPlatform, *benchmarkExtraArgs, *pagesetType, *repeatBenchmark)
				if withPatchErr != nil && exec.IsTimeout(withPatchErr) {
					timeoutTracker.Increment()
				} else {
					timeoutTracker.Reset()
				}
				mutex.RUnlock()

				if timeoutTracker.Read() > MAX_ALLOWED_SEQUENTIAL_TIMEOUTS {
					sklog.Errorf("Ran into %d sequential timeouts. Something is wrong. Killing the goroutine.", MAX_ALLOWED_SEQUENTIAL_TIMEOUTS)
					return
				}
			}
		}()
	}

	if !*worker_common.Local {
		// Start the cleaner.
		go util.ChromeProcessesCleaner(&mutex, *chromeCleanerTimer)
	}

	// Wait for all spawned goroutines to complete.
	wg.Wait()

	if timeoutTracker.Read() > MAX_ALLOWED_SEQUENTIAL_TIMEOUTS {
		return fmt.Errorf("There were %d sequential timeouts.", MAX_ALLOWED_SEQUENTIAL_TIMEOUTS)
	}

	// If "--output-format=csv-pivot-table" was specified then merge all CSV files and upload.
	if strings.Contains(*benchmarkExtraArgs, "--output-format=csv-pivot-table") {
		if err := util.MergeUploadCSVFilesOnWorkers(localOutputDirNoPatch, pathToPyFiles, runIDNoPatch, remoteDirNoPatch, gs, *startRange, true /* handleStrings */); err != nil {
			return fmt.Errorf("Error while processing withpatch CSV files: %s", err)
		}
		if err := util.MergeUploadCSVFilesOnWorkers(localOutputDirWithPatch, pathToPyFiles, runIDWithPatch, remoteDirWithPatch, gs, *startRange, true /* handleStrings */); err != nil {
			return fmt.Errorf("Error while processing withpatch CSV files: %s", err)
		}
	}

	return nil

	// Below stuff needed?????????????????
	// Download the specified chromium build.
	if err := gs.DownloadChromiumBuild(*chromiumBuild); err != nil {
		return err
	}
	// Delete the chromium build to save space when we are done.
	defer skutil.RemoveAll(filepath.Join(util.ChromiumBuildsDir, *chromiumBuild))
	chromiumBinary := filepath.Join(util.ChromiumBuildsDir, *chromiumBuild, util.BINARY_CHROME)
	if *targetPlatform == util.PLATFORM_ANDROID {
		// Install the APK on the Android device.
		if err := util.InstallChromeAPK(*chromiumBuild); err != nil {
			return fmt.Errorf("Could not install the chromium APK: %s", err)
		}
	}

	// Download pagesets if they do not exist locally.
	pathToPagesets := filepath.Join(util.PagesetsDir, *pagesetType)
	if _, err := gs.DownloadSwarmingArtifacts(pathToPagesets, util.PAGESETS_DIR_NAME, *pagesetType, *startRange, *num); err != nil {
		return err
	}
	defer skutil.RemoveAll(pathToPagesets)

	// Download archives if they do not exist locally.
	pathToArchives := filepath.Join(util.WebArchivesDir, *pagesetType)
	archivesToIndex, err := gs.DownloadSwarmingArtifacts(pathToArchives, util.WEB_ARCHIVES_DIR_NAME, *pagesetType, *startRange, *num)
	if err != nil {
		return err
	}
	defer skutil.RemoveAll(pathToArchives)

	// Create the dir that SKPs will be stored in.
	pathToSkps := filepath.Join(util.SkpsDir, *pagesetType, *chromiumBuild)
	// Delete and remake the local SKPs directory.
	skutil.RemoveAll(pathToSkps)
	skutil.MkdirAll(pathToSkps, 0700)
	defer skutil.RemoveAll(pathToSkps)

	// Construct path to the ct_run_benchmark python script.
	pathToPyFiles := util.GetPathToPyFiles(!*worker_common.Local)

	timeoutSecs := util.PagesetTypeToInfo[*pagesetType].CaptureSKPsTimeoutSecs
	fileInfos, err := ioutil.ReadDir(pathToPagesets)
	if err != nil {
		return fmt.Errorf("Unable to read the pagesets dir %s: %s", pathToPagesets, err)
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
	for i := 0; i < WORKER_POOL_SIZE; i++ {
		// Increment the WaitGroup counter.
		wg.Add(1)

		// Create and run a goroutine closure that captures SKPs.
		go func() {
			// Decrement the WaitGroup counter when the goroutine completes.
			defer wg.Done()

			for pagesetName := range pagesetRequests {

				// Read the pageset.
				pagesetPath := filepath.Join(pathToPagesets, pagesetName)
				decodedPageset, err := util.ReadPageset(pagesetPath)
				if err != nil {
					sklog.Errorf("Could not read %s: %s", pagesetPath, err)
					continue
				}

				sklog.Infof("===== Processing %s =====", pagesetPath)

				skutil.LogErr(os.Chdir(pathToPyFiles))
				index, ok := archivesToIndex[decodedPageset.ArchiveDataFile]
				if !ok {
					sklog.Errorf("%s not found in the archivesToIndex map", decodedPageset.ArchiveDataFile)
					continue
				}
				args := []string{
					filepath.Join(util.TelemetryBinariesDir, util.BINARY_RUN_BENCHMARK),
					util.BenchmarksToTelemetryName[util.BENCHMARK_SKPICTURE_PRINTER],
					"--also-run-disabled-tests",
					"--pageset-repeat=1", // Only need one run for SKPs.
					"--skp-outdir=" + path.Join(pathToSkps, strconv.Itoa(index)),
					"--extra-browser-args=" + util.DEFAULT_BROWSER_ARGS,
					"--user-agent=" + decodedPageset.UserAgent,
					"--urls-list=" + decodedPageset.UrlsList,
					"--archive-data-file=" + decodedPageset.ArchiveDataFile,
				}
				// Figure out which browser and device should be used.
				if *targetPlatform == util.PLATFORM_ANDROID {
					args = append(args, "--browser=android-chromium")
				} else {
					args = append(args, "--browser=exact", "--browser-executable="+chromiumBinary)
					args = append(args, "--device=desktop")
				}

				// Set the PYTHONPATH to the pagesets and the telemetry dirs.
				env := []string{
					fmt.Sprintf("PYTHONPATH=%s:%s:%s:%s:$PYTHONPATH", pathToPagesets, util.TelemetryBinariesDir, util.TelemetrySrcDir, util.CatapultSrcDir),
					"DISPLAY=:0",
				}

				mutex.RLock()
				// Retry run_benchmark binary 3 times if there are any errors.
				retryAttempts := 3
				for i := 0; ; i++ {
					err = util.ExecuteCmd("python", args, env, time.Duration(timeoutSecs)*time.Second, nil, nil)
					if err == nil {
						break
					}
					if i >= (retryAttempts - 1) {
						sklog.Errorf("%s failed inspite of 3 retries. Last error: %s", pagesetPath, err)
						break
					}
					time.Sleep(time.Second)
					sklog.Warningf("Retrying due to error: %s", err)
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

	// Move and validate all SKP files.
	if err := util.ValidateSKPs(pathToSkps, pathToPyFiles); err != nil {
		return err
	}

	// Check to see if there is anything in the pathToSKPs dir.
	skpsEmpty, err := skutil.IsDirEmpty(pathToSkps)
	if err != nil {
		return err
	}
	if skpsEmpty {
		return fmt.Errorf("Could not create any SKP in %s", pathToSkps)
	}

	// Upload SKPs dir to Google Storage.
	if err := gs.UploadSwarmingArtifacts(util.SKPS_DIR_NAME, filepath.Join(*pagesetType, *chromiumBuild)); err != nil {
		return err
	}

	return nil
}

func main() {
	retCode := 0
	if err := captureSkps(); err != nil {
		sklog.Errorf("Error while capturing SKPs: %s", err)
		retCode = 255
	}
	os.Exit(retCode)
}
