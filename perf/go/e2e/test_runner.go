package main

import (
	"context"
	"encoding/xml"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"time"

	"cloud.google.com/go/storage"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/gitauth"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"google.golang.org/api/iterator"

	"go.skia.org/infra/task_driver/go/lib/bazel"
	"go.skia.org/infra/task_driver/go/lib/checkout"
	"go.skia.org/infra/task_driver/go/lib/git_steps"
	"go.skia.org/infra/task_driver/go/lib/os_steps"
	"go.skia.org/infra/task_driver/go/td"
)

// flags
var (
	outputPath        = flag.String("output", "local-test-results", "Test result output path.")
	bucketName        = flag.String("bucket", "", "The GCS bucket name to upload the test result to.")
	bazelCacheDir     = flag.String("bazel_cache_dir", "", "Path to the Bazel cache directory.")
	bazelRepoCacheDir = flag.String("bazel_repo_cache_dir", "", "Path to the Bazel repository cache directory.")
	workdir           = flag.String("workdir", "", "Directory to use for scratch work.")
	rbeKey            = flag.String("rbe_key", "", "Path to the service account key to use for RBE.")
	projectId         = flag.String("project_id", "skia-swarming-bots", "The GCE project.")
	taskId            = flag.String("task_id", "", "The Swarming task ID.")
	taskName          = flag.String("task_name", "e2e-test-runner", "The Swarming task name.")

	checkoutFlags = checkout.SetupFlags(nil)

	local = flag.Bool("local", true, "Running locally if true. As opposed to running on a bot.")
)

var (
	testResultsFileName = "test_result.xml"
)

const (
	maxObjectPrefixRetries = 10
)

// TestSuites is the root element for xUnit XML reports.
type TestSuites struct {
	XMLName   xml.Name    `xml:"testsuites"`
	Name      string      `xml:"name,attr"`
	TestSuite []TestSuite `xml:"testsuite"`
}

// TestSuite represents a single suite of tests.
type TestSuite struct {
	XMLName   xml.Name   `xml:"testsuite"`
	Name      string     `xml:"name,attr"`
	Tests     int        `xml:"tests,attr"`
	Failures  int        `xml:"failures,attr"`
	Errors    int        `xml:"errors,attr"`
	Skipped   int        `xml:"skipped,attr"`
	Timestamp string     `xml:"timestamp,attr"`
	Time      string     `xml:"time,attr"`
	TestCases []TestCase `xml:"testcase"`
}

// TestCase represents a single test case.
type TestCase struct {
	XMLName   xml.Name `xml:"testcase"`
	Name      string   `xml:"name,attr"`
	ClassName string   `xml:"classname,attr"`
	Result    string   `xml:"result,attr"`
	Time      string   `xml:"time,attr"`
}

// generateUniqueObjectPrefix creates a unique GCS object prefix.
func generateUniqueObjectPrefix(ctx context.Context, client *storage.Client) (string, error) {
	now := time.Now().UTC()
	basePrefix := now.Format("2006-01-02/15-04-05")
	objectPrefix := basePrefix
	counter := 0

	// Create a unique GCS folder for storing the test result.
	for {
		it := client.Bucket(*bucketName).Objects(ctx, &storage.Query{Prefix: objectPrefix + "/"})
		_, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return "", fmt.Errorf("failed to check for existing GCS objects: %w", err)
		}

		counter++
		if counter > maxObjectPrefixRetries {
			return "", fmt.Errorf("failed to find a unique object prefix after %d tries", counter)
		}
		objectPrefix = fmt.Sprintf("%s_%d", basePrefix, counter)
	}
	return objectPrefix, nil
}

// uploadFile uploads the given file to GCS.
func uploadFile(ctx context.Context, filePath string) error {
	if *bucketName == "" {
		return nil
	}

	client, err := storage.NewClient(ctx)
	if err != nil {
		return fmt.Errorf("storage.NewClient: %w", err)
	}
	defer client.Close()

	objectPrefix, err := generateUniqueObjectPrefix(ctx, client)
	if err != nil {
		return fmt.Errorf("failed to generate unique object name: %w", err)
	}
	objectName := filepath.Join(objectPrefix, testResultsFileName)

	// Open local file.
	f, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("os.Open: %w", err)
	}
	defer f.Close()

	wc := client.Bucket(*bucketName).Object(objectName).NewWriter(ctx)
	wc.ContentType = "application/xml"

	if _, err := io.Copy(wc, f); err != nil {
		return fmt.Errorf("io.Copy: %w", err)
	}

	if err := wc.Close(); err != nil {
		return fmt.Errorf("failed to close GCS writer: %w", err)
	}

	sklog.Infof("Successfully uploaded test result to gs://%s/%s", *bucketName, objectName)
	return nil
}

// runNodejsTest runs an end-to-end Nodejs test.
func runNodejsTest(ctx context.Context, testFile string) (string, int, error) {
	// Compute work dir path.
	var workDir string
	var err error
	if *workdir == "" {
		workDir, err = os.MkdirTemp("", "")
		if err != nil {
			return "", 0, err
		}
		defer util.RemoveAll(workDir)
	} else {
		workDir, err = os_steps.Abs(ctx, *workdir)
		if err != nil {
			td.Fatal(ctx, err)
		}
	}

	// Check out the code.
	ts, err := git_steps.Init(ctx, *local)
	if err != nil {
		td.Fatal(ctx, err)
	}
	if !*local {
		client := httputils.DefaultClientConfig().WithTokenSource(ts).Client()
		g, err := gerrit.NewGerrit("https://skia-review.googlesource.com", client)
		if err != nil {
			td.Fatal(ctx, err)
		}
		email, err := g.GetUserEmail(ctx)
		if err != nil {
			td.Fatal(ctx, err)
		}
		if _, err := gitauth.New(ctx, ts, "/tmp/.gitcookies", true, email); err != nil {
			td.Fatal(ctx, err)
		}
	}

	repoState, err := checkout.GetRepoState(checkoutFlags)
	if err != nil {
		td.Fatal(ctx, err)
	}
	repoPath := filepath.Join(workDir, "repo")
	gitDir, err := checkout.EnsureGitCheckout(ctx, repoPath, repoState)
	if err != nil {
		td.Fatal(ctx, err)
	}

	opts := bazel.BazelOptions{
		CachePath:           *bazelCacheDir,
		RepositoryCachePath: *bazelRepoCacheDir,
	}
	bzl, err := bazel.New(ctx, gitDir.Dir(), *rbeKey, opts)
	if err != nil {
		return "", 0, err
	}

	result, err := bzl.Do(ctx, "test", "--config=mayberemote", "--nocache_test_results", testFile, "--test_output=all")
	if err != nil {
		// Find the number of failing tests from the output.
		re := regexp.MustCompile(`(\d+) (test|tests) FAILED`)
		matches := re.FindStringSubmatch(result)
		if len(matches) > 1 {
			failures, err := strconv.Atoi(matches[1])
			if err != nil {
				sklog.Warningf("Failed to convert number of failures to int: %v", err)
			}
			return result, failures, nil
		}
		sklog.Warningf("Missing number of failures from output: %v", err)
		return result, 1, nil
	}

	return result, 0, nil
}

// generateDummyTestResult generates a dummy test results xml.
// After adding real tests, this function must be removed.
func generateDummyTestResult(ctx context.Context) ([]byte, error) {
	result, failures, err := runNodejsTest(ctx, "//perf/go/e2e/tests:example_nodejs_test")
	if err != nil {
		return nil, fmt.Errorf("failed to run a nodejs test: %w", err)
	}

	suites := TestSuites{
		Name: "results",
		TestSuite: []TestSuite{
			{
				Name:      "dummy suite",
				Tests:     1,
				Failures:  failures,
				Errors:    0,
				Skipped:   0,
				Timestamp: time.Now().UTC().Format(time.RFC3339),
				Time:      "0.1",
				TestCases: []TestCase{
					{
						Name:      "dummy test",
						ClassName: "dummy.class",
						Result:    result,
						Time:      "0.1",
					},
				},
			},
		},
	}

	xmlBytes, err := xml.MarshalIndent(suites, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("error marshalling XML: %w", err)
	}

	return xmlBytes, nil
}

func main() {
	ctx := td.StartRun(projectId, taskId, taskName, outputPath, local)
	defer td.EndRun(ctx)

	flag.Parse()

	if *outputPath == "" {
		sklog.Fatal("The --output flag must be provided.")
	}

	xmlBytes, err := generateDummyTestResult(ctx)
	if err != nil {
		sklog.Fatalf("Failed to generate test result: %v", err)
	}

	if *bucketName == "" {
		if _, err := os.Stat(*outputPath); os.IsNotExist(err) {
			if err := os.MkdirAll(*outputPath, 0755); err != nil {
				sklog.Fatalf("Failed to create output directory %s: %v", *outputPath, err)
			}
		}
	}
	filePath := filepath.Join(*outputPath, testResultsFileName)
	if err := os.WriteFile(filePath, xmlBytes, 0644); err != nil {
		sklog.Fatalf("Failed to write to test result file: %v", err)
	}
	sklog.Infof("Successfully generated test result at %s", filePath)

	if err := uploadFile(ctx, filePath); err != nil {
		sklog.Fatalf("Failed to upload test result to GCS: %v", err)
	}
}
