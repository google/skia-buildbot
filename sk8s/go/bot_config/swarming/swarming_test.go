// Package swarming downloads and runs the swarming_bot.zip code.
package swarming

import (
	"context"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils/unittest"
)

func TestNew_RPIBots(t *testing.T) {
	os.Setenv("SWARMING_BOT_ID", "skia-rpi-test")
	defer os.Unsetenv("SWARMING_BOT_ID")
	const pythonPath = "/usr/bin/python2.7"
	const swarmingBotPath = "/b/s/swarming_bot.zip"
	b := New(pythonPath, swarmingBotPath)
	assert.Equal(t, metadataURL, b.metadataURL)
	assert.Equal(t, pythonPath, b.pythonExeFilename)
	assert.Equal(t, swarmingBotPath, b.swarmingBotZipFilename)
	assert.Contains(t, b.swarmingURL, defaultSwarmingServer)
}

func TestNew_InternalBot(t *testing.T) {
	os.Setenv("SWARMING_BOT_ID", "skia-i-rpi-test")
	const pythonPath = "/usr/bin/python2.7"
	const swarmingBotPath = "/b/s/swarming_bot.zip"
	b := New(pythonPath, swarmingBotPath)
	assert.Equal(t, metadataURL, b.metadataURL)
	assert.Equal(t, pythonPath, b.pythonExeFilename)
	assert.Equal(t, swarmingBotPath, b.swarmingBotZipFilename)
	assert.Contains(t, b.swarmingURL, internalSwarmingServer)
}

func TestNew_DebugBot(t *testing.T) {
	os.Setenv("SWARMING_BOT_ID", "skia-d-rpi-test")
	const pythonPath = "/usr/bin/python2.7"
	const swarmingBotPath = "/b/s/swarming_bot.zip"
	b := New(pythonPath, swarmingBotPath)
	assert.Equal(t, metadataURL, b.metadataURL)
	assert.Equal(t, pythonPath, b.pythonExeFilename)
	assert.Equal(t, swarmingBotPath, b.swarmingBotZipFilename)
	assert.Contains(t, b.swarmingURL, debugSwarmingServer)
}

type cleanupFunc func()

const swarmingBotFakeContents = `Pretend this is Python code.`

func newBotForTest(t *testing.T, metadataHander, botCodeHandler http.HandlerFunc) (*Bot, string, cleanupFunc) {
	// Get a temp dir.
	dir, err := ioutil.TempDir("", "swarming")
	require.NoError(t, err)

	// Create a temp file to stand in for the python executable.
	pythonPath := filepath.Join(dir, "python2.7")
	f, err := os.Create(pythonPath)
	f.WriteString("A stand-in for Python.")
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

	httpTestServer := httptest.NewServer(r)
	cleanup := func() {
		httpTestServer.Close()
	}

	bot := New(pythonPath, swarmingBotPath)

	// Swap out the URLs for ones that point at our local HTTP server.
	bot.metadataURL = httpTestServer.URL + "/metadata"
	bot.swarmingURL = httpTestServer.URL + "/bot_code"

	return bot, swarmingBotPath, cleanup
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
		// Confirm that the flavor header was sent.
		assert.Equal(t, "Google", r.Header.Get("Metadata-Flavor"))
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
