package internal

import (
	"context"
	"encoding/json"
	"fmt"

	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
)

// BuildInfo fromt the ChromiumDash response.
type BuildInfo struct {
	// Browser contains 'Chrome', 'Edge', 'Safari'
	Browser string `json:"browser"`

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
	// chromeInternalBucket is the bucket to save the build info JSON files.
	chromeInternalBucket = "chrome-perf-experiment-non-public"
	// cbbRefInfoPath is the root of the build info files in the bucket.
	cbbRefInfoPath = "cbb_ref_info/chrome/%s/%s.json"
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
	return filterBuilds(ctx, builds)
}

func filterBuilds(ctx context.Context, builds []BuildInfo) ([]BuildInfo, error) {
	// Keys match the ChromiumDash and Values match the subfolders in the GCS.
	var cbbChannels = map[string]string{
		"Dev":    "dev",
		"Stable": "stable",
	}
	var cbbPlatforms = map[string]string{
		"Android": "Android",
		"Mac":     "macOS",
		"Windows": "Windows",
	}
	var store, err = NewStore(ctx, chromeInternalBucket, true)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	var targetBuilds []BuildInfo
	for _, build := range builds {
		if _, found := cbbChannels[build.Channel]; !found {
			continue
		}
		if _, found := cbbPlatforms[build.Platform]; !found {
			continue
		}
		filePath := fmt.Sprintf(cbbRefInfoPath, cbbChannels[build.Channel], cbbPlatforms[build.Platform])
		if store.Exists(filePath) {
			var content, err = store.GetFileContent(filePath)
			if err != nil {
				return nil, skerr.Wrap(err)
			}
			var gcsBuild BuildInfo
			if err := json.Unmarshal(content, &gcsBuild); err != nil {
				return nil, skerr.Wrap(err)
			}
			if build.Version == gcsBuild.Version {
				continue
			}
		}
		build.Browser = "Chrome"

		// TODO(b/388894957): We may need to update the GCS after committing.
		jsonData, err := json.Marshal(build)
		if err != nil {
			return nil, skerr.Wrap(err)
		}
		if err := store.WriteFile(filePath, string(jsonData)); err != nil {
			return nil, skerr.Wrap(err)
		}

		targetBuilds = append(targetBuilds, build)
	}
	return targetBuilds, nil
}
