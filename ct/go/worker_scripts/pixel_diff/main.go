// Application that does pixel diff using CT's webpage repository.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.skia.org/infra/ct/go/util"
	"go.skia.org/infra/ct/go/worker_scripts/worker_common"
	"go.skia.org/infra/go/sklog"
	skutil "go.skia.org/infra/go/util"
)

const (
	// The number of goroutines that will run in parallel to run pixel diff.
	WORKER_POOL_SIZE = 5

	// Hacky way to detect when webpages are missing.
	// See https://bugs.chromium.org/p/skia/issues/detail?id=6778&desc=2#c2
	SIZE_OF_IMAGES_WITH_404 = 7963

	// screenshot_ct specific flags.
	DC_EXTRA_SCREENSHOTS_FLAG = "--dc-extra-screenshots"
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
	chromeCleanerTimer        = flag.Duration("cleaner_timer", 45*time.Minute, "How often all chrome processes will be killed on this slave.")
)

func pixelDiff() error {
	ctx := context.Background()
	worker_common.Init(ctx)
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
		customWebpages = util.GetCustomPagesWithinRange(*startRange, *num, customWebpages)
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
		// Delete the chromium build to save space when we are done.
		defer skutil.RemoveAll(filepath.Join(util.ChromiumBuildsDir, chromiumBuild))
	}

	chromiumBinaryNoPatch := filepath.Join(util.ChromiumBuildsDir, *chromiumBuildNoPatch, util.BINARY_CHROME)
	chromiumBinaryWithPatch := filepath.Join(util.ChromiumBuildsDir, *chromiumBuildWithPatch, util.BINARY_CHROME)

	var pathToPagesets string
	if len(customWebpages) > 0 {
		pathToPagesets = filepath.Join(util.PagesetsDir, "custom")
		if err := util.CreateCustomPagesets(customWebpages, pathToPagesets, util.PLATFORM_LINUX); err != nil {
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

	baseRemoteDir, err := util.GetBasePixelDiffRemoteDir(*runID)
	if err != nil {
		return fmt.Errorf("Could not figure out the base remote dir: %s", err)
	}

	// Establish nopatch output paths.
	runIDNoPatch := fmt.Sprintf("%s-nopatch", *runID)
	localOutputDirNoPatch := filepath.Join(util.StorageDir, util.BenchmarkRunsDir, runIDNoPatch)
	skutil.RemoveAll(localOutputDirNoPatch)
	skutil.MkdirAll(localOutputDirNoPatch, 0700)
	defer skutil.RemoveAll(localOutputDirNoPatch)
	remoteDirNoPatch := filepath.Join(baseRemoteDir, "nopatch")

	// Establish withpatch output paths.
	runIDWithPatch := fmt.Sprintf("%s-withpatch", *runID)
	localOutputDirWithPatch := filepath.Join(util.StorageDir, util.BenchmarkRunsDir, runIDWithPatch)
	skutil.RemoveAll(localOutputDirWithPatch)
	skutil.MkdirAll(localOutputDirWithPatch, 0700)
	defer skutil.RemoveAll(localOutputDirWithPatch)
	remoteDirWithPatch := filepath.Join(baseRemoteDir, "withpatch")

	fileInfos, err := ioutil.ReadDir(pathToPagesets)
	if err != nil {
		return fmt.Errorf("Unable to read the pagesets dir %s: %s", pathToPagesets, err)
	}

	sklog.Infof("===== Going to run the task with %d parallel chrome processes =====", WORKER_POOL_SIZE)
	// Create channel that contains all pageset file names. This channel will
	// be consumed by the worker pool.
	pagesetRequests := util.GetClosedChannelOfPagesets(fileInfos)
	// Dict of rank to URL. Will be used when populating the metadata file.
	rankToURL := map[int]string{}
	// Mutex to control access to above map.
	var rankDictMutex sync.RWMutex

	// Calculate the timeout.
	timeoutSecs := util.GetRunBenchmarkTimeoutValue(*benchmarkExtraArgs, util.PagesetTypeToInfo[*pagesetType].PixelDiffTimeoutSecs)
	numExtraScreenshots := util.GetIntFlagValue(*benchmarkExtraArgs, DC_EXTRA_SCREENSHOTS_FLAG, 0)
	if numExtraScreenshots > 0 {
		// Increase the timeoutSecs.
		timeoutSecs = timeoutSecs * numExtraScreenshots
	}
	sklog.Infof("Using %d seconds for timeout", timeoutSecs)
	// Remove from benchmarkExtraArgs "special" flags that are recognized by CT but not
	// by the run_benchmark script.
	*benchmarkExtraArgs = util.RemoveFlagsFromArgs(*benchmarkExtraArgs, util.RUN_BENCHMARK_TIMEOUT_FLAG)

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
				rank, err := util.GetRankFromPageset(pagesetName)
				if err != nil {
					sklog.Errorf("Could not get rank out of pageset %s: %s", pagesetName, err)
					continue
				}
				rankDictMutex.Lock()
				rankToURL[rank] = decodedPageset.UrlsList
				rankDictMutex.Unlock()

				sklog.Infof("===== Processing %s =====", pagesetPath)

				mutex.RLock()
				runScreenshotBenchmark(ctx, localOutputDirNoPatch, chromiumBinaryNoPatch, pagesetName, pathToPagesets, decodedPageset, timeoutSecs, rank)
				runScreenshotBenchmark(ctx, localOutputDirWithPatch, chromiumBinaryWithPatch, pagesetName, pathToPagesets, decodedPageset, timeoutSecs, rank)
				mutex.RUnlock()
			}
		}()
	}

	if !*worker_common.Local {
		// Start the cleaner.
		go util.ChromeProcessesCleaner(ctx, &mutex, *chromeCleanerTimer)
	}

	// Wait for all spawned goroutines to complete.
	wg.Wait()

	outputDirs := []string{localOutputDirNoPatch, localOutputDirWithPatch}
	for _, d := range outputDirs {
		files, err := ioutil.ReadDir(d)
		if err != nil {
			return fmt.Errorf("Could not read dir %s: %s", d, err)
		}
		if len(files) == 0 {
			// Throw an error if we were unable to capture any screenshots.
			return fmt.Errorf("Could not capture any screenshots in %s", d)
		}
		for _, f := range files {
			if f.Size() == SIZE_OF_IMAGES_WITH_404 {
				path := filepath.Join(d, f.Name())
				sklog.Warningf("%s is an image with '404 Not Found'. Deleting it.", path)
				skutil.Remove(path)
			}
		}
	}

	// Write out the metadata file.
	if err := writeMetadataFile(localOutputDirNoPatch, "nopatch", rankToURL, gs); err != nil {
		return fmt.Errorf("Could not write metadata file for %s: %s", localOutputDirNoPatch, err)
	}
	if err := writeMetadataFile(localOutputDirWithPatch, "withpatch", rankToURL, gs); err != nil {
		return fmt.Errorf("Could not write metadata file for %s: %s", localOutputDirWithPatch, err)
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

func runScreenshotBenchmark(ctx context.Context, outputPath, chromiumBinary, pagesetName, pathToPagesets string, decodedPageset util.PagesetVars, timeoutSecs, rank int) {

	args := []string{
		filepath.Join(util.GetPathToTelemetryBinaries(*worker_common.Local), util.BINARY_RUN_BENCHMARK),
		util.BENCHMARK_SCREENSHOT,
		"--also-run-disabled-tests",
		"--png-outdir=" + filepath.Join(outputPath, strconv.Itoa(rank)),
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
	// Set Display to the env. Cannot pass PYTHONPATH because it interferes
	// with vpython. See crbug.com/826424.
	env := []string{
		"DISPLAY=:0",
	}
	// Append the original environment as well.
	for _, e := range os.Environ() {
		env = append(env, e)
	}

	// Execute run_benchmark and log if there are any errors.
	// Note: util.BENCHMARK_SCREENSHOT needs to be run with vpython because it
	// requires numpy and cv2. See crbug.com/826424.
	err := util.ExecuteCmd(ctx, "vpython", args, env, time.Duration(timeoutSecs)*time.Second, nil, nil)
	if err != nil {
		sklog.Errorf("Error during run_benchmark: %s", err)
	}
}

type Metadata struct {
	RunID             string       `json:"run_id"`
	ChromiumPatchLink string       `json:"chromium_patch"`
	SkiaPatchLink     string       `json:"skia_patch"`
	Screenshots       []Screenshot `json:"screenshots"`
}

type Screenshot struct {
	Type     string `json:"type"`
	Filename string `json:"filename"`
	Rank     int    `json:"rank"`
	URL      string `json:"url"`
}

func writeMetadataFile(outputDir, patchType string, rankToURL map[int]string, gs *util.GcsUtil) error {
	screenshots := []Screenshot{}
	indexDirs, err := filepath.Glob(filepath.Join(outputDir, "*"))
	if err != nil {
		return fmt.Errorf("Unable to read %s: %s", outputDir, err)
	}
	for _, indexDir := range indexDirs {
		index := filepath.Base(indexDir)
		rank, err := strconv.Atoi(index)
		if err != nil {
			return fmt.Errorf("Found a directory %s that is not a rank: %s", index, err)
		}
		imgFileInfos, err := ioutil.ReadDir(indexDir)
		if err != nil {
			return fmt.Errorf("Unable to read %s: %s", indexDir, err)
		}
		for _, fileInfo := range imgFileInfos {
			if fileInfo.IsDir() {
				// We are only interested in files.
				continue
			}
			screenshot := Screenshot{
				Type:     patchType,
				Filename: fileInfo.Name(),
				Rank:     rank,
				URL:      rankToURL[rank],
			}
			screenshots = append(screenshots, screenshot)
		}
		metadata := Metadata{
			RunID:             *runID,
			ChromiumPatchLink: util.GCS_HTTP_LINK + filepath.Join(util.GCSBucketName, util.BenchmarkRunsDir, *runID, *runID+".chromium.patch"),
			SkiaPatchLink:     util.GCS_HTTP_LINK + filepath.Join(util.GCSBucketName, util.BenchmarkRunsDir, *runID, *runID+".skia.patch"),
			Screenshots:       screenshots,
		}
		m, err := json.Marshal(&metadata)
		if err != nil {
			return fmt.Errorf("Could not marshall %s to json: %s", m, err)
		}
		localMetadataFileName := fmt.Sprintf("%d-%d.json", *startRange, *startRange+*num-1)
		localMetadataFilePath := filepath.Join(outputDir, localMetadataFileName)
		if err := ioutil.WriteFile(localMetadataFilePath, m, os.ModePerm); err != nil {
			return fmt.Errorf("Could not write to %s: %s", localMetadataFilePath, err)
		}
	}
	return nil
}

func main() {
	retCode := 0
	if err := pixelDiff(); err != nil {
		sklog.Errorf("Error while running Pixel diff: %s", err)
		retCode = 255
	}
	os.Exit(retCode)
}
