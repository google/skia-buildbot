// Application that captures SKPs from CT's webpage archives.
package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/skia-dev/glog"

	"go.skia.org/infra/ct/go/util"
	"go.skia.org/infra/ct/go/worker_scripts/worker_common"
	"go.skia.org/infra/go/common"
	skutil "go.skia.org/infra/go/util"
)

const (
	// The number of goroutines that will run in parallel to capture SKPs.
	WORKER_POOL_SIZE = 10
)

var (
	startRange         = flag.Int("start_range", 1, "The number this worker will capture SKPs from.")
	num                = flag.Int("num", 100, "The total number of SKPs to capture starting from the start_range.")
	pagesetType        = flag.String("pageset_type", util.PAGESET_TYPE_MOBILE_10k, "The type of pagesets to create SKPs from. Eg: 10k, Mobile10k, All.")
	chromiumBuild      = flag.String("chromium_build", "", "The chromium build that will be used to create the SKPs.")
	runID              = flag.String("run_id", "", "The unique run id (typically requester + timestamp).")
	targetPlatform     = flag.String("target_platform", util.PLATFORM_LINUX, "The platform the benchmark will run on (Android / Linux).")
	chromeCleanerTimer = flag.Duration("cleaner_timer", 30*time.Minute, "How often all chrome processes will be killed on this slave.")
)

func main() {
	defer common.LogPanic()
	worker_common.Init()
	defer util.TimeTrack(time.Now(), "Capturing SKPs")
	defer glog.Flush()

	// Validate required arguments.
	if *chromiumBuild == "" {
		glog.Fatal("Must specify --chromium_build")
	}
	if *runID == "" {
		glog.Fatal("Must specify --run_id")
	}
	if *targetPlatform == util.PLATFORM_ANDROID {
		glog.Fatal("Android is not yet supported for capturing SKPs.")
	}

	// Reset the local chromium checkout.
	if err := util.ResetCheckout(util.ChromiumSrcDir); err != nil {
		glog.Fatalf("Could not reset %s: %s", util.ChromiumSrcDir, err)
	}
	// Sync the local chromium checkout.
	if err := util.SyncDir(util.ChromiumSrcDir); err != nil {
		glog.Fatalf("Could not gclient sync %s: %s", util.ChromiumSrcDir, err)
	}

	// Instantiate GsUtil object.
	gs, err := util.NewGsUtil(nil)
	if err != nil {
		glog.Fatal(err)
	}

	// Download the specified chromium build.
	if err := gs.DownloadChromiumBuild(*chromiumBuild); err != nil {
		glog.Fatal(err)
	}
	// Delete the chromium build to save space when we are done.
	defer skutil.RemoveAll(filepath.Join(util.ChromiumBuildsDir, *chromiumBuild))
	chromiumBinary := filepath.Join(util.ChromiumBuildsDir, *chromiumBuild, util.BINARY_CHROME)
	if *targetPlatform == util.PLATFORM_ANDROID {
		// Install the APK on the Android device.
		if err := util.InstallChromeAPK(*chromiumBuild); err != nil {
			glog.Fatalf("Could not install the chromium APK: %s", err)
		}
	}

	// Download pagesets if they do not exist locally.
	pathToPagesets := filepath.Join(util.PagesetsDir, *pagesetType)
	if _, err := gs.DownloadSwarmingArtifacts(pathToPagesets, util.PAGESETS_DIR_NAME, *pagesetType, *startRange, *num); err != nil {
		glog.Fatal(err)
	}
	defer skutil.RemoveAll(pathToPagesets)

	// Download archives if they do not exist locally.
	pathToArchives := filepath.Join(util.WebArchivesDir, *pagesetType)
	archivesToIndex, err := gs.DownloadSwarmingArtifacts(pathToArchives, util.WEB_ARCHIVES_DIR_NAME, *pagesetType, *startRange, *num)
	if err != nil {
		glog.Fatal(err)
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
		glog.Fatalf("Unable to read the pagesets dir %s: %s", pathToPagesets, err)
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

				mutex.RLock()

				// Read the pageset.
				pagesetPath := filepath.Join(pathToPagesets, pagesetName)
				decodedPageset, err := util.ReadPageset(pagesetPath)
				if err != nil {
					glog.Errorf("Could not read %s: %s", pagesetPath, err)
					continue
				}

				glog.Infof("===== Processing %s =====", pagesetPath)

				skutil.LogErr(os.Chdir(pathToPyFiles))
				index, ok := archivesToIndex[decodedPageset.ArchiveDataFile]
				if !ok {
					glog.Errorf("%s not found in the archivesToIndex map", decodedPageset.ArchiveDataFile)
					continue
				}
				args := []string{
					filepath.Join(util.TelemetryBinariesDir, util.BINARY_RUN_BENCHMARK),
					util.BenchmarksToTelemetryName[util.BENCHMARK_SKPICTURE_PRINTER],
					"--also-run-disabled-tests",
					"--page-repeat=1", // Only need one run for SKPs.
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
				// Retry run_benchmark binary 3 times if there are any errors.
				retryAttempts := 3
				for i := 0; ; i++ {
					err = util.ExecuteCmd("python", args, env, time.Duration(timeoutSecs)*time.Second, nil, nil)
					if err == nil {
						break
					}
					if i >= (retryAttempts - 1) {
						glog.Errorf("%s failed inspite of 3 retries. Last error: %s", pagesetPath, err)
						break
					}
					time.Sleep(time.Second)
					glog.Warningf("Retrying due to error: %s", err)
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
		glog.Fatal(err)
	}

	// Check to see if there is anything in the pathToSKPs dir.
	skpsEmpty, err := skutil.IsDirEmpty(pathToSkps)
	if err != nil {
		glog.Fatal(err)
	}
	if skpsEmpty {
		glog.Fatalf("Could not create any SKP in %s", pathToSkps)
	}

	// Upload SKPs dir to Google Storage.
	if err := gs.UploadSwarmingArtifacts(util.SKPS_DIR_NAME, filepath.Join(*pagesetType, *chromiumBuild)); err != nil {
		glog.Fatal(err)
	}
}
