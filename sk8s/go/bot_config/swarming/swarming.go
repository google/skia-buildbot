// Package swarming downloads and runs the swarming_bot.zip code.
package swarming

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

var (
	// execCommandContext captures exec.CommandContext, which makes testing Bot
	// easier. See https://npf.io/2015/06/testing-exec-command/.
	execCommandContext = exec.CommandContext
)

// Bot handles downloading the swarming code and launching the swarming child
// process.
type Bot struct {
	swarmingBotZipFilename string
	pythonExeFilename      string
	metadataURL            string
	swarmingURL            string
	swarmingBotID          string
}

const (
	defaultSwarmingServer  = "https://chromium-swarm.appspot.com"
	internalSwarmingServer = "https://chrome-swarming.appspot.com"
	debugSwarmingServer    = "https://chromium-swarm-dev.appspot.com"

	// SwarmingBotIDEnvVar is the swarming bot id environment variable name. See
	// https://chromium.googlesource.com/infra/luci/luci-py.git/+doc/master/appengine/swarming/doc/Magic-Values.md#task-runtime-environment-variables
	SwarmingBotIDEnvVar = "SWARMING_BOT_ID"

	// KubernetesImageEnvVar is the environment variable that contains the
	// daemonset image name.
	//
	// See https://skia.googlesource.com/k8s-config/+/refs/heads/master/skolo-rack4/rpi-swarming-daemonset.yaml
	// where IMAGE_NAME is set.
	KubernetesImageEnvVar = "IMAGE_NAME"
)

// New creates a new *Bot instance.
//
// The pythonExe and swarmingBotZip values must be absolute paths.
func New(pythonExeFilename, swarmingBotZipFilename, metadataURL string) (*Bot, error) {
	// Figure out where we should be downloading the Python code from.
	host := os.Getenv(SwarmingBotIDEnvVar)
	if host == "" {
		return nil, skerr.Fmt("Env variable %q must be set.", SwarmingBotIDEnvVar)
	}
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
		swarmingBotID:          host,
	}, nil
}

// tokenStruct is the form of the JSON data that the metadata endpoint returns,
// with just the fields we care about.
type tokenStruct struct {
	AccessToken string `json:"access_token"`
}

// bootstrap downloads the correct swarming bot code.
func (b *Bot) bootstrap(ctx context.Context) error {
	// Make sure the directory where the swarming code goes actually exists.
	downloadDir := filepath.Dir(b.swarmingBotZipFilename)
	if _, err := os.Stat(downloadDir); os.IsNotExist(err) {
		if err := os.MkdirAll(downloadDir, 0777); err != nil {
			return skerr.Wrapf(err, "Failed to create download directory %q", downloadDir)
		}
	}

	// Request the service account token from the metadata server.
	req, err := http.NewRequest("GET", b.metadataURL, nil)
	if err != nil {
		return skerr.Wrapf(err, "Failed to build request for metadata.")
	}
	req.Header.Add("Metadata-Flavor", "Google")
	client := httputils.NewTimeoutClient()
	resp, err := client.Do(req)
	if err != nil {
		return skerr.Wrapf(err, "Failed to get metadata")
	}
	if resp.StatusCode != 200 {
		return skerr.Fmt("Metadata bad status code: %d - %s", resp.StatusCode, resp.Status)
	}
	tokenBytes, err := ioutil.ReadAll(resp.Body)
	if err := resp.Body.Close(); err != nil {
		return skerr.Wrapf(err, "Failed to close metadata response.")
	}
	var token tokenStruct
	if err := json.Unmarshal(tokenBytes, &token); err != nil {
		tokenString := string(tokenBytes)
		n := len(tokenString)
		if n > 10 {
			n = 10
		}
		return skerr.Wrapf(err, "Failed to decode metadata token starting with: %q", string(tokenBytes)[:n])
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
	if resp.StatusCode != 200 {
		return skerr.Fmt("Swarming server bad status code: %d - %s", resp.StatusCode, resp.Status)
	}

	// Copy the bytes into place.
	err = util.WithWriteFile(b.swarmingBotZipFilename, func(w io.Writer) error {
		_, err := io.Copy(w, resp.Body)
		return skerr.Wrap(err)
	})
	if err != nil {
		return skerr.Wrapf(err, "Failed to copy down swarming bot code.")
	}
	return nil
}

// runSwarmingCommand runs the swarming_bot.zip code.
//
// It also captures all stderr output and feeds that into logs.
//
// If swarming_bot exits with a 0 exit code then runSwarmingCommand returns a
// nil, otherwise an error with the exit code is returned.
func (b *Bot) runSwarmingCommand(ctx context.Context) error {
	// Note we use execCommandContext as opposed to exec.CommandContext, which
	// allows us to replace execCommandContext during tests.
	cmd := execCommandContext(ctx, b.pythonExeFilename, b.swarmingBotZipFilename, "start_bot")

	sklog.Infof("Starting: %q", cmd.String())

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return skerr.Wrapf(err, "Failed to get cmd stderr.")
	}
	defer util.Close(stderr)

	if err := cmd.Start(); err != nil {
		return skerr.Wrapf(err, "Failed to start cmd.")
	}

	// Copy stderr output into the logs.
	scanner := bufio.NewScanner(stderr)
	for scanner.Scan() {
		text := scanner.Text()
		sklog.Infof("Swarming: " + text)
	}

	return skerr.Wrapf(cmd.Wait(), "Command exited.")
}

// Launch starts the swarming bot code. This function only returns if the
// bootstrap process to download the swarming bot code fails or if the context
// is cancelled.
func (b *Bot) Launch(ctx context.Context) error {
	liveness := metrics2.NewLiveness("bot_config_swarming_sub_process", map[string]string{"machine": b.swarmingBotID})
	if _, err := os.Stat(b.swarmingBotZipFilename); os.IsNotExist(err) {
		if err := b.bootstrap(ctx); err != nil {
			return skerr.Wrapf(err, "Bootstrap failed.")
		}
	}
	for {
		if ctx.Err() != nil {
			return skerr.Wrapf(ctx.Err(), "Context was cancelled.")
		}
		if err := b.runSwarmingCommand(ctx); err != nil {
			sklog.Errorf("Swarming command exited: %s", err)
		}
		liveness.Reset()
	}
}
