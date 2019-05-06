// Application that does metrics analysis as described in the design doc:
// go/ct_metrics_analysis
//
// Can be tested locally with:
// $ go run go/worker_scripts/metrics_analysis/main.go --start_range=1 --num=3 --run_id=rmistry-test1 --benchmark_extra_args="--output-format=csv" --metric_name="loadingMetric" --logtostderr=true --local
//
package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"go.skia.org/infra/ct/go/util"
	"go.skia.org/infra/ct/go/worker_scripts/worker_common"
	"go.skia.org/infra/go/sklog"
	skutil "go.skia.org/infra/go/util"
)

const (
	// The number of goroutines that will run in parallel to download traces and
	// run metrics analysis.
	WORKER_POOL_SIZE = 5

	METRICS_BENCHMARK_TIMEOUT_SECS = 300

	DEFAULT_TRACE_OUTPUT_BUCKET = "chrome-telemetry-output"
)

var (
	startRange         = flag.Int("start_range", 1, "The number this worker will run metrics analysis from.")
	num                = flag.Int("num", 100, "The total number of traces to run metrics analysis from the start_range.")
	runID              = flag.String("run_id", "", "The unique run id (typically requester + timestamp).")
	benchmarkExtraArgs = flag.String("benchmark_extra_args", "", "The extra arguments that are passed to the specified benchmark.")
	metricName         = flag.String("metric_name", "", "The metric to parse the traces with. Eg: loadingMetric")
	valueColumnName    = flag.String("value_column_name", "", "Which column's entries to use as field values when combining CSVs.")

	cloudUrlBucketRegex = regexp.MustCompile(`/m/cloudstorage/b/(.+)/o/`)
)

func metricsAnalysis() error {
	ctx := context.Background()
	worker_common.Init(ctx)
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

	// Use defaults.
	if *valueColumnName == "" {
		*valueColumnName = util.DEFAULT_VALUE_COLUMN_NAME
	}

	// Instantiate GcsUtil object.
	gs, err := util.NewGcsUtil(nil)
	if err != nil {
		return fmt.Errorf("GcsUtil instantiation failed: %s", err)
	}

	// Download the trace URLs for this run from Google storage.
	tracesFilename := *runID + ".traces.csv"
	skutil.MkdirAll(util.PagesetsDir, 0700)
	tmpDir, err := ioutil.TempDir(util.PagesetsDir, "traces")
	if err != nil {
		return fmt.Errorf("Could not create tmpdir: %s", err)
	}
	defer skutil.RemoveAll(tmpDir)
	remotePatchesDir := filepath.Join(util.BenchmarkRunsDir, *runID)

	// Download traces.
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
	sklog.Infof("Using %d traces", len(traces))

	// Establish output paths for trace downloads and metrics.
	traceDownloadDir := filepath.Join(util.StorageDir, util.TraceDownloadsDir, *runID)
	skutil.RemoveAll(traceDownloadDir)
	skutil.MkdirAll(traceDownloadDir, 0700)
	defer skutil.RemoveAll(traceDownloadDir)

	localOutputDir := filepath.Join(util.StorageDir, util.BenchmarkRunsDir, *runID)
	skutil.RemoveAll(localOutputDir)
	skutil.MkdirAll(localOutputDir, 0700)
	defer skutil.RemoveAll(localOutputDir)
	remoteDir := filepath.Join(util.BenchmarkRunsDir, *runID)

	sklog.Infof("===== Going to run the task with %d parallel goroutines =====", WORKER_POOL_SIZE)
	// Create channel that contains all trace ULs. This channel will
	// be consumed by the worker pool.
	traceRequests := getClosedChannelOfTraces(traces)

	var wg sync.WaitGroup

	// If not a single benchmark run succeeds then throw at error at the end.
	atleastOneBenchmarkSucceeded := false
	// Gather traceURLs that could not be downloaded.
	erroredTraces := []string{}
	// Mutex to control access to the above slice.
	var erroredTracesMutex sync.Mutex

	// Loop through workers in the worker pool.
	for i := 0; i < WORKER_POOL_SIZE; i++ {
		// Increment the WaitGroup counter.
		wg.Add(1)

		// Create and run a goroutine closure that runs the analysis benchmark.
		go func() {
			// Decrement the WaitGroup counter when the goroutine completes.
			defer wg.Done()

			for t := range traceRequests {
				sklog.Infof("========== Downloading trace %s ==========", t)
				downloadedTrace, err := downloadTrace(t, traceDownloadDir, gs)
				if err != nil {
					sklog.Errorf("Could not download %s: %s", t, err)
					erroredTracesMutex.Lock()
					erroredTraces = append(erroredTraces, t)
					erroredTracesMutex.Unlock()
					continue
				}

				sklog.Infof("========== Processing %s ==========", t)
				if err := runMetricsAnalysisBenchmark(ctx, localOutputDir, downloadedTrace, t); err != nil {
					sklog.Errorf("Error during run_benchmark: %s", err)
				} else {
					atleastOneBenchmarkSucceeded = true
				}
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
	if !atleastOneBenchmarkSucceeded {
		return errors.New("Not a single benchmark run was successful. Something is wrong.")
	}

	// If "--output-format=csv" was specified then merge all CSV files and upload.
	if strings.Contains(*benchmarkExtraArgs, "--output-format=csv") {
		// Construct path to CT's python scripts.
		pathToPyFiles := util.GetPathToPyFiles(*worker_common.Local, false /* runOnMaster */)
		if err := util.MergeUploadCSVFilesOnWorkers(ctx, localOutputDir, pathToPyFiles, *runID, remoteDir, *valueColumnName, gs, *startRange, true /* handleStrings */, false /* addRanks */, map[string]map[string]string{} /* pageRankToAdditionalFields */); err != nil {
			return fmt.Errorf("Error while processing withpatch CSV files: %s", err)
		}
	}

	return nil
}

// runMetricsAnalysisBenchmark runs the analysis_metrics_ct benchmark on the provided trace.
func runMetricsAnalysisBenchmark(ctx context.Context, outputPath, downloadedTrace, cloudTraceLink string) error {
	outputCSVDir := filepath.Join(outputPath, getTraceName(downloadedTrace))
	if err := os.MkdirAll(outputCSVDir, 0700); err != nil {
		return fmt.Errorf("Could not create %s: %s", outputCSVDir, err)
	}

	args := []string{
		filepath.Join(util.GetPathToTelemetryCTBinaries(*worker_common.Local), util.BINARY_ANALYZE_METRICS),
		"--local-trace-path", downloadedTrace,
		"--cloud-trace-link", cloudTraceLink,
		"--metric-name", *metricName,
		"--output-csv", filepath.Join(outputCSVDir, "results.csv"),
	}
	// Calculate what timeout should be used when executing run_benchmark.
	timeoutSecs := util.GetRunBenchmarkTimeoutValue(*benchmarkExtraArgs, METRICS_BENCHMARK_TIMEOUT_SECS)
	sklog.Infof("Using %d seconds for timeout", timeoutSecs)
	// Set the DISPLAY.
	env := []string{
		"DISPLAY=:0",
	}
	// Append the original environment as well.
	for _, e := range os.Environ() {
		env = append(env, e)
	}

	// Create buffer for capturing the stdout and stderr of the benchmark run.
	var b bytes.Buffer
	if _, err := b.WriteString(fmt.Sprintf("========== Stdout and stderr for %s ==========\n", downloadedTrace)); err != nil {
		return fmt.Errorf("Error writing to output buffer: %s", err)
	}
	if err := util.ExecuteCmdWithConfigurableLogging(ctx, "python", args, env, time.Duration(timeoutSecs)*time.Second, &b, &b, false, false); err != nil {
		output, getErr := util.GetRunBenchmarkOutput(b)
		skutil.LogErr(getErr)
		fmt.Println(output)
		return fmt.Errorf("Run benchmark command failed with: %s", err)
	}
	output, err := util.GetRunBenchmarkOutput(b)
	if err != nil {
		return fmt.Errorf("Could not get run benchmark output: %s", err)
	}
	// Print the output and return.
	fmt.Println(output)
	return nil
}

// downloadTrace downloads the traceURL from google storage into the specified
// destDir.
func downloadTrace(traceURL, destDir string, gs *util.GcsUtil) (string, error) {
	traceName := getTraceName(traceURL)
	traceBucket := getBucketName(traceURL)
	traceDest := filepath.Join(destDir, traceName)
	if err := gs.DownloadRemoteFileFromBucket(traceBucket, traceName, traceDest); err != nil {
		return "", fmt.Errorf("Error downloading %s from %s to %s: %s", traceName, traceBucket, traceDest, err)
	}
	return traceDest, nil
}

// getBucketName parses the provided traceURL and returns the name of the cloud bucket.
// Returns DEFAULT_TRACE_OUTPUT_BUCKET if the provided URL is not in the form that
// cloudUrlBucketRegex expects.
func getBucketName(traceURL string) string {
	m := cloudUrlBucketRegex.FindStringSubmatch(traceURL)
	if len(m) == 2 {
		return m[1]
	} else {
		return DEFAULT_TRACE_OUTPUT_BUCKET
	}
}

// getTraceName parses the provided traceURI and returns the name of the trace.
// traceURI could be a file path or a URL.
func getTraceName(traceURI string) string {
	traceTokens := strings.Split(traceURI, "/")
	return traceTokens[len(traceTokens)-1]
}

// getClosedChannelOfTraces returns a channel that contains all trace URLs.
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
