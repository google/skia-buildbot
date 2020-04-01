// Package swarming downloads and runs the swarming_bot.zip code.
package swarming

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
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
