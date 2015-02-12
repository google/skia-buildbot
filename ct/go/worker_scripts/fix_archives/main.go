// fix_archives is an application that validates the webpage archives of a
// pageset type and deletes the archives which are found to be deliver
// inconsistent benchmark results.
// See https://code.google.com/p/chromium/issues/detail?id=456883 for more
// details.
package main

import (
	"encoding/csv"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"

	"github.com/skia-dev/glog"

	"strconv"
	"strings"
	"time"

	"skia.googlesource.com/buildbot.git/ct/go/util"
	"skia.googlesource.com/buildbot.git/go/common"
)

var (
	runID                  = flag.String("run_id", "", "The unique run id (typically requester + timestamp).")
	workerNum              = flag.Int("worker_num", 1, "The number of this CT worker. It will be in the {1..100} range.")
	pagesetType            = flag.String("pageset_type", util.PAGESET_TYPE_MOBILE_10k, "The type of pagesets whose archives need to be validated. Eg: 10k, Mobile10k, All.")
	chromiumBuild          = flag.String("chromium_build", "", "The chromium build to use for this validation run.")
	repeatBenchmark        = flag.Int("repeat_benchmark", 5, "The number of times the benchmark should be repeated.")
	benchmarkName          = flag.String("benchmark_name", util.BENCHMARK_REPAINT, "The telemetry benchmark to run on this worker.")
	benchmarkHeaderToCheck = flag.String("benchmark_header_to_check", "mean_frame_time (ms)", "The benchmark header this task will validate.")
	benchmarkArgs          = flag.String("benchmark_args", "--output-format=csv", "The arguments that are passed to the specified benchmark.")
	browserArgs            = flag.String("browser_args", "--disable-setuid-sandbox --enable-threaded-compositing --enable-impl-side-painting", "The arguments that are passed to the browser while running the benchmark.")
)

func main() {
	common.Init()
	defer util.TimeTrack(time.Now(), "Fixing archives")
	defer glog.Flush()

	// Create the task file so that the master knows this worker is still busy.
	util.CreateTaskFile(util.ACTIVITY_FIXING_ARCHIVES)
	defer util.DeleteTaskFile(util.ACTIVITY_FIXING_ARCHIVES)

	if *pagesetType == "" {
		glog.Error("Must specify --pageset_type")
		return
	}
	if *chromiumBuild == "" {
		glog.Error("Must specify --chromium_build")
		return
	}
	if *runID == "" {
		glog.Error("Must specify --run_id")
		return
	}

	// Sync the local chromium checkout.
	if err := util.SyncDir(util.ChromiumSrcDir); err != nil {
		glog.Errorf("Could not gclient sync %s: %s", util.ChromiumSrcDir, err)
		return
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
	defer os.RemoveAll(filepath.Join(util.ChromiumBuildsDir, *chromiumBuild))
	chromiumBinary := filepath.Join(util.ChromiumBuildsDir, *chromiumBuild, util.BINARY_CHROME)

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

	// Establish output paths.
	localOutputDir := filepath.Join(util.StorageDir, util.FixArchivesRunsDir, *runID)
	os.RemoveAll(localOutputDir)
	os.MkdirAll(localOutputDir, 0700)
	defer os.RemoveAll(localOutputDir)

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

	// Location of the WPR logs.
	wprLogs := filepath.Join(util.ChromiumSrcDir, "webpagereplay_logs", "logs.txt")
	// Slice that will contain
	inconsistentArchives := []string{}

	// Loop through all pagesets.
	for _, fileInfo := range fileInfos {
		benchmarkResults := []float64{}
		resourceMissingCounts := []int{}
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

		// Repeat runs the specified number of times.
		for repeatNum := 1; repeatNum <= *repeatBenchmark; repeatNum++ {
			// Delete webpagereplay_logs before every run.
			os.RemoveAll(wprLogs)

			os.Chdir(pathToPyFiles)
			args := []string{
				util.BINARY_RUN_BENCHMARK,
				fmt.Sprintf("%s.%s", *benchmarkName, util.BenchmarksToPagesetName[*benchmarkName]),
				"--page-set-name=" + pagesetName,
				"--page-set-base-dir=" + pathToPagesets,
				"--also-run-disabled-tests",
			}
			// Add output dir.
			outputDirArgValue := filepath.Join(localOutputDir, pagesetName, strconv.Itoa(repeatNum))
			args = append(args, "--output-dir="+outputDirArgValue)
			// Figure out which browser should be used.
			args = append(args, "--browser=exact", "--browser-executable="+chromiumBinary)
			// Split benchmark args if not empty and append to args.
			if *benchmarkArgs != "" {
				for _, benchmarkArg := range strings.Split(*benchmarkArgs, " ") {
					args = append(args, benchmarkArg)
				}
			}
			// Add browserArgs if not empty to args.
			if *browserArgs != "" {
				args = append(args, "--extra-browser-args="+*browserArgs)
			}
			// Set the PYTHONPATH to the pagesets and the telemetry dirs.
			env := []string{
				fmt.Sprintf("PYTHONPATH=%s:%s:%s:$PYTHONPATH", pathToPagesets, util.TelemetryBinariesDir, util.TelemetrySrcDir),
				"DISPLAY=:0",
			}
			util.ExecuteCmd("python", args, env, time.Duration(timeoutSecs)*time.Second, nil, nil)

			// Examine the results.csv file and store the mean frame time.
			resultsCSV := filepath.Join(outputDirArgValue, "results.csv")
			headers, values, err := getRowsFromCSV(resultsCSV)
			if err != nil {
				glog.Errorf("Could not read %s: %s", resultsCSV, err)
				continue
			}
			for i := range headers {
				if headers[i] == *benchmarkHeaderToCheck {
					value, _ := strconv.ParseFloat(values[i], 64)
					benchmarkResults = append(benchmarkResults, value)
					break
				}
			}

			// Find how many times "Could not replay" showed up in wprLogs.
			content, err := ioutil.ReadFile(wprLogs)
			if err != nil {
				glog.Errorf("Could not read %s: %s", wprLogs, err)
				continue
			}
			resourceMissingCount := strings.Count(string(content), "Could not replay")
			resourceMissingCounts = append(resourceMissingCounts, resourceMissingCount)
		}

		glog.Infof("Benchmark results for %s are: %v", fileInfo.Name(), benchmarkResults)
		percentageChange := getPercentageChange(benchmarkResults)
		glog.Infof("Percentage change of results is: %f", percentageChange)
		glog.Infof("\"Could not replay\" showed up %v times in %s", resourceMissingCounts, wprLogs)
		maxResourceMissingCount := 0
		for _, count := range resourceMissingCounts {
			if maxResourceMissingCount < count {
				maxResourceMissingCount = count
			}
		}
		if percentageChange > 10 || maxResourceMissingCount > 10 {
			glog.Infof("The archive for %s is inconsistent!", fileInfo.Name())
			inconsistentArchives = append(inconsistentArchives, fileInfo.Name())
		}
	}

	if len(inconsistentArchives) > 0 {
		glog.Infof("%d archives are inconsistent!", len(inconsistentArchives))
		glog.Infof("The list of inconsistentArchives is: %v", inconsistentArchives)
		// TODO(rmistry): If this script appears to be reliable then the page sets
		// should be deleted here.
	}
}

func getPercentageChange(values []float64) float64 {
	smallest := values[0]
	largest := values[0]
	for _, value := range values {
		if smallest > value {
			smallest = value
		}
		if largest < value {
			largest = value
		}
	}
	diff := largest - smallest
	if smallest == 0 {
		return 0
	}
	return diff / smallest * 100
}

func getRowsFromCSV(csvPath string) ([]string, []string, error) {
	csvFile, err := os.Open(csvPath)
	defer csvFile.Close()
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
