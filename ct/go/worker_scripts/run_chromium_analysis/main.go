// run_chromium_analysis is an application that runs the specified benchmark over
// CT's webpage archives. It is intended to be run on swarming bots.
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
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.skia.org/infra/ct/go/adb"
	"go.skia.org/infra/ct/go/util"
	"go.skia.org/infra/ct/go/worker_scripts/worker_common"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	skutil "go.skia.org/infra/go/util"
)

const (
	// The number of goroutines that will run in parallel to run benchmarks.
	WORKER_POOL_SIZE = 5
	// The number of allowed benchmark timeouts in a row before the worker
	// script fails.
	MAX_ALLOWED_SEQUENTIAL_TIMEOUTS = 20

	STDOUT_COUNT_CSV_FIELD = "CT_stdout_count"
	STDOUT_LINES_CSV_FIELD = "CT_stdout_lines"
)

var (
	startRange         = flag.Int("start_range", 1, "The number this worker will run benchmarks from.")
	num                = flag.Int("num", 100, "The total number of benchmarks to run starting from the start_range.")
	pagesetType        = flag.String("pageset_type", util.PAGESET_TYPE_MOBILE_10k, "The type of pagesets to analyze. Eg: 10k, Mobile10k, All.")
	chromiumBuild      = flag.String("chromium_build", "", "The chromium build to use.")
	runID              = flag.String("run_id", "", "The unique run id (typically requester + timestamp).")
	apkGsPath          = flag.String("apk_gs_path", "", "GS path to a custom APK to use instead of building one from scratch. Eg: gs://chrome-unsigned/android-B0urB0N/79.0.3922.0/arm_64/ChromeModern.apk")
	chromeBuildGsPath  = flag.String("chrome_build_gs_path", "", "GS path to a custom chrome build to use instead of building one from scratch. Eg: gs://chromium-browser-snapshots/Linux_x64/805044/chrome-linux.zip")
	benchmarkName      = flag.String("benchmark_name", "", "The telemetry benchmark to run on this worker.")
	benchmarkExtraArgs = flag.String("benchmark_extra_args", "", "The extra arguments that are passed to the specified benchmark.")
	browserExtraArgs   = flag.String("browser_extra_args", "", "The extra arguments that are passed to the browser while running the benchmark.")
	runInParallel      = flag.Bool("run_in_parallel", true, "Run the benchmark by bringing up multiple chrome instances in parallel.")
	targetPlatform     = flag.String("target_platform", util.PLATFORM_LINUX, "The platform the benchmark will run on (Android / Linux).")
	chromeCleanerTimer = flag.Duration("cleaner_timer", 15*time.Minute, "How often all chrome processes will be killed on this worker.")
	matchStdoutText    = flag.String("match_stdout_txt", "", "Looks for the specified string in the stdout of web page runs. The count of the text's occurence and the lines containing it are added to the CSV of the web page.")
	valueColumnName    = flag.String("value_column_name", "", "Which column's entries to use as field values when combining CSVs.")
)

func runChromiumAnalysis() error {
	ctx := context.Background()
	httpClient, err := worker_common.Init(ctx, false /* useDepotTools */)
	if err != nil {
		return skerr.Wrap(err)
	}
	if !*worker_common.Local {
		defer util.CleanTmpDir()
	}
	defer util.TimeTrack(time.Now(), "Running Chromium Analysis")
	defer sklog.Flush()

	// Validate required arguments.
	if *chromiumBuild == "" && *apkGsPath == "" && *chromeBuildGsPath == "" {
		return errors.New("Must specify either --chromium_build or --apk_gs_path or --chrome_build_gs_path")
	}
	if *runID == "" {
		return errors.New("Must specify --run_id")
	}
	if *benchmarkName == "" {
		return errors.New("Must specify --benchmark_name")
	}

	// Use defaults.
	if *valueColumnName == "" {
		*valueColumnName = util.DEFAULT_VALUE_COLUMN_NAME
	}

	// Instantiate GcsUtil object.
	gs, err := util.NewGcsUtil(httpClient)
	if err != nil {
		return err
	}

	tmpDir, err := ioutil.TempDir("", "patches")
	remotePatchesDir := path.Join(util.ChromiumAnalysisRunsStorageDir, *runID)

	// Download the custom webpages for this run from Google storage.
	customWebpagesName := *runID + ".custom_webpages.csv"
	if _, err := util.DownloadPatch(filepath.Join(tmpDir, customWebpagesName), path.Join(remotePatchesDir, customWebpagesName), gs); err != nil {
		return fmt.Errorf("Could not download %s: %s", customWebpagesName, err)
	}
	customWebpages, err := util.GetCustomPages(filepath.Join(tmpDir, customWebpagesName))
	if err != nil {
		return fmt.Errorf("Could not read custom webpages file %s: %s", customWebpagesName, err)
	}
	if len(customWebpages) > 0 {
		customWebpages = util.GetCustomPagesWithinRange(*startRange, *num, customWebpages)
	}

	chromiumBinary := util.BINARY_CHROME
	if *targetPlatform == util.PLATFORM_WINDOWS {
		chromiumBinary = util.BINARY_CHROME_WINDOWS
	} else if *targetPlatform == util.PLATFORM_ANDROID {
		chromiumBinary = util.ApkName
	}
	pathToBinaryDir := filepath.Join(util.ChromiumBuildsDir, *chromiumBuild)
	// Download the specified chromium build if a custom APK is not specified.
	if *apkGsPath == "" {
		if *chromeBuildGsPath == "" {
			pathToBinaryDir, err = gs.DownloadChromiumBuild(*chromiumBuild, chromiumBinary)
			if err != nil {
				return err
			}
		} else {
			// If a custom chrome build is specified then download that instead.
			customChromeBuildDir := filepath.Join(util.ChromiumBuildsDir, "custom-chrome-build")
			util.MkdirAll(customChromeBuildDir, 0700)
			r := regexp.MustCompile(`gs://(.+?)/(.*)/(.*\.zip)`)
			m := r.FindStringSubmatch(*chromeBuildGsPath)
			bucket := m[1]
			gsDir := m[2]
			zipName := m[3]
			pathToBinaryDir, err = gs.DownloadChromiumBuildFromTo(customChromeBuildDir, bucket, gsDir, zipName, chromiumBinary)
			if err != nil {
				return err
			}
		}
		// Delete the chromium build to save space when we are done.
		defer skutil.RemoveAll(pathToBinaryDir)
	} else {
		pathToBinaryDir = filepath.Join(util.ChromiumBuildsDir, util.CUSTOM_APK_DIR_NAME)
		util.MkdirAll(pathToBinaryDir, 0700)
		defer skutil.RemoveAll(pathToBinaryDir)

		// Download the specified APK from Google storage.
		r := regexp.MustCompile(`gs://(.+?)/(.*)`)
		m := r.FindStringSubmatch(*apkGsPath)
		bucket := m[1]
		remotePath := m[2]
		localPath := filepath.Join(pathToBinaryDir, chromiumBinary)
		if err := gs.DownloadRemoteFileFromBucket(bucket, remotePath, localPath); err != nil {
			return fmt.Errorf("Error downloading %s from %s to %s: %s", remotePath, bucket, localPath, err)
		}
	}
	chromiumBinaryPath := filepath.Join(pathToBinaryDir, chromiumBinary)

	var pathToPagesets string
	if len(customWebpages) > 0 {
		pathToPagesets = filepath.Join(util.PagesetsDir, "custom")
		if err := util.CreateCustomPagesets(customWebpages, pathToPagesets, *targetPlatform); err != nil {
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
	localOutputDir := filepath.Join(util.StorageDir, util.BenchmarkRunsDir, *runID)
	skutil.RemoveAll(localOutputDir)
	util.MkdirAll(localOutputDir, 0700)
	defer skutil.RemoveAll(localOutputDir)
	remoteDir := path.Join(util.BenchmarkRunsStorageDir, *runID)

	// Construct path to CT's python scripts.
	pathToPyFiles, err := util.GetPathToPyFiles(*worker_common.Local)
	if err != nil {
		return fmt.Errorf("Could not get path to py files: %s", err)
	}

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
		sklog.Info("===== Going to run the task serially =====")
	} else {
		sklog.Info("===== Going to run the task with parallel chrome processes =====")
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
	// The num of times run_benchmark binary should be retried if there are errors.
	retryNum := util.GetNumAnalysisRetriesValue(*benchmarkExtraArgs, 2)
	// Map that keeps track of which additional fields need to be added to the output CSV.
	pageRankToAdditionalFields := map[string]map[string]string{}
	// Mutex that controls access to the above map.
	var additionalFieldsMutex sync.Mutex

	// Loop through workers in the worker pool.
	for i := 0; i < numWorkers; i++ {
		// Increment the WaitGroup counter.
		wg.Add(1)

		// Create and run a goroutine closure that runs the benchmark.
		go func() {
			// Decrement the WaitGroup counter when the goroutine completes.
			defer wg.Done()

			for pagesetName := range pagesetRequests {

				mutex.RLock()
				for i := 0; ; i++ {
					output, err := util.RunBenchmark(ctx, pagesetName, pathToPagesets, pathToPyFiles, localOutputDir, chromiumBinaryPath, *runID, *browserExtraArgs, *benchmarkName, *targetPlatform, *benchmarkExtraArgs, *pagesetType, 1, !*worker_common.Local)
					if err == nil {
						timeoutTracker.Reset()
						// If *matchStdoutText is specified then add the number of times the text shows up in stdout
						// and the lines it shows up in to the pageRankToAdditionalFields map.
						// See skbug.com/7448 and skbug.com/7455 for context.
						if *matchStdoutText != "" && output != "" {
							rank, err := util.GetRankFromPageset(pagesetName)
							if err != nil {
								sklog.Errorf("Could not get rank out of pageset %s: %s", pagesetName, err)
								continue
							}
							linesWithText := []string{}
							for _, l := range strings.Split(output, "\n") {
								if strings.Contains(l, *matchStdoutText) {
									linesWithText = append(linesWithText, l)
								}
							}
							additionalFields := map[string]string{
								STDOUT_COUNT_CSV_FIELD: strconv.Itoa(strings.Count(output, *matchStdoutText)),
								STDOUT_LINES_CSV_FIELD: strings.Join(linesWithText, "\n"),
							}
							additionalFieldsMutex.Lock()
							pageRankToAdditionalFields[strconv.Itoa(rank)] = additionalFields
							additionalFieldsMutex.Unlock()

						}
						break
					} else {
						if exec.IsTimeout(err) {
							timeoutTracker.Increment()
						}
						// For Android runs make sure that the device is online. If not then stop early.
						if *targetPlatform == util.PLATFORM_ANDROID {
							if err := adb.VerifyLocalDevice(ctx); err != nil {
								sklog.Errorf("Could not find Android device: %s", err)
								return
							}
						}
					}
					if i >= retryNum {
						sklog.Errorf("%s failed inspite of %d retries. Last error: %s", pagesetName, retryNum, err)
						break
					}
					time.Sleep(time.Second)
					sklog.Warningf("Retrying due to error: %s", err)
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
		go util.ChromeProcessesCleaner(ctx, &mutex, *chromeCleanerTimer)
	}

	// Wait for all spawned goroutines to complete.
	wg.Wait()

	if timeoutTracker.Read() > MAX_ALLOWED_SEQUENTIAL_TIMEOUTS {
		return fmt.Errorf("There were %d sequential timeouts.", MAX_ALLOWED_SEQUENTIAL_TIMEOUTS)
	}

	// If "--output-format=csv" was specified then merge all CSV files and upload.
	if strings.Contains(*benchmarkExtraArgs, "--output-format=csv") {
		if err := util.MergeUploadCSVFilesOnWorkers(ctx, localOutputDir, pathToPyFiles, *runID, remoteDir, *valueColumnName, gs, *startRange, true /* handleStrings */, true /* addRank */, pageRankToAdditionalFields); err != nil {
			return fmt.Errorf("Error while processing withpatch CSV files: %s", err)
		}
	}

	return nil
}

func main() {
	retCode := 0
	if err := runChromiumAnalysis(); err != nil {
		sklog.Errorf("Error while running chromium analysis: %s", err)
		retCode = 255
	}
	os.Exit(retCode)
}
