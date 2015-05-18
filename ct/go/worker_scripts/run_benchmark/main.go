// Application that runs the specified benchmark over CT's webpage archives.
package main

import (
	"encoding/csv"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/skia-dev/glog"

	"strings"

	"go.skia.org/infra/ct/go/adb"
	"go.skia.org/infra/ct/go/util"
	"go.skia.org/infra/go/common"
	skutil "go.skia.org/infra/go/util"
)

var (
	workerNum          = flag.Int("worker_num", 1, "The number of this CT worker. It will be in the {1..100} range.")
	pagesetType        = flag.String("pageset_type", util.PAGESET_TYPE_MOBILE_10k, "The type of pagesets to create from the Alexa CSV list. Eg: 10k, Mobile10k, All.")
	chromiumBuild      = flag.String("chromium_build", "", "The chromium build that was used to create the SKPs we would like to run lua scripts against.")
	runID              = flag.String("run_id", "", "The unique run id (typically requester + timestamp).")
	benchmarkName      = flag.String("benchmark_name", "", "The telemetry benchmark to run on this worker.")
	benchmarkExtraArgs = flag.String("benchmark_extra_args", "", "The extra arguments that are passed to the specified benchmark.")
	browserExtraArgs   = flag.String("browser_extra_args", "", "The extra arguments that are passed to the browser while running the benchmark.")
	repeatBenchmark    = flag.Int("repeat_benchmark", 3, "The number of times the benchmark should be repeated. For skpicture_printer benchmark this value is always 1.")
	targetPlatform     = flag.String("target_platform", util.PLATFORM_ANDROID, "The platform the benchmark will run on (Android / Linux).")
)

func main() {
	common.Init()
	defer util.CleanTmpDir()
	defer util.TimeTrack(time.Now(), "Running Benchmark")
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
	if *benchmarkName == "" {
		glog.Error("Must specify --benchmark_name")
		return
	}
	benchmarkArgs := *benchmarkExtraArgs
	browserArgs := *browserExtraArgs
	repeats := *repeatBenchmark

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
	skutil.LogErr(util.CreateTaskFile(util.ACTIVITY_RUNNING_BENCHMARKS))
	defer util.DeleteTaskFile(util.ACTIVITY_RUNNING_BENCHMARKS)

	if *targetPlatform == util.PLATFORM_ANDROID {
		if err := adb.VerifyLocalDevice(); err != nil {
			// Android device missing or offline.
			glog.Errorf("Could not find Android device: %s", err)
			return
		}
		// Make sure adb shell is running as root.
		skutil.LogErr(
			util.ExecuteCmd(util.BINARY_ADB, []string{"root"}, []string{}, 5*time.Minute, nil, nil))
	}

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

	// Special handling for the "skpicture_printer" benchmark. Need to create the
	// dir that SKPs will be stored in.
	pathToSkps := filepath.Join(util.SkpsDir, *pagesetType, *chromiumBuild)
	if *benchmarkName == util.BENCHMARK_SKPICTURE_PRINTER {
		// Delete and remake the local SKPs directory.
		skutil.RemoveAll(pathToSkps)
		skutil.MkdirAll(pathToSkps, 0700)
		// Tell skpicture_printer where to output SKPs.
		// Do not allow unneeded whitespace for benchmarkArgs since they are
		// directly passed to run_benchmarks.
		if benchmarkArgs != "" {
			benchmarkArgs += " "
		}
		benchmarkArgs += "--skp-outdir=" + pathToSkps
		// Only do one run for SKPs.
		repeats = 1
	}
	// Special handling for the "smoothness" benchmark.
	if *benchmarkName == util.BENCHMARK_SMOOTHNESS {
		// A synthetic scroll needs to be able to output at least two frames. Make
		// the viewport size smaller than the page size.
		// TODO(rmistry): I dont think smoothness honors the below flag, fix this
		// in telemetry code.
		browserArgs += " --window-size=1280,512"
	}

	// Establish output paths.
	localOutputDir := filepath.Join(util.StorageDir, util.BenchmarkRunsDir, *runID)
	skutil.RemoveAll(localOutputDir)
	skutil.MkdirAll(localOutputDir, 0700)
	defer skutil.RemoveAll(localOutputDir)
	remoteDir := filepath.Join(util.BenchmarkRunsDir, *runID)

	// Construct path to the ct_run_benchmark python script.
	_, currentFile, _, _ := runtime.Caller(0)
	pathToPyFiles := filepath.Join(
		filepath.Dir((filepath.Dir(filepath.Dir(filepath.Dir(currentFile))))),
		"py")

	timeoutSecs := util.PagesetTypeToInfo[*pagesetType].RunBenchmarksTimeoutSecs
	fileInfos, err := ioutil.ReadDir(pathToPagesets)
	if err != nil {
		glog.Errorf("Unable to read the pagesets dir %s: %s", pathToPagesets, err)
		return
	}
	// Loop through all pagesets.
	for _, fileInfo := range fileInfos {
		pagesetBaseName := filepath.Base(fileInfo.Name())
		if pagesetBaseName == util.TIMESTAMP_FILE_NAME || filepath.Ext(pagesetBaseName) == ".pyc" {
			// Ignore timestamp files and .pyc files.
			continue
		}

		// Convert the filename into a format consumable by the run_benchmarks
		// binary.
		pagesetName := strings.TrimSuffix(pagesetBaseName, filepath.Ext(pagesetBaseName))
		pagesetPath := filepath.Join(pathToPagesets, fileInfo.Name())

		glog.Infof("===== Processing %s =====", pagesetPath)

		skutil.LogErr(os.Chdir(pathToPyFiles))
		args := []string{
			util.BINARY_RUN_BENCHMARK,
			fmt.Sprintf("%s.%s", *benchmarkName, util.BenchmarksToPagesetName[*benchmarkName]),
			"--page-set-name=" + pagesetName,
			"--page-set-base-dir=" + pathToPagesets,
			"--also-run-disabled-tests",
		}
		// Need to capture output for all benchmarks except skpicture_printer.
		if *benchmarkName != util.BENCHMARK_SKPICTURE_PRINTER {
			outputDirArgValue := filepath.Join(localOutputDir, pagesetName)
			args = append(args, "--output-dir="+outputDirArgValue)
		}
		// Figure out which browser should be used.
		if *targetPlatform == util.PLATFORM_ANDROID {
			args = append(args, "--browser=android-chrome-shell")
		} else {
			args = append(args, "--browser=exact", "--browser-executable="+chromiumBinary)
		}
		// Split benchmark args if not empty and append to args.
		if benchmarkArgs != "" {
			for _, benchmarkArg := range strings.Split(benchmarkArgs, " ") {
				args = append(args, benchmarkArg)
			}
		}
		// Add the number of times to repeat.
		args = append(args, fmt.Sprintf("--page-repeat=%d", repeats))
		// Add browserArgs if not empty to args.
		if browserArgs != "" {
			args = append(args, "--extra-browser-args="+browserArgs)
		}
		// Set the PYTHONPATH to the pagesets and the telemetry dirs.
		env := []string{
			fmt.Sprintf("PYTHONPATH=%s:%s:%s:$PYTHONPATH", pathToPagesets, util.TelemetryBinariesDir, util.TelemetrySrcDir),
			"DISPLAY=:0",
		}
		skutil.LogErr(
			util.ExecuteCmd("python", args, env, time.Duration(timeoutSecs)*time.Second, nil, nil))
	}

	// If "--output-format=csv" was specified then merge all CSV files and upload.
	if strings.Contains(benchmarkArgs, "--output-format=csv") {
		// Move all results into a single directory.
		fileInfos, err := ioutil.ReadDir(localOutputDir)
		if err != nil {
			glog.Errorf("Unable to read %s: %s", localOutputDir, err)
			return
		}
		for _, fileInfo := range fileInfos {
			if !fileInfo.IsDir() {
				continue
			}
			outputFile := filepath.Join(localOutputDir, fileInfo.Name(), "results.csv")
			newFile := filepath.Join(localOutputDir, fmt.Sprintf("%s.csv", fileInfo.Name()))
			if err := os.Rename(outputFile, newFile); err != nil {
				glog.Errorf("Could not rename %s to %s: %s", outputFile, newFile, err)
				continue
			}
			// Add the rank of the page to the CSV file.
			headers, values, err := getRowsFromCSV(newFile)
			if err != nil {
				glog.Errorf("Could not read %s: %s", newFile, err)
				continue
			}
			pageRank := strings.Split(fileInfo.Name(), "_")[1]
			for i := range headers {
				if headers[i] == "page_name" {
					values[i] = fmt.Sprintf("%s (#%s)", values[i], pageRank)
				}
			}
			if err := writeRowsToCSV(newFile, headers, values); err != nil {
				glog.Errorf("Could not write to %s: %s", newFile, err)
				continue
			}
		}
		// Call csv_merger.py to merge all results into a single results CSV.
		pathToCsvMerger := filepath.Join(pathToPyFiles, "csv_merger.py")
		outputFileName := *runID + ".output"
		args := []string{
			pathToCsvMerger,
			"--csv_dir=" + localOutputDir,
			"--output_csv_name=" + filepath.Join(localOutputDir, outputFileName),
		}
		if err := util.ExecuteCmd("python", args, []string{}, 10*time.Minute, nil, nil); err != nil {
			glog.Errorf("Error running csv_merger.py: %s", err)
			return
		}
		// Copy the output file to Google Storage.
		remoteOutputDir := filepath.Join(remoteDir, fmt.Sprintf("slave%d", *workerNum), "outputs")
		if err := gs.UploadFile(outputFileName, localOutputDir, remoteOutputDir); err != nil {
			glog.Errorf("Unable to upload %s to %s: %s", outputFileName, remoteOutputDir, err)
			return
		}
	}

	// Move, validate and upload all SKP files if skpicture_printer was used.
	if *benchmarkName == util.BENCHMARK_SKPICTURE_PRINTER {
		// List all directories in pathToSkps and copy out the skps.
		fileInfos, err := ioutil.ReadDir(pathToSkps)
		if err != nil {
			glog.Errorf("Unable to read %s: %s", pathToSkps, err)
			return
		}
		for _, fileInfo := range fileInfos {
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
		skutil.LogErr(util.ExecuteCmd("python", args, []string{}, 10*time.Minute, nil, nil))

		// Write timestamp to the SKPs dir.
		skutil.LogErr(util.CreateTimestampFile(pathToSkps))

		// Upload SKPs dir to Google Storage.
		if err := gs.UploadWorkerArtifacts(util.SKPS_DIR_NAME, filepath.Join(*pagesetType, *chromiumBuild), *workerNum); err != nil {
			glog.Error(err)
			return
		}
	}
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
