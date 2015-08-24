// Application that captures SKPs from CT's webpage archives.
package main

import (
	"encoding/csv"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"github.com/skia-dev/glog"

	"strings"

	"go.skia.org/infra/ct/go/util"
	"go.skia.org/infra/go/common"
	skutil "go.skia.org/infra/go/util"
)

const (
	// The number of goroutines that will run in parallel to capture SKPs.
	WORKER_POOL_SIZE = 10
)

var (
	workerNum          = flag.Int("worker_num", 1, "The number of this CT worker. It will be in the {1..100} range.")
	pagesetType        = flag.String("pageset_type", util.PAGESET_TYPE_MOBILE_10k, "The type of pagesets to create SKPs from. Eg: 10k, Mobile10k, All.")
	chromiumBuild      = flag.String("chromium_build", "", "The chromium build that will be used to create the SKPs.")
	runID              = flag.String("run_id", "", "The unique run id (typically requester + timestamp).")
	targetPlatform     = flag.String("target_platform", util.PLATFORM_LINUX, "The platform the benchmark will run on (Android / Linux).")
	chromeCleanerTimer = flag.Duration("cleaner_timer", 30*time.Minute, "How often all chrome processes will be killed on this slave.")
)

func main() {
	common.Init()
	defer util.CleanTmpDir()
	defer util.TimeTrack(time.Now(), "Capturing SKPs")
	defer glog.Flush()

	// Validate required arguments.
	if *chromiumBuild == "" {
		glog.Error("Must specify --chromium_build")
		return
	}
	if *runID == "" {
		glog.Error("Must specify --run_id")
		return
	}
	if *targetPlatform == util.PLATFORM_ANDROID {
		glog.Error("Android is not yet supported for capturing SKPs.")
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

	// Create the task file so that the master knows this worker is still busy.
	skutil.LogErr(util.CreateTaskFile(util.ACTIVITY_CAPTURING_SKPS))
	defer util.DeleteTaskFile(util.ACTIVITY_CAPTURING_SKPS)

	// Instantiate GsUtil object.
	gs, err := util.NewGsUtil(nil)
	if err != nil {
		glog.Error(err)
		return
	}

	// Download the specified chromium build.
	if err := gs.DownloadChromiumBuild(*chromiumBuild); err != nil {
		glog.Error(err)
		return
	}
	// Delete the chromium build to save space when we are done.
	defer skutil.RemoveAll(filepath.Join(util.ChromiumBuildsDir, *chromiumBuild))
	chromiumBinary := filepath.Join(util.ChromiumBuildsDir, *chromiumBuild, util.BINARY_CHROME)
	if *targetPlatform == util.PLATFORM_ANDROID {
		// Install the APK on the Android device.
		chromiumApk := filepath.Join(util.ChromiumBuildsDir, *chromiumBuild, util.ApkName)
		if err := util.ExecuteCmd(util.BINARY_ADB, []string{"install", "-r", chromiumApk}, []string{}, 5*time.Minute, nil, nil); err != nil {
			glog.Errorf("Could not install the chromium APK: %s", err)
			return
		}
	}

	// Download pagesets if they do not exist locally.
	if err := gs.DownloadWorkerArtifacts(util.PAGESETS_DIR_NAME, *pagesetType, *workerNum); err != nil {
		glog.Error(err)
		return
	}
	pathToPagesets := filepath.Join(util.PagesetsDir, *pagesetType)

	// Download archives if they do not exist locally.
	if err := gs.DownloadWorkerArtifacts(util.WEB_ARCHIVES_DIR_NAME, *pagesetType, *workerNum); err != nil {
		glog.Error(err)
		return
	}

	// Create the dir that SKPs will be stored in.
	pathToSkps := filepath.Join(util.SkpsDir, *pagesetType, *chromiumBuild)
	// Delete and remake the local SKPs directory.
	skutil.RemoveAll(pathToSkps)
	skutil.MkdirAll(pathToSkps, 0700)

	// Establish output paths.
	localOutputDir := filepath.Join(util.StorageDir, util.BenchmarkRunsDir, *runID)
	skutil.RemoveAll(localOutputDir)
	skutil.MkdirAll(localOutputDir, 0700)
	defer skutil.RemoveAll(localOutputDir)

	// Construct path to the ct_run_benchmark python script.
	_, currentFile, _, _ := runtime.Caller(0)
	pathToPyFiles := filepath.Join(
		filepath.Dir((filepath.Dir(filepath.Dir(filepath.Dir(currentFile))))),
		"py")

	timeoutSecs := util.PagesetTypeToInfo[*pagesetType].CaptureSKPsTimeoutSecs
	fileInfos, err := ioutil.ReadDir(pathToPagesets)
	if err != nil {
		glog.Errorf("Unable to read the pagesets dir %s: %s", pathToPagesets, err)
		return
	}

	// Create channel that contains all pageset file names. This channel will
	// be consumed by the worker pool.
	pagesetRequests := getClosedChannelOfPagesets(fileInfos)

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

				pagesetBaseName := filepath.Base(pagesetName)
				// Convert the filename into a format consumable by the run_benchmarks
				// binary.
				pagesetNameNoExt := strings.TrimSuffix(pagesetBaseName, filepath.Ext(pagesetBaseName))
				pagesetPath := filepath.Join(pathToPagesets, pagesetName)

				glog.Infof("===== Processing %s =====", pagesetPath)

				skutil.LogErr(os.Chdir(pathToPyFiles))
				args := []string{
					util.BINARY_RUN_BENCHMARK,
					fmt.Sprintf("%s.%s", util.BENCHMARK_SKPICTURE_PRINTER, util.BenchmarksToPagesetName[util.BENCHMARK_SKPICTURE_PRINTER]),
					"--page-set-name=" + pagesetNameNoExt,
					"--page-set-base-dir=" + pathToPagesets,
					"--also-run-disabled-tests",
					"--page-repeat=1", // Only need one run for SKPs.
					"--skp-outdir=" + pathToSkps,
					"--extra-browser-args=" + util.DEFAULT_BROWSER_ARGS,
				}
				// Figure out which browser should be used.
				if *targetPlatform == util.PLATFORM_ANDROID {
					args = append(args, "--browser=android-chrome-shell")
				} else {
					args = append(args, "--browser=exact", "--browser-executable="+chromiumBinary)
				}
				// Set the PYTHONPATH to the pagesets and the telemetry dirs.
				env := []string{
					fmt.Sprintf("PYTHONPATH=%s:%s:%s:$PYTHONPATH", pathToPagesets, util.TelemetryBinariesDir, util.TelemetrySrcDir),
					"DISPLAY=:0",
				}
				skutil.LogErr(
					util.ExecuteCmd("python", args, env, time.Duration(timeoutSecs)*time.Second, nil, nil))

				mutex.RUnlock()

			}
		}()
	}

	// Start the cleaner.
	go chromeProcessesCleaner(&mutex)

	// Wait for all spawned goroutines to complete.
	wg.Wait()

	// Move, validate and upload all SKP files.
	// List all directories in pathToSkps and copy out the skps.
	skpFileInfos, err := ioutil.ReadDir(pathToSkps)
	if err != nil {
		glog.Errorf("Unable to read %s: %s", pathToSkps, err)
		return
	}
	for _, fileInfo := range skpFileInfos {
		if !fileInfo.IsDir() {
			// We are only interested in directories.
			continue
		}
		skpName := fileInfo.Name()
		// Find the largest layer in this directory.
		layerInfos, err := ioutil.ReadDir(filepath.Join(pathToSkps, skpName))
		if err != nil {
			glog.Errorf("Unable to read %s: %s", filepath.Join(pathToSkps, skpName), err)
		}
		if len(layerInfos) > 0 {
			largestLayerInfo := layerInfos[0]
			for _, layerInfo := range layerInfos {
				fmt.Println(layerInfo.Size())
				if layerInfo.Size() > largestLayerInfo.Size() {
					largestLayerInfo = layerInfo
				}
			}
			// Only save SKPs greater than 6000 bytes. Less than that are probably
			// malformed.
			if largestLayerInfo.Size() > 6000 {
				layerPath := filepath.Join(pathToSkps, skpName, largestLayerInfo.Name())
				skutil.Rename(layerPath, filepath.Join(pathToSkps, skpName+".skp"))
			} else {
				glog.Warningf("Skipping %s because size was less than 5000 bytes", skpName)
			}
		}
		// We extracted what we needed from the directory, now delete it.
		skutil.RemoveAll(filepath.Join(pathToSkps, skpName))
	}

	glog.Info("Calling remove_invalid_skps.py")
	// Sync Skia tree.
	skutil.LogErr(util.SyncDir(util.SkiaTreeDir))
	// Build tools.
	skutil.LogErr(util.BuildSkiaTools())
	// Run remove_invalid_skps.py
	pathToRemoveSKPs := filepath.Join(pathToPyFiles, "remove_invalid_skps.py")
	pathToSKPInfo := filepath.Join(util.SkiaTreeDir, "out", "Release", "skpinfo")
	args := []string{
		pathToRemoveSKPs,
		"--skp_dir=" + pathToSkps,
		"--path_to_skpinfo=" + pathToSKPInfo,
	}
	skutil.LogErr(util.ExecuteCmd("python", args, []string{}, 3*time.Hour, nil, nil))

	// Write timestamp to the SKPs dir.
	skutil.LogErr(util.CreateTimestampFile(pathToSkps))

	// Upload SKPs dir to Google Storage.
	if err := gs.UploadWorkerArtifacts(util.SKPS_DIR_NAME, filepath.Join(*pagesetType, *chromiumBuild), *workerNum); err != nil {
		glog.Error(err)
		return
	}
}

// SKPs are captured in parallel leading to multiple chrome instances coming up
// at the same time, when there are crashes chrome processes stick around which
// can severely impact the machine's performance. To stop this from
// happening chrome zombie processes are periodically killed.
func chromeProcessesCleaner(mutex *sync.RWMutex) {
	for _ = range time.Tick(*chromeCleanerTimer) {
		glog.Info("The chromeProcessesCleaner goroutine has started")
		glog.Info("Waiting for all existing tasks to complete before killing zombie chrome processes")
		mutex.Lock()
		skutil.LogErr(util.ExecuteCmd("pkill", []string{"-9", "chrome"}, []string{}, 5*time.Minute, nil, nil))
		mutex.Unlock()
	}
}

func getClosedChannelOfPagesets(fileInfos []os.FileInfo) chan string {
	pagesetsChannel := make(chan string, len(fileInfos))
	for i := 0; i < len(fileInfos); i++ {
		pagesetName := fileInfos[i].Name()
		pagesetBaseName := filepath.Base(pagesetName)
		if pagesetBaseName == util.TIMESTAMP_FILE_NAME || filepath.Ext(pagesetBaseName) == ".pyc" {
			// Ignore timestamp files and .pyc files.
			continue
		}
		pagesetsChannel <- pagesetName
	}
	close(pagesetsChannel)
	return pagesetsChannel
}

func getRowsFromCSV(csvPath string) ([]string, []string, error) {
	csvFile, err := os.Open(csvPath)
	defer skutil.Close(csvFile)
	if err != nil {
		return nil, nil, fmt.Errorf("Could not open %s: %s", csvPath, err)
	}
	reader := csv.NewReader(csvFile)
	reader.FieldsPerRecord = -1
	rawCSVdata, err := reader.ReadAll()
	if err != nil {
		return nil, nil, fmt.Errorf("Could not read %s: %s", csvPath, err)
	}
	if len(rawCSVdata) != 2 {
		return nil, nil, fmt.Errorf("No data in %s", csvPath)
	}
	return rawCSVdata[0], rawCSVdata[1], nil
}

func writeRowsToCSV(csvPath string, headers, values []string) error {
	csvFile, err := os.OpenFile(csvPath, os.O_WRONLY, 666)
	defer skutil.Close(csvFile)
	if err != nil {
		return fmt.Errorf("Could not open %s: %s", csvPath, err)
	}
	writer := csv.NewWriter(csvFile)
	defer writer.Flush()
	for _, row := range [][]string{headers, values} {
		if err := writer.Write(row); err != nil {
			return fmt.Errorf("Could not write to %s: %s", csvPath, err)
		}
	}
	return nil
}
