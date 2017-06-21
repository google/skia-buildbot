// Application that does pixel diff using CT's webpage repository.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"
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
	startRange                = flag.Int("start_range", 1, "The number this worker will pixel diff from.")
	num                       = flag.Int("num", 100, "The total number of web pages to pixel diff from the start_range.")
	pagesetType               = flag.String("pageset_type", util.PAGESET_TYPE_10k, "The type of pagesets to run PixelDiff on. Eg: 10k, Mobile10k, All.")
	chromiumBuildNoPatch      = flag.String("chromium_build_nopatch", "", "The chromium build to use for the nopatch run.")
	chromiumBuildWithPatch    = flag.String("chromium_build_withpatch", "", "The chromium build to use for the withpatch run.")
	runID                     = flag.String("run_id", "", "The unique run id (typically requester + timestamp).")
	benchmarkExtraArgs        = flag.String("benchmark_extra_args", "", "The extra arguments that are passed to the specified benchmark.")
	browserExtraArgsNoPatch   = flag.String("browser_extra_args_nopatch", "", "The extra arguments that are passed to the browser while running the benchmark during the nopatch run.")
	browserExtraArgsWithPatch = flag.String("browser_extra_args_withpatch", "", "The extra arguments that are passed to the browser while running the benchmark during the withpatch run.")
	chromeCleanerTimer        = flag.Duration("cleaner_timer", 30*time.Minute, "How often all chrome processes will be killed on this slave.")
)

func pixelDiff() error {
	defer common.LogPanic()
	worker_common.Init()
	if !*worker_common.Local {
		defer util.CleanTmpDir()
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
	// Parse out the Chromium and Skia hashes.
	chromiumHash, _ := util.GetHashesFromBuild(*chromiumBuildNoPatch)
	// Sync the local chromium checkout.
	if err := util.SyncDir(util.ChromiumSrcDir, map[string]string{"src": chromiumHash}, []string{}); err != nil {
		return fmt.Errorf("Could not gclient sync %s: %s", util.ChromiumSrcDir, err)
	}

	// Instantiate GcsUtil object.
	gs, err := util.NewGcsUtil(nil)
	if err != nil {
		return fmt.Errorf("GcsUtil instantiation failed: %s", err)
	}

	// Download the custom webpages for this run from Google storage.
	customWebpagesName := *runID + ".custom_webpages.csv"
	tmpDir, err := ioutil.TempDir("", "custom_webpages")
	defer skutil.RemoveAll(tmpDir)
	remotePatchesDir := filepath.Join(util.BenchmarkRunsDir, *runID)
	if err != nil {
		return fmt.Errorf("Could not create tmpdir: %s", err)
	}
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
	// Download the specified chromium builds.
	for _, chromiumBuild := range chromiumBuilds {
		if err := gs.DownloadChromiumBuild(chromiumBuild); err != nil {
			return fmt.Errorf("Could not download chromium build %s: %s", chromiumBuild, err)
		}
		//Delete the chromium build to save space when we are done.
		defer skutil.RemoveAll(filepath.Join(util.ChromiumBuildsDir, chromiumBuild))
	}

	chromiumBinaryNoPatch := filepath.Join(util.ChromiumBuildsDir, *chromiumBuildNoPatch, util.BINARY_CHROME)
	chromiumBinaryWithPatch := filepath.Join(util.ChromiumBuildsDir, *chromiumBuildWithPatch, util.BINARY_CHROME)

	var pathToPagesets string
	if len(customWebpages) > 0 {
		pathToPagesets = filepath.Join(util.PagesetsDir, "custom")
		if err := util.CreateCustomPagesets(customWebpages, pathToPagesets); err != nil {
			return fmt.Errorf("Could not create custom pagesets: %s", err)
		}
	} else {
		// Download pagesets if they do not exist locally.
		pathToPagesets = filepath.Join(util.PagesetsDir, *pagesetType)
		if _, err := gs.DownloadSwarmingArtifacts(pathToPagesets, util.PAGESETS_DIR_NAME, *pagesetType, *startRange, *num); err != nil {
			return fmt.Errorf("Could not download pagesets: %s", err)
		}
	}
	defer skutil.RemoveAll(pathToPagesets)

	if !strings.Contains(*benchmarkExtraArgs, util.USE_LIVE_SITES_FLAGS) {
		// Download archives if they do not exist locally.
		pathToArchives := filepath.Join(util.WebArchivesDir, *pagesetType)
		if _, err := gs.DownloadSwarmingArtifacts(pathToArchives, util.WEB_ARCHIVES_DIR_NAME, *pagesetType, *startRange, *num); err != nil {
			return fmt.Errorf("Could not download archives: %s", err)
		}
		defer skutil.RemoveAll(pathToArchives)
	}

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

	fileInfos, err := ioutil.ReadDir(pathToPagesets)
	if err != nil {
		return fmt.Errorf("Unable to read the pagesets dir %s: %s", pathToPagesets, err)
	}

	sklog.Infof("===== Going to run the task with %d parallel chrome processes =====", WORKER_POOL_SIZE)
	// Create channel that contains all pageset file names. This channel will
	// be consumed by the worker pool.
	pagesetRequests := util.GetClosedChannelOfPagesets(fileInfos)
	timeoutSecs := util.PagesetTypeToInfo[*pagesetType].PixelDiffTimeoutSecs

	var wg sync.WaitGroup
	// Use a RWMutex for the chromeProcessesCleaner goroutine to communicate to
	// the workers (acting as "readers") when it wants to be the "writer" and
	// kill all zombie chrome processes.
	var mutex sync.RWMutex

	// Loop through workers in the worker pool.
	for i := 0; i < WORKER_POOL_SIZE; i++ {
		// Increment the WaitGroup counter.
		wg.Add(1)

		// Create and run a goroutine closure that captures screenshots.
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

				mutex.RLock()
				runScreenshotBenchmark(localOutputDirNoPatch, chromiumBinaryNoPatch, pagesetName, pathToPagesets, decodedPageset, timeoutSecs)
				runScreenshotBenchmark(localOutputDirWithPatch, chromiumBinaryWithPatch, pagesetName, pathToPagesets, decodedPageset, timeoutSecs)
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

	// Check to see if there is anything in the output dirs.
	outputDirs := []string{localOutputDirNoPatch, localOutputDirWithPatch}
	for _, d := range outputDirs {
		empty, err := skutil.IsDirEmpty(d)
		if err != nil {
			return fmt.Errorf("Could not read dir %s: %s", d, err)
		}
		if empty {
			return fmt.Errorf("Could not capture any screenshots in %s", d)
		}
	}

	// Upload screenshots to Google Storage.
	if err := gs.UploadDir(localOutputDirNoPatch, remoteDirNoPatch, false); err != nil {
		return fmt.Errorf("Could not upload images from %s to %s: %s", localOutputDirNoPatch, remoteDirNoPatch, err)
	}
	if err := gs.UploadDir(localOutputDirWithPatch, remoteDirWithPatch, false); err != nil {
		return fmt.Errorf("Could not upload images from %s to %s: %s", localOutputDirWithPatch, remoteDirWithPatch, err)
	}

	return nil
}

func runScreenshotBenchmark(outputPath, chromiumBinary, pagesetName, pathToPagesets string, decodedPageset util.PagesetVars, timeoutSecs int) {

	args := []string{
		filepath.Join(util.TelemetryBinariesDir, util.BINARY_RUN_BENCHMARK),
		util.BenchmarksToTelemetryName[util.BENCHMARK_SCREENSHOT],
		"--also-run-disabled-tests",
		"--pageset-repeat=1", // Only need one run for Pixel diffs.
		"--png-outdir=" + path.Join(outputPath, strings.TrimSuffix(pagesetName, filepath.Ext(pagesetName))),
		"--extra-browser-args=" + util.DEFAULT_BROWSER_ARGS,
		"--user-agent=" + decodedPageset.UserAgent,
		"--urls-list=" + decodedPageset.UrlsList,
		"--archive-data-file=" + decodedPageset.ArchiveDataFile,
		"--browser=exact",
		"--browser-executable=" + chromiumBinary,
		"--device=desktop",
	}
	// Split benchmark args if not empty and append to args.
	if *benchmarkExtraArgs != "" {
		args = append(args, strings.Fields(*benchmarkExtraArgs)...)
	}
	// Set the PYTHONPATH to the pagesets and the telemetry dirs.
	env := []string{
		fmt.Sprintf("PYTHONPATH=%s:%s:%s:%s:$PYTHONPATH", pathToPagesets, util.TelemetryBinariesDir, util.TelemetrySrcDir, util.CatapultSrcDir),
		"DISPLAY=:0",
	}
	// Execute run_benchmark and log if there are any errors.
	err := util.ExecuteCmd("python", args, env, time.Duration(timeoutSecs)*time.Second, nil, nil)
	if err != nil {
		sklog.Errorf("Error during run_benchmark: %s", err)
	}
}

func main() {
	retCode := 0
	if err := pixelDiff(); err != nil {
		sklog.Errorf("Error while running Pixel diff: %s", err)
		retCode = 255
	}
	os.Exit(retCode)
}
