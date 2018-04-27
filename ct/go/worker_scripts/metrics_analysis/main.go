// Application that does metrics analysis as described in the design doc:
// go/ct_metrics_analysis
package main

import (
	"context"
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

	"go.skia.org/infra/go/sklog"

	"go.skia.org/infra/ct/go/util"
	"go.skia.org/infra/ct/go/worker_scripts/worker_common"
	"go.skia.org/infra/go/common"
	skutil "go.skia.org/infra/go/util"
)

const (
	// The number of goroutines that will run in parallel to run metrics analysis.
	WORKER_POOL_SIZE = 10

	METRICS_BENCHMARK_TIMEOUT_SECS = 300
)

var (
	startRange         = flag.Int("start_range", 1, "The number this worker will run metrics analysis from.")
	num                = flag.Int("num", 100, "The total number of traces to run metrics analysis from the start_range.")
	runID              = flag.String("run_id", "", "The unique run id (typically requester + timestamp).")
	benchmarkExtraArgs = flag.String("benchmark_extra_args", "", "The extra arguments that are passed to the specified benchmark.")
)

func metricsAnalysis() error {
	defer common.LogPanic()
	worker_common.Init()
	if !*worker_common.Local {
		defer util.CleanTmpDir()
	}
	defer util.TimeTrack(time.Now(), "Metrics Analysis")
	defer sklog.Flush()

	// Validate required arguments.
	if *runID == "" {
		return errors.New("Must specify --run_id")
	}

	ctx := context.Background()

	// Instantiate GcsUtil object.
	gs, err := util.NewGcsUtil(nil)
	if err != nil {
		return fmt.Errorf("GcsUtil instantiation failed: %s", err)
	}

	// Download the trace URLs for this run from Google storage.
	tracesFilename := *runID + ".traces.csv"
	// TODO(rmistry): This should not be in the tmp dir, could run out of memory!
	// TODO(rmistry): Remove this directory later.
	tmpDir, err := ioutil.TempDir(util.PagesetsDir, "traces")
	defer skutil.RemoveAll(tmpDir)
	remotePatchesDir := filepath.Join(util.BenchmarkRunsDir, *runID)
	if err != nil {
		return fmt.Errorf("Could not create tmpdir: %s", err)
	}
	if _, err := util.DownloadPatch(filepath.Join(tmpDir, tracesFilename), filepath.Join(remotePatchesDir, tracesFilename), gs); err != nil {
		return fmt.Errorf("Could not download %s: %s", tracesFilename, err)
	}
	traces, err := util.GetCustomPages(filepath.Join(tmpDir, tracesFilename))
	if err != nil {
		return fmt.Errorf("Could not read custom traces file %s: %s", tracesFilename, err)
	}
	if len(traces) == 0 {
		return errors.New(fmt.Sprintf("No traces found in %s", tracesFilename))
	}
	traces = util.GetCustomPagesWithinRange(*startRange, *num, traces)
	sklog.Info("Using %d traces", len(traces))

	// Establish nopatch output paths.
	localOutputDir := filepath.Join(util.StorageDir, util.BenchmarkRunsDir, *runID)
	skutil.RemoveAll(localOutputDir)
	skutil.MkdirAll(localOutputDir, 0700)
	defer skutil.RemoveAll(localOutputDir)
	remoteDir := filepath.Join(util.BenchmarkRunsDir, *runID)

	sklog.Infof("===== Going to run the task with %d parallel chrome processes =====", WORKER_POOL_SIZE)
	// Create channel that contains all trace ULs. This channel will
	// be consumed by the worker pool.
	traceRequests := getClosedChannelOfTraces(traces)

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

			for t := range traceRequests {
				sklog.Infof("===== Processing %s =====", t)
				runMetricsAnalysisBenchmark(ctx, localOutputDirNoPatch, chromiumBinaryNoPatch, pagesetName, pathToPagesets, decodedPageset, timeoutSecs, rank)
			}
		}()
	}

	// Wait for all spawned goroutines to complete.
	wg.Wait()

	// If "--output-format=csv" was specified then merge all CSV files and upload.
	if strings.Contains(*benchmarkExtraArgs, "--output-format=csv") {
		// Can map[string]map[string]string{} be nil instead?
		if err := util.MergeUploadCSVFilesOnWorkers(ctx, localOutputDir, pathToPyFiles, *runID, remoteDir, gs, *startRange, true /* handleStrings */, map[string]map[string]string{} /* pageRankToAdditionalFields */); err != nil {
			return fmt.Errorf("Error while processing withpatch CSV files: %s", err)
		}
	}

	return nil
}

func runScreenshotBenchmark(ctx context.Context, outputPath, chromiumBinary, pagesetName, pathToPagesets string, decodedPageset util.PagesetVars, timeoutSecs, rank int) {

	args := []string{
		filepath.Join(util.TelemetryBinariesDir, util.BINARY_RUN_BENCHMARK),
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
	// Set the PYTHONPATH to the pagesets and the telemetry dirs.
	env := []string{
		fmt.Sprintf("PYTHONPATH=%s:%s:%s:%s:$PYTHONPATH", pathToPagesets, util.TelemetryBinariesDir, util.TelemetrySrcDir, util.CatapultSrcDir),
		"DISPLAY=:0",
	}
	// Append the original environment as well.
	for _, e := range os.Environ() {
		env = append(env, e)
	}

	// Execute run_benchmark and log if there are any errors.
	err := util.ExecuteCmd(ctx, "python", args, env, time.Duration(timeoutSecs)*time.Second, nil, nil)
	if err != nil {
		sklog.Errorf("Error during run_benchmark: %s", err)
	}
}

// Returns channel that contains all trace URLs.
func getClosedChannelOfTraces(traces []string) chan string {
	tracesChannel := make(chan string, len(traces))
	for _, t := range traces {
		tracesChannel <- t
	}
	close(tracesChannel)
	return tracesChannel
}

func main() {
	retCode := 0
	if err := metricsAnalysis(); err != nil {
		sklog.Errorf("Error while running metrics analysis: %s", err)
		retCode = 255
	}
	os.Exit(retCode)
}
