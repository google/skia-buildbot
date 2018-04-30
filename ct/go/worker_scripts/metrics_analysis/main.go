// Application that does metrics analysis as described in the design doc:
// go/ct_metrics_analysis
//
// Can be run locally with:
//
//
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"go.skia.org/infra/go/sklog"

	"go.skia.org/infra/ct/go/util"
	"go.skia.org/infra/ct/go/worker_scripts/worker_common"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/httputils"
	skutil "go.skia.org/infra/go/util"
)

const (
	// The number of goroutines that will run in parallel to download traces and
	// run metrics analysis.
	WORKER_POOL_SIZE = 20

	METRICS_BENCHMARK_TIMEOUT_SECS = 300
)

var (
	startRange         = flag.Int("start_range", 1, "The number this worker will run metrics analysis from.")
	num                = flag.Int("num", 100, "The total number of traces to run metrics analysis from the start_range.")
	runID              = flag.String("run_id", "", "The unique run id (typically requester + timestamp).")
	benchmarkExtraArgs = flag.String("benchmark_extra_args", "", "The extra arguments that are passed to the specified benchmark.")
	metricName         = flag.String("metric_name", "", "The metric to parse the traces with.")
)

// TODO(rmistry): Make sure all downloaded artifacts and directories are cleaned up at the end of the run.
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
	if *metricName == "" {
		return errors.New("Must specify --metric_name")
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

	// Establish output paths for pdf downloads and metrics.
	traceDownloadDir := filepath.Join(util.StorageDir, util.TraceDownloadsDir, *runID)
	skutil.RemoveAll(traceDownloadDir)
	skutil.MkdirAll(traceDownloadDir, 0700)
	defer skutil.RemoveAll(traceDownloadDir)

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

	// Gather traceURLs with errors.
	erroredTraces := []string{}
	// Mutex to control access to the above.
	var erroredTracesMutex sync.Mutex

	// Loop through workers in the worker pool.
	for i := 0; i < WORKER_POOL_SIZE; i++ {
		// Increment the WaitGroup counter.
		wg.Add(1)

		// Create and run a goroutine closure that captures screenshots.
		go func() {
			// Decrement the WaitGroup counter when the goroutine completes.
			defer wg.Done()

			// Instantiate timeout client for downloading traces.
			transport := &http.Transport{
				Dial: httputils.DialTimeout,
			}
			httpTimeoutClient := &http.Client{
				Transport: transport,
				Timeout:   httputils.REQUEST_TIMEOUT,
			}

			for t := range traceRequests {
				sklog.Infof("===== Downloading trace %s =====", t)
				if err := downloadTrace(t, traceDownloadDir, httpTimeoutClient); err != nil {
					sklog.Errorf("Could not download %s: %s", t, err)
					erroredTracesMutex.Lock()
					erroredTraces = append(erroredTraces, t)
					erroredTracesMutex.Unlock()
					continue
				}

				// By default, transport caches connections for future re-use.
				// This may leave many open connections when accessing many hosts.
				transport.CloseIdleConnections()

				sklog.Infof("===== Processing %s =====", t)
				runMetricsAnalysisBenchmark(ctx, localOutputDir, t)
			}
		}()
	}

	// Wait for all spawned goroutines to complete.
	wg.Wait()

	// Summarize errors.
	if len(erroredTraces) > 0 {
		sklog.Error("The following traces could not be downloaded:")
		for _, erroredTrace := range erroredTraces {
			sklog.Errorf("\t%s", erroredTrace)
		}
	}

	// If "--output-format=csv" was specified then merge all CSV files and upload.
	if strings.Contains(*benchmarkExtraArgs, "--output-format=csv") {
		// Construct path to CT's python scripts.
		pathToPyFiles := util.GetPathToPyFiles(!*worker_common.Local)
		// TODO(rmistry): Can map[string]map[string]string{} be nil instead?
		if err := util.MergeUploadCSVFilesOnWorkers(ctx, localOutputDir, pathToPyFiles, *runID, remoteDir, gs, *startRange, true /* handleStrings */, map[string]map[string]string{} /* pageRankToAdditionalFields */); err != nil {
			return fmt.Errorf("Error while processing withpatch CSV files: %s", err)
		}
	}

	return nil
}

func runMetricsAnalysisBenchmark(ctx context.Context, outputPath, tracePath string) {
	args := []string{
		filepath.Join(util.TelemetryBinariesDir, util.BINARY_RUN_BENCHMARK),
		util.BENCHMARK_METRICS_ANALYSIS,
		"--urls-list=" + tracePath,
		"--metric-name=" + *metricName,
		"--browser=system", // No browser is brought up but unfortunately this is needed by the framework.
	}
	// Split benchmark args if not empty and append to args.
	if *benchmarkExtraArgs != "" {
		args = append(args, strings.Fields(*benchmarkExtraArgs)...)
	}
	// Set the PYTHONPATH to the pagesets and the telemetry dirs.
	env := []string{
		// Removed pathToPagesets below.
		fmt.Sprintf("PYTHONPATH=%s:%s:%s:$PYTHONPATH", util.TelemetryBinariesDir, util.TelemetrySrcDir, util.CatapultSrcDir),
		// "DISPLAY=:0", // not needed since no browser
	}
	// Append the original environment as well.
	for _, e := range os.Environ() {
		env = append(env, e)
	}

	// Execute run_benchmark and log if there are any errors.
	err := util.ExecuteCmd(ctx, "python", args, env, METRICS_BENCHMARK_TIMEOUT_SECS*time.Second, nil, nil)
	if err != nil {
		sklog.Errorf("Error during run_benchmark: %s", err)
	}
}

func downloadTrace(traceURL, destDir string, httpTimeoutClient *http.Client) error {
	traceTokens := strings.Split(traceURL, "/")
	tracePath := filepath.Join(destDir, traceTokens[len(traceTokens)-1])
	resp, err := httpTimeoutClient.Get(traceURL)
	if err != nil {
		return fmt.Errorf("Could not GET %s: %s", traceURL, err)
	}
	defer skutil.Close(resp.Body)
	out, err := os.Create(tracePath)
	defer skutil.Close(out)
	if err != nil {
		return fmt.Errorf("Unable to create file %s: %s", tracePath, err)
	}
	if _, err = io.Copy(out, resp.Body); err != nil {
		return fmt.Errorf("Unable to write to file %s: %s", tracePath, err)
	}
	return nil
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
