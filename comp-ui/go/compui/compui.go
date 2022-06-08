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
//     --extra-keys=KEY_VALUE_PAIRS
//                           A comma separated list of key,value pairs.
//
package compui

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"cloud.google.com/go/storage"

	"go.skia.org/infra/comp-ui/go/compui/download"
	"go.skia.org/infra/comp-ui/go/compui/urls"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/executil"
	"go.skia.org/infra/go/gcs"
	"go.skia.org/infra/go/gcs/gcsclient"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/gitauth"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/now"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"golang.org/x/oauth2"
	"google.golang.org/api/option"
)

const (
	// The Google Cloud Storage Bucket to write the results to.
	bucket = "chrome-comp-ui-perf-skia"

	// The path in the bucket where Perf results should be written.
	bucketPath = "ingest"

	// The repo that has commits associated with runs of the cron job.
	repo = "https://skia.googlesource.com/perf-compui"

	benchmarkTimeout = 2 * time.Hour
)

// DriverType is the type of executable to pass to the script via the
// --executable-path flag.
type DriverType string

const (
	ChromeStableDriver DriverType = "stable"
	ChromeCanaryDriver DriverType = "canary"
	NoDriver           DriverType = ""
)

// Benchmark represents a single Benchmark configuration.
type Benchmark struct {
	// The checkout URL of the git repo that contains the scripts to run
	RepoURL string `json:"repoURL"`

	// The directories in the git repo that need to be checked out.
	CheckoutPaths []string `json:"checkoutPaths"`

	// The full name of the script to run in the git repo relative to the root
	// of the checkout.
	ScriptName string `json:"scriptName"`

	// DriverType is the type of driver Selenium needs.
	DriverType DriverType `json:"driverType"`

	// Flags to pass to the Python script.
	Flags []string `json:"flags"`
}

var defaultBenchmarks = map[string]*Benchmark{
	// We always run the canary to validate that the whole pipeline works even
	// if the "real" benchmark scripts start to fail.
	"canary": {
		RepoURL:       "https://skia.googlesource.com/buildbot",
		CheckoutPaths: []string{"comp-ui"},
		ScriptName:    "comp-ui/benchmark-mock.py",
		DriverType:    NoDriver,
		Flags: []string{
			"--browser", "mock",
		},
	},
	"chrome-motionmark": {
		RepoURL:       "https://chromium.googlesource.com/chromium/src",
		CheckoutPaths: []string{"tools/browserbench-webdriver"},
		ScriptName:    "tools/browserbench-webdriver/motionmark.py",
		DriverType:    ChromeStableDriver,
		Flags: []string{
			"--browser", "chrome",
			"--extra-keys", "channel,stable",
		},
	},
	"chrome-jetstream": {
		RepoURL:       "https://chromium.googlesource.com/chromium/src",
		CheckoutPaths: []string{"tools/browserbench-webdriver"},
		ScriptName:    "tools/browserbench-webdriver/jetstream.py",
		DriverType:    ChromeStableDriver,
		Flags: []string{
			"--browser", "chrome",
			"--extra-keys", "channel,stable",
		},
	},
	"chrome-speedometer": {
		RepoURL:       "https://chromium.googlesource.com/chromium/src",
		CheckoutPaths: []string{"tools/browserbench-webdriver"},
		ScriptName:    "tools/browserbench-webdriver/speedometer.py",
		DriverType:    ChromeStableDriver,
		Flags: []string{
			"--browser", "chrome",
			"--extra-keys", "channel,stable",
		},
	},
	"safari-speedometer": {
		RepoURL:       "https://chromium.googlesource.com/chromium/src",
		CheckoutPaths: []string{"tools/browserbench-webdriver"},
		ScriptName:    "tools/browserbench-webdriver/speedometer.py",
		DriverType:    NoDriver,
		Flags: []string{
			"--browser", "safari",
			"--extra-keys", "channel,stable",
		},
	},
}

// Application used to control how Run() executes.
type Application struct {
	local                      bool
	useDefaultAuth             bool
	python                     string
	benchmarksFile             string
	downloadDrivers            bool
	chromeDriverFilename       string
	chromeCanaryDriverFilename string
	skipUploads                bool
}

// Flagset constructs a flag.FlagSet for the Application.
func (f *Application) Flagset() *flag.FlagSet {
	flagSet := flag.NewFlagSet("", flag.ExitOnError)
	flagSet.Usage = func() {
		fmt.Printf("Usage: %s <flags>\n\n", os.Args[0])
		flagSet.PrintDefaults()
	}

	flagSet.BoolVar(&f.local, "local", false, "Running locally if true. As opposed to in production.")
	flagSet.BoolVar(&f.useDefaultAuth, "use-default-auth", false, "Use Google Default Application Credentials if true, otherwise use embedded auth Key.")
	flagSet.StringVar(&f.python, "python-exe", "/Library/Frameworks/Python.framework/Versions/3.9/bin/python3", "Absolute path to the Python exe to use.")
	flagSet.StringVar(&f.benchmarksFile, "benchmarks", "", "If provided, read the configs to test from the provided JSON file.")
	flagSet.BoolVar(&f.downloadDrivers, "download_drivers", true, "If true then download the Chrome drivers.")
	flagSet.StringVar(&f.chromeDriverFilename, "chrome_driver", "", "If --download_drivers is false then this flag must provide the absolute path to an already downloaded Chrome Selenium driver executable.")
	flagSet.StringVar(&f.chromeCanaryDriverFilename, "chrome_canary_driver", "", "If --download_drivers is false then this flag must provide the absolute path to an already downloaded Chrome Canary Selenium driver executable.")
	flagSet.BoolVar(&f.skipUploads, "skip_uploads", false, "If true then skip uploading the files.")

	return flagSet
}

// Main runs the application.
func Main(version, key string) {
	var app Application

	common.InitWithMust(
		"comp-ui-cron-job",
		common.CloudLogging(&app.local, "skia-public"),
		common.FlagSetOpt(app.Flagset()),
	)
	app.Run(version, key)
}

// Run the application with the given flags.
func (app Application) Run(version, key string) {
	sklog.Infof("Version: %s", version)

	ctx := context.Background()

	benchmarks := defaultBenchmarks
	if app.benchmarksFile != "" {
		var err error
		benchmarks, err = readBenchMarksFromFile(ctx, app.benchmarksFile)
		if err != nil {
			sklog.Fatal(err)
		}
	}

	chromeDriver, chromeCanaryDriver, cleanup, err := driverFilenames(app.downloadDrivers, app.chromeDriverFilename, app.chromeCanaryDriverFilename)

	if err != nil {
		sklog.Fatal(err)
	}

	populateBenchmarksWithDrivers(benchmarks, chromeDriver, chromeCanaryDriver)

	ts, err := auth.NewTokenSourceFromKeyString(ctx, app.useDefaultAuth, key, storage.ScopeFullControl, auth.ScopeUserinfoEmail, auth.ScopeGerrit)
	if err != nil {
		sklog.Fatal(err)
	}

	gcsClient, err := getGCSClient(ctx, ts)
	if err != nil {
		sklog.Fatal(err)
	}

	workDir, err := os.MkdirTemp("", "comp-ui-cron-job")
	if err != nil {
		sklog.Fatal(err)
	}
	defer func() {
		err := os.RemoveAll(workDir)
		if err != nil {
			sklog.Error(err)
		}
		if err := cleanup(); err != nil {
			sklog.Fatal(err)
		}
	}()

	// We presume that if running locally that you've already authenticated to
	// Gerrit, otherwise write out a git cookie that enables R/W access to the
	// git repo.
	//
	// Authenticate to Gerrit since the perf-compui repo is private.
	if !app.useDefaultAuth {
		sklog.Info("Configuring git auth.")
		if _, err := gitauth.New(ts, "/tmp/git-cookie", true, ""); err != nil {
			sklog.Fatal(err)
		}
	}

	sklog.Info("Getting githash.")
	gitHash, err := getGitHash(ctx, workDir)
	if err != nil {
		sklog.Fatal(err)
	}

	for benchmarkName, config := range benchmarks {
		outputFilename, err := runSingleBenchmark(ctx, app.python, benchmarkName, config, gitHash, workDir)
		if err != nil {
			sklog.Errorf("Failed to run benchmark %q: %s", benchmarkName, err)
			continue
		}
		if app.skipUploads {
			continue
		}
		err = uploadResultsFile(ctx, gcsClient, benchmarkName, outputFilename)
		if err != nil {
			sklog.Errorf("Failed to upload benchmark results %q: %s", benchmarkName, err)
		}
	}
	sklog.Flush()
}

func driverFilenames(downloadDrivers bool, chromeDriverFilename, chromeCanaryDriverFilename string) (string, string, download.Cleanup, error) {
	if !downloadDrivers {
		if chromeDriverFilename == "" {
			return "", "", nil, fmt.Errorf("Since --download_drivers=false, the --chrome_driver must be supplied.")
		}
		if chromeCanaryDriverFilename == "" {
			return "", "", nil, fmt.Errorf("Since --download_drivers=false, the --chrome_canary_driver must be supplied.")
		}
		return chromeDriverFilename, chromeCanaryDriverFilename, download.NoopCleanup, nil
	}
	/*
		We need to download and use the latest vesion of the Selenium Chrome driver for both
		Stable and Canary channels.

		For the Stable driver the version to use is available at:

		https://chromedriver.storage.googleapis.com/LATEST_RELEASE

			$ curl https://chromedriver.storage.googleapis.com/LATEST_RELEASE
			102.0.5005.61

		That version value can then be used to construct the URL of file to download:

			$ curl https://chromedriver.storage.googleapis.com/102.0.5005.61/chromedriver_mac64_m1.zip --output /tmp/chr.zip
			$ unzip chr.zip
			Archive:  chr.zip
			  inflating: chromedriver


		The same goes for the canary driver, but note that the os/arch is also part of the URL:

		https://commondatastorage.googleapis.com/chromium-browser-snapshots/Mac_Arm/LAST_CHANGE


			$ curl https://commondatastorage.googleapis.com/chromium-browser-snapshots/Mac_Arm/LAST_CHANGE
			1010187

		Again we can use the version value and the os/arch to build the download URL:

			$ curl https://commondatastorage.googleapis.com/chromium-browser-snapshots/Mac_Arm/1010187/chromedriver_mac64.zip --output /tmp/driver.zip
			$ unzip /tmp/driver.zip
			Archive:  driver.zip
			  inflating: chromedriver_mac64/chromedriver

		The list of os/arch filenames we care about are:

			chromedriver_linux64.zip
			chromedriver_mac64.zip
			chromedriver_mac64_m1.zip

		There's also a Win version, but we ignore that for now.
	*/

	urls, err := urls.NewDownloadURLs(runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return "", "", nil, err
	}
	client := httputils.DefaultClientConfig().Client()
	driverFilename, driverCleanup, err := download.DownloadAndUnzipDriver(client, urls.LatestURL, urls.DriverURL)
	if err != nil {
		return "", "", nil, err
	}

	canaryDriverFilename, canaryDriverCleanup, err := download.DownloadAndUnzipDriver(client, urls.LatestCanaryURL, urls.CanaryDriverURL)
	if err != nil {
		if err := driverCleanup(); err != nil {
			sklog.Error(err)
		}
		return "", "", nil, err
	}

	// Build a single cleanup from the two cleanup functions we got from both
	// downloadAndUnzip calls.
	cleanup := func() error {
		if err := driverCleanup(); err != nil {
			return err
		}
		if err := canaryDriverCleanup(); err != nil {
			return err
		}
		return nil
	}
	return driverFilename, canaryDriverFilename, cleanup, nil
}

func populateBenchmarksWithDrivers(in map[string]*Benchmark, chromeDriverFilename, chromeCanaryDriverFilename string) {
	for _, value := range in {
		switch value.DriverType {
		case ChromeStableDriver:
			value.Flags = append(value.Flags, "--executable-path", chromeDriverFilename)
		case ChromeCanaryDriver:
			value.Flags = append(value.Flags, "--executable-path", chromeCanaryDriverFilename)
		}
	}
}

func readBenchMarksFromFile(ctx context.Context, filename string) (map[string]*Benchmark, error) {
	sklog.Infof("Reading configs from %q", filename)
	benchmarks := map[string]*Benchmark{}
	b, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to read file.")
	}
	if err := json.Unmarshal(b, &benchmarks); err != nil {
		return nil, skerr.Wrapf(err, "Failed to parse JSON.")
	}
	return benchmarks, nil
}

// getGitHash returns the git hash of the last commit to the perf-compui repo,
// which only gets a single commit per day.
func getGitHash(ctx context.Context, workDir string) (string, error) {
	// Find the githash for 'today' from https://skia.googlesource.com/perf-compui.
	g, err := git.NewRepo(ctx, repo, filepath.Join(workDir, "perf-compui"))
	if err != nil {
		return "", skerr.Wrap(err)
	}

	hashes, err := g.RevList(ctx, "HEAD", "-n1")
	if err != nil {
		return "", skerr.Wrap(err)
	}
	return hashes[0], nil
}

func getGCSClient(ctx context.Context, ts oauth2.TokenSource) (*gcsclient.StorageClient, error) {
	storageClient, err := storage.NewClient(ctx, option.WithTokenSource(ts))
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	gcsClient := gcsclient.New(storageClient, bucket)
	return gcsClient, nil

}

func runSingleBenchmark(ctx context.Context, python string, benchmarkName string, config *Benchmark, gitHash string, workDir string) (string, error) {
	sklog.Infof("runSingleBenchMark - benchmarkName: %q  url: %q  gitHash: %q workDir: %q", benchmarkName, config.ScriptName, gitHash, workDir)

	gitCheckoutDir, err := checkoutPythonScript(ctx, config, workDir, benchmarkName)
	if err != nil {
		return "", skerr.Wrap(err)
	}

	// Compute the filenames we will use.
	scriptFilename := filepath.Join(gitCheckoutDir, config.ScriptName)
	outputDirectory := filepath.Join(workDir, benchmarkName)
	outputFilename := filepath.Join(outputDirectory, "results.json")

	// Create output directory.
	err = os.MkdirAll(outputDirectory, 0755)
	if err != nil {
		return "", skerr.Wrap(err)
	}

	// Compute the full set of args to pass to the python script.
	flags := append([]string{}, config.Flags...)
	flags = append(flags, "--githash", gitHash)
	flags = append(flags, "--output", outputFilename)
	args := append([]string{scriptFilename}, flags...)

	sklog.Infof("Running: %q", args)

	// Run the script.
	err = runBenchMarkScript(ctx, python, args, workDir)
	if err != nil {
		return "", skerr.Wrap(err)
	}
	return outputFilename, nil
}

func runBenchMarkScript(ctx context.Context, python string, args []string, workDir string) error {
	ctx, cancel := context.WithTimeout(ctx, benchmarkTimeout)
	defer cancel()

	return runCmdLogOutput(ctx, workDir, python, args...)
}

func uploadResultsFile(ctx context.Context, gcsClient gcs.GCSClient, benchmarkName string, outputFilename string) error {
	// GCS paths always use "/" separators.
	destinationPath := path.Join(bucketPath, computeUploadPathFromTime(ctx), benchmarkName, "results.json")
	sklog.Infof("Upload to %q", destinationPath)
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

// checkoutPythonScript checks out a sparse checkout of the specified directories
// into workDir.
func checkoutPythonScript(ctx context.Context, config *Benchmark, workDir string, benchmarkName string) (string, error) {
	dest := filepath.Join(workDir, "git", benchmarkName)
	return dest, newSparseCheckout(ctx, workDir, config.RepoURL, dest, config.CheckoutPaths)
}

// runCmdLogOutput runs a command using executil.CommandContext and logs any output
// to sklog.Info().
func runCmdLogOutput(ctx context.Context, cwd string, cmd string, args ...string) error {
	cc := executil.CommandContext(ctx, cmd, args...)
	cc.Dir = cwd
	output, err := cc.CombinedOutput()
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		sklog.Info(line)
	}
	if err != nil {
		return skerr.Wrap(err)
	}
	return nil
}

// newSparseCheckout does a sparse checkout of the given 'directories'.
func newSparseCheckout(ctx context.Context, workDir, repoURL, dest string, directories []string) error {
	if err := runCmdLogOutput(ctx, workDir, "git", "clone", "--depth", "1", "--filter=blob:none", "--sparse", repoURL, dest); err != nil {
		return skerr.Wrapf(err, "Failed to clone.")
	}
	if err := runCmdLogOutput(ctx, dest, "git", "sparse-checkout", "init", "--cone"); err != nil {
		return skerr.Wrapf(err, "Failed to init sparse checkout.")
	}

	args := []string{"sparse-checkout", "set"}
	args = append(args, directories...)
	if err := runCmdLogOutput(ctx, dest, "git", args...); err != nil {
		return skerr.Wrapf(err, "Failed to do a sparse checkout.")
	}

	return nil
}
