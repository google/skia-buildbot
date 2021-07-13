// Package swarming downloads and runs the swarming_bot.zip code.
package swarming

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/sklog/glog_and_cloud"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/go/util"
)

const metadataURL = "https://example.org"

func setBotSwarmingIDEnvVar(t *testing.T, value string) util.CleanupFunc {
	originalValue := os.Getenv(SwarmingBotIDEnvVar)
	err := os.Setenv(SwarmingBotIDEnvVar, value)
	require.NoError(t, err)
	cleanup := func() {
		err := os.Setenv(SwarmingBotIDEnvVar, originalValue)
		require.NoError(t, err)
	}
	return cleanup
}

func TestNew_CorrectSwarmingURLForRPIBot(t *testing.T) {
	unittest.SmallTest(t)
	cleanup := setBotSwarmingIDEnvVar(t, "skia-rpi-test")
	defer cleanup()
	const pythonPath = "/usr/bin/python2.7"
	const swarmingBotPath = "/b/s/swarming_bot.zip"
	b, err := New(pythonPath, swarmingBotPath, metadataURL)
	require.NoError(t, err)
	assert.Equal(t, metadataURL, b.metadataURL)
	assert.Equal(t, pythonPath, b.pythonExeFilename)
	assert.Equal(t, swarmingBotPath, b.swarmingBotZipFilename)
	assert.Contains(t, b.swarmingURL, defaultSwarmingServer)
}

func TestNew_CorrectSwarmingURLForInternalBot(t *testing.T) {
	unittest.SmallTest(t)
	cleanup := setBotSwarmingIDEnvVar(t, "skia-i-rpi-test")
	defer cleanup()
	const pythonPath = "/usr/bin/python2.7"
	const swarmingBotPath = "/b/s/swarming_bot.zip"
	b, err := New(pythonPath, swarmingBotPath, metadataURL)
	require.NoError(t, err)
	assert.Equal(t, metadataURL, b.metadataURL)
	assert.Equal(t, pythonPath, b.pythonExeFilename)
	assert.Equal(t, swarmingBotPath, b.swarmingBotZipFilename)
	assert.Contains(t, b.swarmingURL, internalSwarmingServer)
}

func TestNew_CorrectSwarmingURLForDebugBot(t *testing.T) {
	unittest.SmallTest(t)
	cleanup := setBotSwarmingIDEnvVar(t, "skia-d-rpi-test")
	defer cleanup()
	const pythonPath = "/usr/bin/python2.7"
	const swarmingBotPath = "/b/s/swarming_bot.zip"
	b, err := New(pythonPath, swarmingBotPath, metadataURL)
	require.NoError(t, err)
	assert.Equal(t, metadataURL, b.metadataURL)
	assert.Equal(t, pythonPath, b.pythonExeFilename)
	assert.Equal(t, swarmingBotPath, b.swarmingBotZipFilename)
	assert.Contains(t, b.swarmingURL, debugSwarmingServer)
}

func TestNew_ErrIfNoSwarmingBotIDEnvVar(t *testing.T) {
	unittest.SmallTest(t)
	cleanup := setBotSwarmingIDEnvVar(t, "")
	defer cleanup()
	const pythonPath = "/usr/bin/python2.7"
	const swarmingBotPath = "/b/s/swarming_bot.zip"
	_, err := New(pythonPath, swarmingBotPath, metadataURL)
	require.Error(t, err)
}

type cleanupFunc func()

const swarmingBotFakeContents = `Pretend this is Python code.`

// newBotForTest returns a new *Bot for testing, the full path to the swarming
// bot code, and a cleanup function to call when tests are complete.
//
// Works by starting an httptest.Server and redirecting the Bot URLs to that
// server.
func newBotForTest(t *testing.T, metadataHander, botCodeHandler http.HandlerFunc) (*Bot, string, cleanupFunc) {
	// Get a temp dir.
	dir, err := ioutil.TempDir("", "swarming")
	require.NoError(t, err)

	// Create a temp file to stand in for the python executable.
	pythonPath := filepath.Join(dir, "python2.7")
	f, err := os.Create(pythonPath)
	_, err = f.WriteString("A stand-in for Python.")
	require.NoError(t, err)
	require.NoError(t, f.Close())

	// Pick a spot in that dir where the swarming bot code should go. With a
	// couple intervening directories to make sure they get created.
	swarmingBotPath := filepath.Join(dir, "b", "s", "swarming_bot.py")

	// Now launch a local HTTP server that will stand in place for both the
	// metadata server and the swarming server.
	r := mux.NewRouter()

	// This endpoint will pretend to be the metadata server.
	r.HandleFunc("/metadata", metadataHander)

	// This endpoint will pretend to be the swarming server.
	r.HandleFunc("/bot_code", botCodeHandler)

	envCleanup := setBotSwarmingIDEnvVar(t, "skia-rpi-test")

	httpTestServer := httptest.NewServer(r)
	cleanup := func() {
		httpTestServer.Close()
		envCleanup()
	}

	bot, err := New(pythonPath, swarmingBotPath, metadataURL)
	require.NoError(t, err)

	// Swap out the URLs for ones that point at our local HTTP server.
	bot.metadataURL = httpTestServer.URL + "/metadata"
	bot.swarmingURL = httpTestServer.URL + "/bot_code"

	return bot, swarmingBotPath, cleanup
}

// newBotForTestWithSuccessHandlers is just like newBotForTest, but we also
// automatically set the handlers for the happy path.
func newBotForTestWithSuccessHandlers(t *testing.T) (*Bot, cleanupFunc) {
	metadataHandler := func(w http.ResponseWriter, r *http.Request) {
		_, err := w.Write([]byte(`{"access_token":"123"}`))
		assert.NoError(t, err)
	}
	botCodeHandler := func(w http.ResponseWriter, r *http.Request) {
		_, err := w.Write([]byte(swarmingBotFakeContents))
		assert.NoError(t, err)
	}
	bot, _, cleanup := newBotForTest(t, metadataHandler, botCodeHandler)
	return bot, cleanup
}

func TestBootstrap_Success(t *testing.T) {
	unittest.SmallTest(t)
	metadataHandler := func(w http.ResponseWriter, r *http.Request) {
		// Confirm that the flavor header was sent.
		assert.Equal(t, "Google", r.Header.Get("Metadata-Flavor"))
		_, err := w.Write([]byte(`{"access_token":"123"}`))
		assert.NoError(t, err)
	}
	botCodeHandler := func(w http.ResponseWriter, r *http.Request) {
		// Confirm that the authorization header was sent correctly.
		assert.Equal(t, "Bearer 123", r.Header.Get("Authorization"))
		_, err := w.Write([]byte(swarmingBotFakeContents))
		assert.NoError(t, err)
	}

	bot, swarmingBotPath, cleanup := newBotForTest(t, metadataHandler, botCodeHandler)
	defer cleanup()

	require.NoError(t, bot.bootstrap(context.Background()))

	// Confirm that we downloaded the swarming bot contents correctly.
	b, err := ioutil.ReadFile(swarmingBotPath)
	require.NoError(t, err)
	assert.Equal(t, swarmingBotFakeContents, string(b))
}

func TestBootstrap_ErrOnMetadataRequestFail(t *testing.T) {
	unittest.SmallTest(t)
	metadataHandler := func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
	botCodeHandler := func(w http.ResponseWriter, r *http.Request) {
		assert.Fail(t, "Should never get here.")
	}

	bot, _, cleanup := newBotForTest(t, metadataHandler, botCodeHandler)
	defer cleanup()

	err := bot.bootstrap(context.Background())
	assert.Contains(t, err.Error(), "Metadata bad status code")
}

func TestBootstrap_ErrOnMetadataResponseNotJSON(t *testing.T) {
	unittest.SmallTest(t)
	metadataHandler := func(w http.ResponseWriter, r *http.Request) {
		// Confirm that the flavor header was sent.
		assert.Equal(t, "Google", r.Header.Get("Metadata-Flavor"))
		_, err := w.Write([]byte(`This is not valid JSON.`))
		assert.NoError(t, err)
	}
	botCodeHandler := func(w http.ResponseWriter, r *http.Request) {
		assert.Fail(t, "Should never get here.")
	}

	bot, _, cleanup := newBotForTest(t, metadataHandler, botCodeHandler)
	defer cleanup()

	err := bot.bootstrap(context.Background())
	assert.Contains(t, err.Error(), "Failed to decode metadata")
}

func TestBootstrap_ErrOnSwarmingRequestFail(t *testing.T) {
	unittest.SmallTest(t)
	metadataHandler := func(w http.ResponseWriter, r *http.Request) {
		_, err := w.Write([]byte(`{"access_token":"123"}`))
		assert.NoError(t, err)
	}
	botCodeHandler := func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}

	bot, _, cleanup := newBotForTest(t, metadataHandler, botCodeHandler)
	defer cleanup()

	err := bot.bootstrap(context.Background())
	assert.Contains(t, err.Error(), "Swarming server bad status code")
}

const swarmingbotFakeStderrOutput = "This is a line of logging output."

func TestRunSwarmingCommand_ReturnsNilOnExitCodeZero(t *testing.T) {
	unittest.SmallTest(t)

	bot, cleanup := newBotForTestWithSuccessHandlers(t)
	defer cleanup()

	// Swap out execCommandContext with an executable that we know exists and
	// returns a zero exit code. See
	// https://npf.io/2015/06/testing-exec-command/ for details on how this
	// works.
	execCommandContext = fakeExecCommandContext_ExitCodeZero
	defer func() {
		execCommandContext = exec.CommandContext
	}()

	// We also want to test that capturing the stderr output of swarming is
	// captured and emitted as sklog lines. To captures sklog we need to capture
	// this programs stderr output.

	// First create a temp file.
	f, err := ioutil.TempFile("", "swarming")
	require.NoError(t, err)

	// Now swap out os.Stderr with our file.
	tmp := os.Stderr
	os.Stderr = f

	// Swap everything back in place when we leave.
	defer func() {
		os.Stderr = tmp
		util.Close(f)
		err := os.Remove(f.Name())
		assert.NoError(t, err)
	}()

	// Now tell sklog to emit to stderr.
	glog_and_cloud.SetLogger(glog_and_cloud.NewStdErrCloudLogger(glog_and_cloud.SLogStderr))
	ctx := context.Background()

	// Run bootstrap so everything is in place for calling runSwarmingCommand.
	err = bot.bootstrap(ctx)
	require.NoError(t, err)

	err = bot.runSwarmingCommand(ctx)
	require.NoError(t, err)

	// Check the output of sklog.
	_, err = f.Seek(0, 0)
	require.NoError(t, err)
	b, err := ioutil.ReadAll(f)
	require.NoError(t, err)
	assert.Contains(t, string(b), swarmingbotFakeStderrOutput) // See TestFakeSwarmingExecutable_ExitCodeZero
}

// An exec.CommandContext fake that actually executes another test in this file
// TestFakeSwarmingExecutable_ExitCodeZero instead of the requested exe.
func fakeExecCommandContext_ExitCodeZero(ctx context.Context, command string, args ...string) *exec.Cmd {
	extendedArgs := []string{"-test.run=TestFakeSwarmingExecutable_ExitCodeZero", "--", command}
	extendedArgs = append(extendedArgs, args...)
	cmd := exec.CommandContext(ctx, os.Args[0], extendedArgs...)
	cmd.Env = []string{"EMULATE_SWARMING_BOT_EXECUTABLE=1"}
	return cmd
}

// TestFakeSwarmingExecutable_ExitCodeZero is used by fakeExecCommandContext_ExitCodeZero.
func TestFakeSwarmingExecutable_ExitCodeZero(t *testing.T) {
	unittest.SmallTest(t)
	if os.Getenv("EMULATE_SWARMING_BOT_EXECUTABLE") != "1" {
		return
	}
	// Confirm the args are getting passed along.
	if os.Args[len(os.Args)-1] != "start_bot" {
		sklog.Fatal("Missing start_bot in os.Args.")
	}
	// Printf on stderr, which should appear in the callers logs.
	fmt.Fprintf(os.Stderr, swarmingbotFakeStderrOutput)
	os.Exit(0)
}

const nonZeroExitCode = 17

func TestRunSwarmingCommand_ReturnsErrorOnNonZeroExitCode(t *testing.T) {
	unittest.SmallTest(t)

	bot, cleanup := newBotForTestWithSuccessHandlers(t)
	defer cleanup()

	// Swap out execCommandContext with an executable that we know exists and
	// returns a non-zero exit code. See
	// https://npf.io/2015/06/testing-exec-command/ for details on how this
	// works.
	execCommandContext = fakeExecCommandContext_ExitCodeNonZero
	defer func() {
		execCommandContext = exec.CommandContext
	}()

	ctx := context.Background()
	err := bot.bootstrap(ctx)
	require.NoError(t, err)

	err = bot.runSwarmingCommand(ctx)
	assert.Contains(t, err.Error(), fmt.Sprintf("exit status %d", nonZeroExitCode))
}

// An exec.CommandContext fake that actually executes another test in this file
// TestFakeSwarmingExecutable_ExitCodeNonZero instead of the requested exe.
func fakeExecCommandContext_ExitCodeNonZero(ctx context.Context, command string, args ...string) *exec.Cmd {
	extendedArgs := []string{"-test.run=TestFakeSwarmingExecutable_ExitCodeNonZero", "--", command}
	extendedArgs = append(extendedArgs, args...)
	cmd := exec.CommandContext(ctx, os.Args[0], extendedArgs...)
	cmd.Env = []string{"EMULATE_SWARMING_BOT_EXECUTABLE=1"}
	return cmd
}

// TestFakeSwarmingExecutable_ExitCodeNonZero is used by fakeExecCommandContext_ExitCodeNonZero.
func TestFakeSwarmingExecutable_ExitCodeNonZero(t *testing.T) {
	unittest.SmallTest(t)
	if os.Getenv("EMULATE_SWARMING_BOT_EXECUTABLE") != "1" {
		return
	}
	os.Exit(nonZeroExitCode)
}

func TestLaunch_IsCancellable(t *testing.T) {
	unittest.SmallTest(t)

	// Create a new Bot.
	bot, cleanup := newBotForTestWithSuccessHandlers(t)
	defer cleanup()

	// Call bootstrap now so when we do call Launch() it won't call bootstrap
	// and we'll fall directly into the for {} loop.
	err := bot.bootstrap(context.Background())
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err = bot.Launch(ctx)
	assert.Contains(t, err.Error(), "Context was cancelled")
}
