package internal

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"regexp"
	"time"

	"go.skia.org/infra/go/gerrit"
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
	chromiumDashUrl = "https://chromiumdash.appspot.com/fetch_releases"
	// chromeInternalBucket is the bucket to save the build info JSON files.
	chromeInternalBucket = "chrome-perf-non-public"
	// chromeExperimentBucket is the experimental bucket to save the build info JSON files.
	chromeExperimentBucket = "chrome-perf-experiment-non-public"
	// cbbRefInfoPath is the root of the build info files in the bucket.
	cbbRefInfoPath = "cbb_ref_info/%s/%s/%s.json"
	// cbbRefInfoRepo is the root of the build info files in the chromium/src.
	cbbRefInfoRepo = "testing/perf/cbb_ref_info/%s/%s/%s.json"
	// cbbCommitMessage provides a default commit message.
	cbbCommitMessage = "Update CBB autoroll for the builds refs\n\nNo-try: true"
	// clCommitNumber to get CL commit number from `git cl status` output.
	// e.g. "  Cr-Commit-Position: refs/heads/main@{#99999}"
	// match[1] == "99999"
	clCommitNumber = ".*Cr-Commit-Position: refs/heads/main@{#(\\d+)}"
)

var (
	// Keys match the ChromiumDash and Values match the subfolders in the GCS.
	cbbChannels = map[string]string{
		"Dev":    "dev",
		"Stable": "stable",
	}
	cbbPlatforms = map[string]string{
		"Android": "android",
		"Mac":     "mac",
		"Windows": "windows",
	}
)

// getChromiumDashInfo detects new Chrome releases, submits their info to the
// main branch, and returns a commit position.
func GetChromeReleasesInfoActivity(ctx context.Context, otherBrowsers []BuildInfo, isDev bool) (*ChromeReleaseInfo, error) {
	chromeBuilds, err := getChromeBuilds(ctx)
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	var builds []BuildInfo
	builds = append(builds, chromeBuilds...)
	builds = append(builds, otherBrowsers...)

	newBuilds, err := filterBuilds(ctx, builds, isDev)
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	return commitBuildsInfo(ctx, newBuilds, isDev)
}

type buildInfoEx struct {
	buildInfo         *BuildInfo
	comparableVersion []byte
}

// getChromeBuilds gets the latest Chrome builds for each supported channel/platform from ChromiumDash.
func getChromeBuilds(ctx context.Context) ([]BuildInfo, error) {
	resp, err := httputils.GetWithContext(ctx, httpClient, chromiumDashUrl)
	if err != nil {
		sklog.Fatalf("Failed to get ChromiumDash response: %s", err)
	}
	var builds []BuildInfo
	if err := json.NewDecoder(resp.Body).Decode(&builds); err != nil {
		sklog.Fatalf("Invalid ChromiumDash response:%s, err: %s", resp.Body, err)
	}

	// A map to store the latest build for each supported channel/platform.
	// Its keys have the format "stable,mac".
	var latestBuilds map[string]buildInfoEx = make(map[string]buildInfoEx)
	for _, build := range builds {
		if _, found := cbbChannels[build.Channel]; !found {
			continue
		}
		if _, found := cbbPlatforms[build.Platform]; !found {
			continue
		}
		build.Browser = "chrome"
		build.Channel = cbbChannels[build.Channel]
		build.Platform = cbbPlatforms[build.Platform]
		key := build.Channel + "," + build.Platform
		comparableVersion, err := versionToBytes(build.Version)
		if err != nil {
			sklog.Errorf("Chrome with invalid version %s, ignored", build.Version)
			continue
		}
		existingBuild, found := latestBuilds[key]
		if found && bytes.Compare(comparableVersion, existingBuild.comparableVersion) <= 0 {
			continue
		}
		latestBuilds[key] = buildInfoEx{
			buildInfo:         &build,
			comparableVersion: comparableVersion,
		}
	}

	builds = nil
	for _, buildEx := range latestBuilds {
		builds = append(builds, *buildEx.buildInfo)
	}

	return builds, nil
}

// filterBuilds removes the builds if their version hasn't changed from previous run.
func filterBuilds(ctx context.Context, builds []BuildInfo, isDev bool) ([]BuildInfo, error) {
	bucket := chromeInternalBucket
	if isDev {
		bucket = chromeExperimentBucket
	}
	var store, err = NewStore(ctx, bucket, true)
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	var newBuilds []BuildInfo
	for _, build := range builds {
		filePath := fmt.Sprintf(cbbRefInfoPath, build.Browser, build.Channel, build.Platform)
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
				sklog.Infof("Version did not change. store: %v, repo: %v", gcsBuild, build)
				continue
			}
		} else {
			sklog.Infof("No history found for %s", filePath)
		}

		// TODO(b/388894957): We may need to update the GCS after committing.
		jsonData, err := json.MarshalIndent(build, "", "  ")
		if err != nil {
			return nil, skerr.Wrap(err)
		}
		if err := store.WriteFile(filePath, string(jsonData)); err != nil {
			return nil, skerr.Wrap(err)
		}

		newBuilds = append(newBuilds, build)
	}
	return newBuilds, nil
}

// commitBuildsInfo creates JSON files and uploads the associated commit.
func commitBuildsInfo(ctx context.Context, builds []BuildInfo, isDev bool) (*ChromeReleaseInfo, error) {
	sklog.Infof("Builds to commit their info: %v", builds)
	if len(builds) == 0 {
		sklog.Infof("No new build was detected.")
		return nil, nil
	}
	client, err := NewGitChromium(ctx)
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	ci, err := client.gerritClient.CreateChange(client.ctx, "chromium/src", "main", cbbCommitMessage, "", "")
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to create Gerrit change.")
	}
	sklog.Infof("Gerrit change created successfully, change ID: %s", ci.Id)
	for _, build := range builds {
		filename := fmt.Sprintf(cbbRefInfoRepo, build.Browser, build.Channel, build.Platform)
		jsonData, err := json.MarshalIndent(build, "", "  ")
		if err != nil {
			return nil, skerr.Wrapf(err, "Failed to convert %v to JSON", build)
		}
		err = client.gerritClient.EditFile(client.ctx, ci, filename, string(jsonData))
		if err != nil {
			return nil, skerr.Wrapf(err, "Failed to add %s to Gerrit change", filename)
		}
	}

	err = client.gerritClient.PublishChangeEdit(client.ctx, ci)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to publish the change to Gerrit.")
	}
	// Must call GetChange to refresh the ChangeInfo, otherwise SetReview will fail.
	ci, err = client.gerritClient.GetChange(client.ctx, ci.Id)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to refresh change info after publishing edit.")
	}

	if isDev {
		// During dev testing, we create the CL but doesn't submit it.
		return nil, nil
	}

	labels := map[string]int{"Auto-Submit": 1}
	reviewers := []string{"rubber-stamper@appspot.gserviceaccount.com"}
	err = client.gerritClient.SetReview(client.ctx, ci, "", labels, reviewers, "", nil, "", 0, nil)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to set review info on Gerrit.")
	}
	sklog.Infof("Change published to Gerrit, change ID: %v", ci.Issue)

	return waitForSubmitCl(client, ci, builds)
}

// waitForSubmitCl waits for the 'rubber-stamper' to submit uploaded CLs, then
// returns the commit position.
func waitForSubmitCl(client *gitClient, ci *gerrit.ChangeInfo, builds []BuildInfo) (*ChromeReleaseInfo, error) {
	var commitPosition string
	sklog.Infof("Waiting for CL to be submitted.")
	start := time.Now()
	for {
		if time.Now().Sub(start) > ClSubmissionTimeout {
			return nil, fmt.Errorf("waitForSubmitCl timeout!")
		}
		// Refresh the change info to get the latest CL status.
		ci, err := client.gerritClient.GetChange(client.ctx, ci.Id)
		if err != nil {
			return nil, skerr.Wrapf(err, "Failed to refresh change info.")
		}
		if ci.Committed {
			commitHash := ci.Patchsets[len(ci.Patchsets)-1].ID
			commit, err := client.gerritClient.GetCommit(client.ctx, ci.Issue, commitHash)
			if err != nil {
				return nil, skerr.Wrapf(err, "Failed to get commit info")
			}
			re := regexp.MustCompile(clCommitNumber)
			match := re.FindStringSubmatch(commit.Message)
			if len(match) != 2 {
				return nil, fmt.Errorf("Failed to detect Commit Number: %s", commit.Message)
			}
			commitPosition = match[1]
			sklog.Infof("Detected commit number=%s", commitPosition)
			releaseInfo := &ChromeReleaseInfo{
				CommitPosition: commitPosition,
				CommitHash:     commitHash,
				Builds:         builds,
			}
			return releaseInfo, nil
		} else {
			sklog.Infof("CL status: %s", ci.Status)
		}

		time.Sleep(10 * time.Second)
	}
}

// versionToBytes converts a version string into a byte slice that can be lexicographically
// compared. Each component of the version string is converted to a 4-byte big-endian
// representation.
func versionToBytes(version string) ([]byte, error) {
	parts := strings.Split(version, ".")
	if len(parts) != 4 {
		return nil, skerr.Fmt("invalid version format: %s", version)
	}

	versionBytes := make([]byte, 0, 16)
	for _, part := range parts {
		val, err := strconv.ParseUint(part, 10, 32)
		if err != nil {
			return nil, skerr.Wrapf(err, "invalid version component: %s", part)
		}
		buf := make([]byte, 4)
		binary.BigEndian.PutUint32(buf, uint32(val))
		versionBytes = append(versionBytes, buf...)
	}
	return versionBytes, nil
}
