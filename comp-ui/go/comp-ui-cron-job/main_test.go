package main

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/executil"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/now"
	"go.skia.org/infra/go/testutils/unittest"
)

func setupForTest(t *testing.T, h http.HandlerFunc) (string, *httptest.Server, *http.Client) {
	ts := httptest.NewServer(h)
	t.Cleanup(func() {
		ts.Close()
	})

	client := httputils.DefaultClientConfig().WithoutRetries().With2xxOnly().Client()

	return t.TempDir(), ts, client
}

func TestComputeUploadPathFromTime(t *testing.T) {
	unittest.SmallTest(t)
	var mockTime = time.Unix(1649770315, 12).UTC()
	ctx := context.WithValue(context.Background(), now.ContextKey, mockTime)
	assert.Equal(t, "2022/04/12/13", computeUploadPathFromTime(ctx))
}

func TestDownloadPythonScript_HTTPErrorOnRequest_ReturnsError(t *testing.T) {
	unittest.MediumTest(t)
	_, ts, client := setupForTest(t, http.NotFound) // Not Found error will cause download to fail.

	err := downloadPythonScript(context.Background(), ts.URL, "foo.py", client)
	require.Error(t, err)
}

func TestDownloadPythonScript_HappyPath_FileIsDownloadedAndDecodedAndWritten(t *testing.T) {
	unittest.MediumTest(t)

	const body = "this is my mock script"

	workDir, ts, client := setupForTest(t, func(w http.ResponseWriter, r *http.Request) {
		encodedBody := base64.StdEncoding.EncodeToString([]byte(body))
		_, err := w.Write([]byte(encodedBody))
		require.NoError(t, err)
	})

	filename := filepath.Join(workDir, "foo.py")
	err := downloadPythonScript(context.Background(), ts.URL, filename, client)
	require.NoError(t, err)

	b, err := os.ReadFile(filename)
	require.NoError(t, err)
	require.Equal(t, body, string(b))
}

var benchmarkScriptArgs = []string{"--output", "results.json"}

func TestRunBenchMarkScript_ScriptReturnsError_ReturnsError(t *testing.T) {
	unittest.MediumTest(t)

	workDir, _, _ := setupForTest(t, nil)
	ctx := executil.WithFakeTests(context.Background(), "Test_FakeExe_Python_Script_Crashes")

	err := runBenchMarkScript(ctx, benchmarkScriptArgs, workDir)
	require.Error(t, err)
}

func TestRunBenchMarkScript_ScriptSucceeds_DoesNotReturnError(t *testing.T) {
	unittest.MediumTest(t)

	workDir, _, _ := setupForTest(t, nil)
	ctx := executil.WithFakeTests(context.Background(), "Test_FakeExe_Python_Script_Success")

	err := runBenchMarkScript(ctx, benchmarkScriptArgs, workDir)
	require.NoError(t, err)
}

func TestRunSingleBenchMark_HappyPath(t *testing.T) {
	unittest.MediumTest(t)

	body := "this is my mock script"

	workDir, ts, client := setupForTest(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		encodedBody := base64.StdEncoding.EncodeToString([]byte(body))
		_, err := w.Write([]byte(encodedBody))
		require.NoError(t, err)
	}))
	ctx := executil.WithFakeTests(context.Background(), "Test_FakeExe_Python_Script_Success")

	_, err := runSingleBenchMark(ctx, "myBenchmark", benchmark{downloadURL: ts.URL}, "abcd", workDir, client)
	require.NoError(t, err)
}

func Test_FakeExe_Python_Script_Crashes(t *testing.T) {
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
	require.Contains(t, args, scriptExecutable)
	require.Contains(t, args, "--output")
	os.Exit(0)
}
