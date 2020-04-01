// Package swarming downloads and runs the swarming_bot.zip code.
package swarming

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

const metadataURL = "http://metadata:8000/computeMetadata/v1/instance/service-accounts/default/token"

// Bot handles downloading the swarming code and launcing the swarming child
// process.
type Bot struct {
	swarmingBotZipFilename string
	pythonExeFilename      string
	metadataURL            string
	swarmingURL            string
}

const (
	defaultSwarmingServer  = "https://chromium-swarm.appspot.com"
	internalSwarmingServer = "https://chrome-swarming.appspot.com"
	debugSwarmingServer    = "https://chromium-swarm-dev.appspot.com"
)

// New creates a new *Bot instance.
//
// The pythonExe and swarmingBotZip value must be absolute paths.
func New(pythonExeFilename, swarmingBotZipFilename string) *Bot {
	// Figure out where we should be downloading the Python code from.
	host := os.Getenv("SWARMING_BOT_ID")
	swarmingURL := defaultSwarmingServer
	if strings.HasPrefix(host, "skia-i-") {
		swarmingURL = internalSwarmingServer
	} else if strings.HasPrefix(host, "skia-d-") {
		swarmingURL = debugSwarmingServer
	}

	// Note, not /bootstrap, but /bot_code to get the code directly.
	swarmingURL += "/bot_code"

	return &Bot{
		swarmingBotZipFilename: swarmingBotZipFilename,
		pythonExeFilename:      pythonExeFilename,
		swarmingURL:            swarmingURL,
		metadataURL:            metadataURL,
	}
}

// tokenStruct is the form of the JSON data that the metadata endpoint returns.
type tokenStruct struct {
	AccessToken  string `json:"access_token"`
	ExpiresInSec int    `json:"expires_in"`
	TokenType    string `json:"token_type"`
}

// bootstrap downloads the correct swarming bot code.
//
// See also RPI_DESIGN.md
func (b *Bot) bootstrap(ctx context.Context) error {

	// Make sure the directory where the swarming code goes actually exists.
	downloadDir := filepath.Dir(b.swarmingBotZipFilename)
	if _, err := os.Stat(downloadDir); os.IsNotExist(err) {
		if err := os.MkdirAll(downloadDir, 0777); err != nil {
			return skerr.Wrapf(err, "Failed to create download directory %q", downloadDir)
		}
	}

	// Open the file for writing.
	f, err := os.OpenFile(b.swarmingBotZipFilename, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0777)
	if err != nil {
		return skerr.Wrapf(err, "Failed to open swarming bot code file %q", b.swarmingBotZipFilename)
	}
	defer util.Close(f)

	// Request the service account token from the metadata server.
	req, err := http.NewRequest("GET", b.metadataURL, nil)
	if err != nil {
		return skerr.Wrapf(err, "Failed to build request for metadata.")
	}
	req.Header.Add("Metadata-Flavor", "Google")
	client := httputils.NewTimeoutClient()
	resp, err := client.Do(req)
	if err != nil {
		return skerr.Wrapf(err, "Failed to get metadata: %s", err)
	}
	defer util.Close(resp.Body)
	var token tokenStruct
	if err := json.NewDecoder(resp.Body).Decode(&token); err != nil {
		return skerr.Wrapf(err, "Failed to decode metadata token.")
	}

	// Now request the swarming bot code.
	req, err = http.NewRequest("GET", b.swarmingURL, nil)
	if err != nil {
		return skerr.Wrapf(err, "Failed to retrieve swarming bot code.")
	}
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", token.AccessToken))
	resp, err = client.Do(req)
	if err != nil {
		return skerr.Wrapf(err, "Failed to make request for swarming bot code to %q", b.swarmingURL)
	}
	defer util.Close(resp.Body)

	// Copy the bytes into place.
	if _, err := io.Copy(f, req.Body); err != nil {
		return skerr.Wrapf(err, "Failed to copy down swarming bot code.")
	}
	return nil
}

// Launch starts the swarming bot code. This function only returns
// if the bootstrap process to download the swarming bot code fails.
func (b *Bot) Launch(ctx context.Context) error {
	if _, err := os.Stat(b.swarmingBotZipFilename); os.IsNotExist(err) {
		if err := b.bootstrap(ctx); err != nil {
			return err
		}
	}
	for {
		cmd := exec.CommandContext(ctx, b.pythonExeFilename, b.swarmingBotZipFilename, "start_bot")
		sklog.Infof("Starting: %q", cmd.String())

		stderr, err := cmd.StderrPipe()
		if err != nil {
			sklog.Errorf("Failed to get cmd stderr: %s", err)
			continue
		}
		defer util.Close(stderr)

		if err := cmd.Start(); err != nil {
			sklog.Errorf("Failed to start cmd: %s", err)
			continue
		}

		// Copy stderr output into the logs.
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			sklog.Infof("Swarming: " + scanner.Text())
		}

		if err := cmd.Wait(); err != nil {
			sklog.Errorf("Command exited: %s", err)
		}
	}
}
