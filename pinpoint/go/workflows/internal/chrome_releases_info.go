package internal

import (
	"context"
	"encoding/json"

	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/sklog"
)

// BuildInfo fromt the ChromiumDash response.
type BuildInfo struct {
	// Channel contains 'Canary', 'Dev', 'Beta', 'Stable'
	Channel string `json:"channel"`

	// Platform contains the build platform. e.g. 'Windows'
	Platform string `json:"platform"`

	// Version contains the latest Chrome build version e.g. `136.0.7103.153`
	Version string `json:"version"`
}

const (
	// chromiumDashUrl response contains the latest Chrome build versions.
	chromiumDashUrl = "https://chromiumdash.appspot.com/fetch_releases?num=1"
)

// getChromiumDashInfo provides a list of latest Chrome build versions.
func GetChromeReleasesInfoActivity(ctx context.Context) ([]BuildInfo, error) {
	// TODO(b/388894957): Create HTTP Client in the Orchestrator to share.
	client := httputils.NewTimeoutClient()
	resp, err := httputils.GetWithContext(ctx, client, chromiumDashUrl)
	if err != nil {
		sklog.Fatalf("Failed to get ChromiumDash response: %s", err)
	}
	var builds []BuildInfo
	if err := json.NewDecoder(resp.Body).Decode(&builds); err != nil {
		sklog.Fatalf("Invalid ChromiumDash response:%s, err: %s", resp.Body, err)
	}
	return builds, nil
}
