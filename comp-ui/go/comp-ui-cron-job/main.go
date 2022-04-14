// Application to wrap Python scripts downloaded from Gerrit.
//
// The Python scripts run benchmarks and emit Perf compatible JSON files, which
// are then uploaded to Google Cloud Storage.
//
// Every Python script must support the following flags:
//
//     -b BROWSER, --browser=BROWSER
//                           The browser to use to run MotionMark in.
//     -s SUITE, --suite=SUITE
//                           Run only the specified suite of tests.
//     -e EXECUTABLE, --executable-path=EXECUTABLE
//                           Path to the executable to the driver binary.
//     -a ARGUMENTS, --arguments=ARGUMENTS
//                           Extra arguments to pass to the browser.
//     -g GITHASH, --githash=GITHASH
//                           A git-hash associated with this run.
//     -o OUTPUT, --output=OUTPUT
//                           Path to the output json file.
//
package main

import (
	"context"
	"encoding/base64"
	"flag"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"cloud.google.com/go/storage"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/executil"
	"go.skia.org/infra/go/gcs"
	"go.skia.org/infra/go/gcs/gcsclient"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/now"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"google.golang.org/api/option"
)

const (
	// The Google Cloud Storage Bucket to write the results to.
	bucket = "chrome-comp-ui-perf-skia"

	// The path in the bucket where Perf results should be written.
	bucketPath = "perf"

	// The repo that has commits associated with runs of the cron job.
	repo = "https://skia.googlesource.com/perf-compui"

	scriptExecutable = "python3"
)

// flags
var (
	local = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
)

var (
	// Key can be changed via -ldflags.
	Key = "base64 encoded service account key JSON goes here."

	// Version can be changed via -ldflags.
	Version = "unsupplied"
)

// benchmark represents a single benchmark configuration.
type benchmark struct {
	// The URL to download a Python script that actually runs the benchmarks, as served up from Gitiles.
	downloadURL string

	// Flags to pass to the Python script.
	flags []string
}

// All the various benchmarks we run.
var benchmarks = map[string]benchmark{
	// We always run the canary to validate that the whole pipeline works even
	// if the "real" benchmark scripts start to fail.
	"canary": {
		downloadURL: "https://skia.googlesource.com/buildbot/+/refs/heads/main/comp-ui/benchmark-mock.py?format=TEXT",
		flags: []string{
			"--browser", "mock",
		},
	},
	"chrome": {
		downloadURL: "https://chromium.googlesource.com/chromium/src/+/refs/heads/main/tools/browserbench-webdriver/motionmark.py?format=TEXT",
		flags: []string{
			"--browser", "chrome",
			"--executable-path", filepath.Join(os.Getenv("HOME"), "chromedriver"),
		},
	},
}

func main() {
	common.InitWithMust(
		"comp-ui-cron-job",
		common.CloudLogging(local, "skia-public"),
	)
	sklog.Infof("Version: %s", Version)

	ctx := context.Background()
	gcsClient, err := getGcsClient(ctx)
	if err != nil {
		sklog.Fatal(err)
	}

	workDir, err := os.MkdirTemp("", "comp-ui-cron-job")
	if err != nil {
		sklog.Fatal(err)
	}

	gitHash, err := getGitHash(ctx, workDir)
	if err != nil {
		sklog.Fatal(err)
	}

	httpClient := httputils.DefaultClientConfig().With2xxOnly().Client()
	for benchmarkName, config := range benchmarks {
		outputFilename, err := runSingleBenchMark(ctx, benchmarkName, config, gitHash, workDir, httpClient)
		if err != nil {
			sklog.Errorf("Failed to run benchmark %q: %s", benchmarkName, err)
			continue
		}

		err = uploadResultsFile(ctx, gcsClient, benchmarkName, outputFilename)
		if err != nil {
			sklog.Errorf("Failed to upload benchmark results %q: %s", benchmarkName, err)
		}
	}
}

func getGitHash(ctx context.Context, workDir string) (string, error) {
	// Find the githash for 'today' from https://skia.googlesource.com/perf-compui.
	g, err := git.NewRepo(ctx, repo, filepath.Join(workDir, "git"))
	if err != nil {
		return "", skerr.Wrap(err)
	}
	hashes, err := g.RevList(ctx, "HEAD", "-n1")
	if err != nil {
		return "", skerr.Wrap(err)
	}
	return hashes[0], nil
}

func getGcsClient(ctx context.Context) (*gcsclient.StorageClient, error) {
	ts, err := auth.NewTokenSourceFromKeyString(ctx, *local, Key, storage.ScopeFullControl)
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	storageClient, err := storage.NewClient(ctx, option.WithTokenSource(ts))
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	gcsClient := gcsclient.New(storageClient, bucket)
	return gcsClient, nil

}

func runSingleBenchMark(ctx context.Context, benchmarkName string, config benchmark, gitHash string, workDir string, httpClient *http.Client) (string, error) {
	sklog.Infof("runSingleBenchMark - benchmarkName: %q  url: %q  gitHash: %q workDir: %q", benchmarkName, config.downloadURL, gitHash, workDir)

	// Compute the filenames we will use.
	scriptBaseName := benchmarkName + ".py"
	scriptFilename := filepath.Join(workDir, scriptBaseName)
	outputDirectory := filepath.Join(workDir, benchmarkName)
	outputFilename := filepath.Join(outputDirectory, "results.json")

	// Create output directory.
	err := os.MkdirAll(outputDirectory, 0755)
	if err != nil {
		return "", skerr.Wrap(err)
	}

	err = downloadPythonScript(ctx, config.downloadURL, scriptFilename, httpClient)
	if err != nil {
		return "", skerr.Wrap(err)
	}

	// Compute the full set of args to pass to the python script.
	flags := append([]string{}, config.flags...)
	flags = append(flags, "--githash", gitHash)
	flags = append(flags, "--output", outputFilename)
	args := append([]string{scriptFilename}, flags...)

	// Run the script.
	err = runBenchMarkScript(ctx, args, workDir)
	if err != nil {
		return "", skerr.Wrap(err)
	}
	return outputFilename, nil
}

func runBenchMarkScript(ctx context.Context, args []string, workDir string) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Minute)
	defer cancel()

	cmd := executil.CommandContext(ctx, scriptExecutable, args...)
	cmd.Dir = workDir

	output, err := cmd.CombinedOutput()
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		sklog.Info(line)
	}
	if err != nil {
		return skerr.Wrap(err)
	}
	return nil
}

func uploadResultsFile(ctx context.Context, gcsClient gcs.GCSClient, benchmarkName string, outputFilename string) error {
	destinationPath := path.Join(computeUploadPathFromTime(ctx), benchmarkName, "results.json")
	w := gcsClient.FileWriter(ctx, destinationPath, gcs.FileWriteOptions{
		ContentEncoding: "application/json",
	})

	// Copy the output file up to GCS.
	err := util.WithReadFile(outputFilename, func(f io.Reader) error {
		_, err := io.Copy(w, f)
		return skerr.Wrap(err)
	})
	if err != nil {
		return skerr.Wrap(err)
	}

	return skerr.Wrap(w.Close())
}

// computeUploadPathFromTime returns the date-time portion of the GCS path that
// Perf expects uploads to use, see
// https://skia.googlesource.com/buildbot/+/refs/heads/main/perf/FORMAT.md.
func computeUploadPathFromTime(ctx context.Context) string {
	return now.Now(ctx).UTC().Format("2006/01/02/15")
}

// downloadPythonScript downloads the script from Gitiles and writes it to the
// workDir.
//
// It also base64 decodes the downloaded file since that's how Gitiles serves up
// 'raw' files.
func downloadPythonScript(ctx context.Context, downloadURL string, filename string, httpClient *http.Client) error {
	// Retrieve the file.
	resp, err := httpClient.Get(downloadURL)
	if err != nil {
		return skerr.Wrap(err)
	}
	defer util.Close(resp.Body)
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return skerr.Wrap(err)
	}

	// Base64 decode the file.
	decoded, err := base64.StdEncoding.DecodeString(string(b))
	if err != nil {
		return skerr.Wrap(err)
	}

	// Write the Python file to its destination.
	return util.WithWriteFile(filename, func(w io.Writer) error {
		_, err := w.Write(decoded)
		return skerr.Wrap(err)
	})
}
