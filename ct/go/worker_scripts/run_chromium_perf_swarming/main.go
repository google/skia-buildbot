// run_chromium_perf_swarming is an application that is meant to be run on
// a swarming slave. It runs the specified benchmark over CT's webpage
// archives and uploads results to chromeperf.appspot.com
package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"github.com/skia-dev/glog"

	"strings"

	"go.skia.org/infra/ct/go/util"
	"go.skia.org/infra/ct/go/worker_scripts/worker_common"
	"go.skia.org/infra/go/common"
	skutil "go.skia.org/infra/go/util"
)

var (
	workerNum            = flag.Int("worker_num", 1, "The number of this CT worker. It will be in the {1..100} range.")
	chromiumBuild        = flag.String("chromium_build", "", "The chromium build dir to use.")
	benchmarkName        = flag.String("benchmark_name", "", "The telemetry benchmark to run on this worker.")
	benchmarkExtraArgs   = flag.String("benchmark_extra_args", "", "The extra arguments that are passed to the specified benchmark.")
	browserExtraArgs     = flag.String("browser_extra_args", "--disable-setuid-sandbox --enable-threaded-compositing --enable-impl-side-painting", "The extra arguments that are passed to the browser while running the benchmark.")
	repeatBenchmark      = flag.Int("repeat_benchmark", 2, "The number of times the benchmark should be repeated. For skpicture_printer benchmark this value is always 1.")
	targetPlatform       = flag.String("target_platform", util.PLATFORM_LINUX, "The platform the benchmark will run on (Android / Linux).")
	telemetryBinariesDir = flag.String("telemetry_binaries_dir", "", "The directory that contains the telemetry binaries.")
	pageSetsDir          = flag.String("page_sets_dir", "", "The directory that contains the CT page sets.")
	outDir               = flag.String("out_dir", "", "The directory to send results to.")
)

func main() {
	defer common.LogPanic()
	worker_common.Init()

	defer util.TimeTrack(time.Now(), "Running Chromium Perf on Swarming")
	defer glog.Flush()

	// Validate required arguments.
	if *chromiumBuild == "" {
		glog.Error("Must specify --chromium_build")
		return
	}
	if *benchmarkName == "" {
		glog.Error("Must specify --benchmark_name")
		return
	}
	if *telemetryBinariesDir == "" {
		glog.Error("Must specify --telemetry_binaries_dir")
		return
	}
	if *pageSetsDir == "" {
		glog.Error("Must specify --page_sets_dir")
		return
	}
	if *outDir == "" {
		glog.Error("Must specify --out_dir")
		return
	}
	chromiumBinary := filepath.Join(*chromiumBuild, util.BINARY_CHROME)

	// Establish output paths.
	localOutputDir := util.BenchmarkRunsDir
	skutil.MkdirAll(localOutputDir, 0700)

	fileInfos, err := ioutil.ReadDir(*pageSetsDir)
	if err != nil {
		glog.Errorf("Unable to read the pagesets dir %s: %s", *pageSetsDir, err)
		return
	}

	for _, fileInfo := range fileInfos {
		if fileInfo.IsDir() {
			continue
		}
		if err := runBenchmark(fileInfo.Name(), *pageSetsDir, localOutputDir, *chromiumBuild, chromiumBinary, *browserExtraArgs); err != nil {
			glog.Errorf("Error while running benchmark: %s", err)
			return
		}
	}

	// Convert output to dashboard JSON v1 in order to upload to chromeperf.
	// More information is in http://www.chromium.org/developers/speed-infra/performance-dashboard/sending-data-to-the-performance-dashboard
	outputFileInfos, err := ioutil.ReadDir(localOutputDir)
	if err != nil {
		glog.Errorf("Unable to read %s: %s", localOutputDir, err)
		return
	}
	for _, fileInfo := range outputFileInfos {
		if !fileInfo.IsDir() {
			continue
		}
		resultsFile := filepath.Join(localOutputDir, fileInfo.Name(), "results-chart.json")
		outputFile := filepath.Join(*outDir, fmt.Sprintf("%s-results.json", fileInfo.Name()))
		if err := os.Rename(resultsFile, outputFile); err != nil {
			glog.Errorf("Could not rename %s to %s: %s", resultsFile, outputFile, err)
			continue
		}
	}
}

func runBenchmark(fileInfoName, pathToPagesets, localOutputDir, chromiumBuildName, chromiumBinary, browserExtraArgs string) error {
	pagesetBaseName := filepath.Base(fileInfoName)
	if pagesetBaseName == util.TIMESTAMP_FILE_NAME || filepath.Ext(pagesetBaseName) == ".pyc" {
		// Ignore timestamp files and .pyc files.
		return nil
	}

	// Read the pageset.
	pagesetName := strings.TrimSuffix(pagesetBaseName, filepath.Ext(pagesetBaseName))
	pagesetPath := filepath.Join(pathToPagesets, fileInfoName)
	decodedPageset, err := util.ReadPageset(pagesetPath)
	if err != nil {
		return fmt.Errorf("Could not read %s: %s", pagesetPath, err)
	}

	glog.Infof("===== Processing %s =====", pagesetPath)
	benchmark, present := util.BenchmarksToTelemetryName[*benchmarkName]
	if !present {
		// If it is custom benchmark use the entered benchmark name.
		benchmark = *benchmarkName
	}
	args := []string{
		filepath.Join(*telemetryBinariesDir, util.BINARY_RUN_BENCHMARK),
		benchmark,
		"--also-run-disabled-tests",
		"--user-agent=" + decodedPageset.UserAgent,
		"--urls-list=" + decodedPageset.UrlsList,
		"--archive-data-file=" + filepath.Join(*pageSetsDir, decodedPageset.ArchiveDataFile),
		// Output in chartjson. Needed for uploading to performance dashboard.
		// Documentation is here: http://www.chromium.org/developers/speed-infra/performance-dashboard/sending-data-to-the-performance-dashboard
		"--output-format=chartjson",
		// Upload traces.
		"--upload-results",
		"--upload-bucket=output",
	}

	// Need to capture output for all benchmarks.
	outputDirArgValue := filepath.Join(localOutputDir, pagesetName)
	args = append(args, "--output-dir="+outputDirArgValue)
	// Figure out which browser should be used.
	if *targetPlatform == util.PLATFORM_ANDROID {
		if err := util.InstallChromeAPK(chromiumBuildName); err != nil {
			return fmt.Errorf("Error while installing APK: %s", err)
		}
		args = append(args, "--browser=android-chromium")
	} else {
		args = append(args, "--browser=exact", "--browser-executable="+chromiumBinary)
	}
	// Split benchmark args if not empty and append to args.
	if *benchmarkExtraArgs != "" {
		args = append(args, strings.Fields(*benchmarkExtraArgs)...)
	}
	// Add the number of times to repeat.
	args = append(args, fmt.Sprintf("--page-repeat=%d", *repeatBenchmark))
	// Add browserArgs if not empty to args.
	if browserExtraArgs != "" {
		args = append(args, "--extra-browser-args="+browserExtraArgs)
	}
	// Set the PYTHONPATH to the pagesets and the telemetry dirs.
	env := []string{
		fmt.Sprintf("PYTHONPATH=%s:$PYTHONPATH", *telemetryBinariesDir),
		"DISPLAY=:0",
	}
	timeoutSecs := 2 * 60 // 2 mins timeout
	if err := util.ExecuteCmd("python", args, env, time.Duration(timeoutSecs)*time.Second, nil, nil); err != nil {
		glog.Errorf("Run benchmark command failed with: %s", err)
	}
	return nil
}
