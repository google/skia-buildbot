package compui

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/executil"
	"go.skia.org/infra/go/gcs"
	"go.skia.org/infra/go/gcs/mocks"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/now"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
)

const (
	myFakePythonExe             = "/usr/local/bin/python"
	alternateChromeDriver       = "/tmp/my-chrome-driver"
	alternateChromeCanaryDriver = "/tmp/my-chrome-canary-driver"
)

func setupForTest(t *testing.T, h http.HandlerFunc) (string, *http.Client) {
	client := httputils.DefaultClientConfig().WithoutRetries().With2xxOnly().Client()

	return t.TempDir(), client
}

func TestComputeUploadPathFromTime(t *testing.T) {
	unittest.SmallTest(t)
	var mockTime = time.Unix(1649770315, 12).UTC()
	ctx := context.WithValue(context.Background(), now.ContextKey, mockTime)
	assert.Equal(t, "2022/04/12/13", computeUploadPathFromTime(ctx))
}

var benchmarkScriptArgs = []string{"--output", "results.json"}

func TestRunBenchMarkScript_ScriptReturnsError_ReturnsError(t *testing.T) {
	unittest.MediumTest(t)

	workDir, _ := setupForTest(t, nil)
	ctx := executil.WithFakeTests(context.Background(), "Test_FakeExe_Exec_Fails")

	err := runBenchMarkScript(ctx, myFakePythonExe, benchmarkScriptArgs, workDir)
	require.Error(t, err)
}

func TestRunBenchMarkScript_ScriptSucceeds_DoesNotReturnError(t *testing.T) {
	unittest.MediumTest(t)

	workDir, _ := setupForTest(t, nil)
	ctx := executil.WithFakeTests(context.Background(), "Test_FakeExe_Python_Script_Success")

	err := runBenchMarkScript(ctx, myFakePythonExe, benchmarkScriptArgs, workDir)
	require.NoError(t, err)
}

func Test_FakeExe_Exec_Fails(t *testing.T) {
	unittest.FakeExeTest(t)
	if os.Getenv(executil.OverrideEnvironmentVariable) == "" {
		return
	}
	os.Exit(1)
}

func Test_FakeExe_Python_Script_Success(t *testing.T) {
	unittest.FakeExeTest(t)
	if os.Getenv(executil.OverrideEnvironmentVariable) == "" {
		return
	}

	args := executil.OriginalArgs()
	require.Contains(t, args, myFakePythonExe)
	require.Contains(t, args, "--output")
	os.Exit(0)
}

const (
	repoURL = "https://example.com/git"
)

var (
	// We need to use real directories for workDir and dest because the exec
	// package checks that Command.Path exists before running the command.
	workDir = os.TempDir()

	dest = os.TempDir()

	directories = []string{"a/b", "c"}

	benchmarkName = "canary"

	benchmarkConfig = &Benchmark{
		RepoURL:       repoURL,
		CheckoutPaths: directories,
		ScriptName:    "a/b/benchmark.py",
		Flags:         []string{"--githash", "abcdef"},
	}
)

func TestNewSparseCheckout_Success(t *testing.T) {
	unittest.MediumTest(t)
	ctx := executil.WithFakeTests(context.Background(),
		"Test_FakeExe_Clone_Success",
		"Test_FakeExe_Sparse_Init_Success",
		"Test_FakeExe_Sparse_Set_Success",
	)

	err := newSparseCheckout(ctx, workDir, repoURL, dest, directories)
	require.NoError(t, err)
}

func TestNewSparseCheckout_GitCommandFails_ReturnsError(t *testing.T) {
	unittest.MediumTest(t)
	ctx := executil.WithFakeTests(context.Background(),
		"Test_FakeExe_Exec_Fails",
	)

	err := newSparseCheckout(ctx, workDir, repoURL, dest, directories)
	require.Error(t, err)
}

func Test_FakeExe_Clone_Success(t *testing.T) {
	unittest.FakeExeTest(t)
	if os.Getenv(executil.OverrideEnvironmentVariable) == "" {
		return
	}

	args := executil.OriginalArgs()
	expected := []string{"git", "clone", "--depth", "1", "--filter=blob:none", "--sparse", "https://example.com/git"}
	// The checkout dest will always change, so just check all the other arguments.
	require.Equal(t, expected, args[:len(expected)])
	os.Exit(0)
}

func Test_FakeExe_Sparse_Init_Success(t *testing.T) {
	unittest.FakeExeTest(t)
	if os.Getenv(executil.OverrideEnvironmentVariable) == "" {
		return
	}

	args := executil.OriginalArgs()
	expected := []string{"git", "sparse-checkout", "init", "--cone"}
	require.Equal(t, expected, args)
	os.Exit(0)
}

func Test_FakeExe_Sparse_Set_Success(t *testing.T) {
	unittest.FakeExeTest(t)
	if os.Getenv(executil.OverrideEnvironmentVariable) == "" {
		return
	}

	args := executil.OriginalArgs()
	expected := []string{"git", "sparse-checkout", "set", "a/b", "c"}
	require.Equal(t, expected, args)
	os.Exit(0)
}

func TestRunSingleBenchmark_HappyPath(t *testing.T) {
	unittest.MediumTest(t)
	ctx := executil.WithFakeTests(context.Background(),
		"Test_FakeExe_Exec_Success",
		"Test_FakeExe_Exec_Success",
		"Test_FakeExe_Exec_Success",
		"Test_FakeExe_Run_Canary_Python_Script_Success",
	)
	workDir := t.TempDir()
	err := os.MkdirAll(filepath.Join(workDir, "git", "canary"), 0755)
	require.NoError(t, err)

	outputFileName, err := runSingleBenchmark(ctx, myFakePythonExe, benchmarkName, benchmarkConfig, "abcdef", workDir)
	require.NoError(t, err)
	require.Contains(t, outputFileName, "canary/results.json")

}

func Test_FakeExe_Exec_Success(t *testing.T) {
	unittest.FakeExeTest(t)
	if os.Getenv(executil.OverrideEnvironmentVariable) == "" {
		return
	}
	os.Exit(0)
}

func Test_FakeExe_Run_Canary_Python_Script_Success(t *testing.T) {
	unittest.FakeExeTest(t)
	if os.Getenv(executil.OverrideEnvironmentVariable) == "" {
		return
	}

	args := executil.OriginalArgs()
	expected := []string{myFakePythonExe, "a/b/benchmark.py", "--githash", "abcdef", "--githash", "abcdef", "--output", "canary/results.json"}
	for i, endsWith := range expected {
		require.Contains(t, args[i], endsWith)
	}
	os.Exit(0)
}

// myWriteCloser is a wrapper that turns a bytes.Buffer from an io.Writer to an io.WriteCloser.
type myWriteCloser struct {
	bytes.Buffer
}

func (*myWriteCloser) Close() error {
	return nil
}

var (
	myError = fmt.Errorf("my fake error")

	fileContents = []byte("{}")
)

func setupForUploadTest(t *testing.T) (context.Context, string) {
	// Create a context with a mockTime so that the GCS upload directory is always the same.
	var mockTime = time.Unix(1649770315, 12).UTC()
	ctx := context.WithValue(context.Background(), now.ContextKey, mockTime)

	// Create a results.json file that will be uploaded by uploadResultsFile.
	tempDir := t.TempDir()
	resultsFile := filepath.Join(tempDir, "results.json")
	err := os.WriteFile(resultsFile, fileContents, 0644)
	require.NoError(t, err)

	return ctx, resultsFile
}

func TestUploadResultsFile_HappyPath(t *testing.T) {
	unittest.MediumTest(t)
	ctx, resultsFile := setupForUploadTest(t)

	var b myWriteCloser

	// Mock out the gcs client.
	gcsClient := mocks.NewGCSClient(t)
	gcsClient.On("FileWriter", testutils.AnyContext, fmt.Sprintf("ingest/2022/04/12/13/%s/results.json", benchmarkName), gcs.FileWriteOptions{
		ContentEncoding: "application/json",
	}).Return(&b)

	err := uploadResultsFile(ctx, gcsClient, benchmarkName, resultsFile)
	require.NoError(t, err)
	require.Equal(t, fileContents, b.Bytes())
}

// myWriteCloser is an io.WriteCloser that always fails on Write.
type myFailingWriteCloser struct {
}

func (*myFailingWriteCloser) Write([]byte) (int, error) {
	return 0, myError
}

func (*myFailingWriteCloser) Close() error {
	return nil
}

func TestUploadResultsFile_WriteCloserFailsToWrite_ReturnsError(t *testing.T) {
	unittest.MediumTest(t)
	ctx, resultsFile := setupForUploadTest(t)

	var b myFailingWriteCloser

	// Mock out the gcs client.
	gcsClient := mocks.NewGCSClient(t)
	gcsClient.On("FileWriter", testutils.AnyContext, fmt.Sprintf("ingest/2022/04/12/13/%s/results.json", benchmarkName), gcs.FileWriteOptions{
		ContentEncoding: "application/json",
	}).Return(&b)

	err := uploadResultsFile(ctx, gcsClient, benchmarkName, resultsFile)
	require.Error(t, err)
}

func TestReadBenchmarksFromFile_NonExistentFile_ReturnsError(t *testing.T) {
	unittest.MediumTest(t)
	filename := filepath.Join(t.TempDir(), "file.json")
	_, err := readBenchMarksFromFile(context.Background(), filename)
	require.Error(t, err)
}

const TestFileContents = `{
    "canary": {
		"repoURL":       "https://skia.googlesource.com/buildbot",
		"checkoutPaths": ["comp-ui"],
		"scriptName":    "comp-ui/benchmark-mock.py",
		"flags": [
			"--browser", "mock"
        ]
	}
}`

func TestReadBenchmarksFromFile_ReadCanaryJSON_ReturnsParsedFile(t *testing.T) {
	unittest.MediumTest(t)
	filename := filepath.Join(t.TempDir(), "file.json")
	err := ioutil.WriteFile(filename, []byte(TestFileContents), 0644)
	require.NoError(t, err)
	benchmarks, err := readBenchMarksFromFile(context.Background(), filename)
	require.NoError(t, err)
	require.Len(t, benchmarks, 1)
	require.Equal(t, "https://skia.googlesource.com/buildbot", benchmarks["canary"].RepoURL)
}

func TestDriverFilenames_DownloadIsFalseButAlternateFilenamesAreNotProvided_ReturnsError(t *testing.T) {
	unittest.SmallTest(t)
	_, _, _, err := driverFilenames(false, "", "")
	require.Error(t, err)
}

func TestDriverFilenames_DownloadIsFalseAndAlternateFilenamesAreProvided_ReturnsAlternateFilename(t *testing.T) {
	unittest.SmallTest(t)
	got1, got2, _, err := driverFilenames(false, alternateChromeDriver, alternateChromeCanaryDriver)
	require.NoError(t, err)
	require.Equal(t, got1, alternateChromeDriver)
	require.Equal(t, got2, alternateChromeCanaryDriver)
}

func TestPopulateBenchmarksWithDrivers(t *testing.T) {
	unittest.SmallTest(t)
	var benchmarks = map[string]*Benchmark{
		// We always run the canary to validate that the whole pipeline works even
		// if the "real" benchmark scripts start to fail.
		"canary": {
			DriverType: NoDriver,
			Flags: []string{
				"--browser", "mock",
			},
		},
		"chrome-stable": {
			DriverType: ChromeStableDriver,
			Flags: []string{
				"--browser", "chrome",
			},
		},
		"chrome-canary": {
			DriverType: ChromeCanaryDriver,
			Flags: []string{
				"--browser", "chrome",
			},
		},
	}

	var expected = map[string]*Benchmark{
		// We always run the canary to validate that the whole pipeline works even
		// if the "real" benchmark scripts start to fail.
		"canary": {
			DriverType: NoDriver,
			Flags: []string{
				"--browser", "mock",
			},
		},
		"chrome-stable": {
			DriverType: ChromeStableDriver,
			Flags: []string{
				"--browser", "chrome",
				"--executable-path", alternateChromeDriver,
			},
		},
		"chrome-canary": {
			DriverType: ChromeCanaryDriver,
			Flags: []string{
				"--browser", "chrome",
				"--executable-path", alternateChromeCanaryDriver,
			},
		},
	}

	populateBenchmarksWithDrivers(benchmarks,
		alternateChromeDriver,
		alternateChromeCanaryDriver,
	)
	require.Equal(t, expected, benchmarks)
}
