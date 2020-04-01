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

func TestBootstrap_Success(t *testing.T) {
	unittest.SmallTest(t)

	const swarmingBotFakeContents = `Pretend this is Python code.`

	// Get a temp dir.
	dir, err := ioutil.TempDir("", "swarming")
	require.NoError(t, err)

	// Create a temp file to stand in for the python executable.
	pythonPath := filepath.Join(dir, "python2.7")
	f, err := os.Create(pythonPath)
	f.WriteString("A stand-in for Python.")
	require.NoError(t, f.Close())

	// Pick a spot in that dir where the swarming bot code should go.
	swarmingBotPath := filepath.Join(dir, "b", "s", "swarming_bot.py")

	// Now launch a local server that will stand in place for both the metadata
	// server and the swarming server.
	r := mux.NewRouter()
	r.HandleFunc("/metadata", func(w http.ResponseWriter, r *http.Request) {
		// Confirm that the flavor header was sent.
		assert.Equal(t, "Google", r.Header.Get("Metadata-Flavor"))
		_, err := w.Write([]byte(`{"access_token":"123"}`))
		assert.NoError(t, err)
	})
	r.HandleFunc("/bot_code", func(w http.ResponseWriter, r *http.Request) {
		// Confirm that the authorization header was sent correctly.
		assert.Equal(t, "Bearer 123", r.Header.Get("Authorization"))
		_, err := w.Write([]byte(swarmingBotFakeContents))
		assert.NoError(t, err)
	})

	httpTestServer := httptest.NewServer(r)
	defer httpTestServer.Close()

	ctx := context.Background()
	bot := New(pythonPath, swarmingBotPath)
	bot.metadataURL = httpTestServer.URL + "/metadata"
	bot.swarmingURL = httpTestServer.URL + "/bot_code"
	err = bot.bootstrap(ctx)
	require.NoError(t, err)

	b, err := ioutil.ReadFile(swarmingBotPath)
	assert.Equal(t, swarmingBotFakeContents, string(b))
}
