// Application that captures SKPs from CT's webpage archives.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"go.skia.org/infra/ct/go/util"
	"go.skia.org/infra/ct/go/worker_scripts/worker_common"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	skutil "go.skia.org/infra/go/util"
)

const (
	// The number of goroutines that will run in parallel to capture SKPs.
	WORKER_POOL_SIZE = 5
)

var (
	startRange         = flag.Int("start_range", 1, "The number this worker will capture SKPs from.")
	num                = flag.Int("num", 100, "The total number of SKPs to capture starting from the start_range.")
	pagesetType        = flag.String("pageset_type", util.PAGESET_TYPE_MOBILE_10k, "The type of pagesets to create SKPs from. Eg: 10k, Mobile10k, All.")
	chromiumBuild      = flag.String("chromium_build", "", "The chromium build that will be used to create the SKPs.")
	skpinfoRemotePath  = flag.String("skpinfo_remote_path", "", "The location of the skpinfo binary in Google Storage.")
	runID              = flag.String("run_id", "", "The unique run id (typically requester + timestamp).")
	targetPlatform     = flag.String("target_platform", util.PLATFORM_LINUX, "The platform the benchmark will run on (Android / Linux).")
	chromeCleanerTimer = flag.Duration("cleaner_timer", 30*time.Minute, "How often all chrome processes will be killed on this worker.")
)

func captureSkps() error {
	ctx := context.Background()
	httpClient, err := worker_common.Init(ctx, false /* useDepotTools */)
	if err != nil {
		return skerr.Wrap(err)
	}
	if !*worker_common.Local {
		defer util.CleanTmpDir()
	}
	defer util.TimeTrack(time.Now(), "Capturing SKPs")
	defer sklog.Flush()

	// Validate required arguments.
	if *chromiumBuild == "" {
		return errors.New("Must specify --chromium_build")
	}
	if *skpinfoRemotePath == "" {
		return errors.New("Must specify --skpinfo_remote_path")
	}
	if *runID == "" {
		return errors.New("Must specify --run_id")
	}
	if *targetPlatform == util.PLATFORM_ANDROID {
		return errors.New("Android is not yet supported for capturing SKPs.")
	}

	// Instantiate GcsUtil object.
	gs, err := util.NewGcsUtil(httpClient)
	if err != nil {
		return err
	}

	// Download the specified chromium build.
	if err := gs.DownloadChromiumBuild(*chromiumBuild); err != nil {
		return err
	}
	// Delete the chromium build to save space when we are done.
	defer skutil.RemoveAll(filepath.Join(util.ChromiumBuildsDir, *chromiumBuild))
	chromiumBinary := filepath.Join(util.ChromiumBuildsDir, *chromiumBuild, util.BINARY_CHROME)
	if *targetPlatform == util.PLATFORM_ANDROID {
		// Install the APK on the Android device.
		chromiumApkPath := filepath.Join(util.ChromiumBuildsDir, *chromiumBuild, util.ApkName)
		if err := util.InstallChromeAPK(ctx, chromiumApkPath); err != nil {
			return fmt.Errorf("Could not install the chromium APK: %s", err)
		}
	}

	// Copy over the skpinfo binary to this worker.
	skpinfoLocalPath := filepath.Join(os.TempDir(), util.BINARY_SKPINFO)
	respBody, err := gs.GetRemoteFileContents(*skpinfoRemotePath)
	if err != nil {
		return fmt.Errorf("Could not fetch %s: %s", *skpinfoRemotePath, err)
	}
	defer skutil.Close(respBody)
	writeErr := skutil.WithWriteFile(skpinfoLocalPath, func(w io.Writer) error {
		_, err = io.Copy(w, respBody)
		return err
	})
	if writeErr != nil {
		return fmt.Errorf("Failed to write to %s: %s", skpinfoLocalPath, writeErr)
	}
	// Downloaded skpinfo binary needs to be set as an executable.
	if err := os.Chmod(skpinfoLocalPath, 0777); err != nil {
		return fmt.Errorf("Failed to set %s as executable: %s", skpinfoLocalPath, err)
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
	util.MkdirAll(pathToSkps, 0700)
	defer skutil.RemoveAll(pathToSkps)

	// Construct path to the ct_run_benchmark python script.
	pathToPyFiles, err := util.GetPathToPyFiles(*worker_common.Local)
	if err != nil {
		return fmt.Errorf("Could not get path to py files: %s", err)
	}

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
					filepath.Join(util.GetPathToTelemetryBinaries(*worker_common.Local), util.BINARY_RUN_BENCHMARK),
					util.BENCHMARK_SKPICTURE_PRINTER,
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
				// Append the original environment as well.
				for _, e := range os.Environ() {
					env = append(env, e)
				}

				mutex.RLock()
				// Retry run_benchmark binary 3 times if there are any errors.
				retryAttempts := 3
				for i := 0; ; i++ {
					err = util.ExecuteCmd(ctx, "python", args, env, time.Duration(timeoutSecs)*time.Second, nil, nil)
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
		go util.ChromeProcessesCleaner(ctx, &mutex, *chromeCleanerTimer)
	}

	// Wait for all spawned goroutines to complete.
	wg.Wait()

	// Move and validate all SKP files.
	if err := util.ValidateSKPs(ctx, pathToSkps, pathToPyFiles, skpinfoLocalPath); err != nil {
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
