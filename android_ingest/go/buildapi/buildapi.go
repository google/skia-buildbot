// buildapi allows querying the Android Build API to find buildid's.
package buildapi

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	androidbuildinternal "go.skia.org/infra/go/androidbuildinternal/v2beta1"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
)

const (
	// PAGE_SIZE is the number of builds to request per call.
	PAGE_SIZE = 100

	// RETRIES this many times before giving up on a call.
	RETRIES = 5

	// SLEEP_DURATION is the time to sleep between failed calls.
	SLEEP_DURATION = 5 * time.Second
)

// API allows finding all the Build's.
type API struct {
	service *androidbuildinternal.Service
}

// NewAPI returns a new *API.
//
// The 'client' must be authenticated to use the androidbuildinternal api.
func NewAPI(client *http.Client) (*API, error) {
	service, err := androidbuildinternal.New(client)
	if err != nil {
		return nil, fmt.Errorf("Failed to build API: %s", err)
	}

	return &API{
		service: service,
	}, nil
}

// Build represents a single build and its creation timestamp.
type Build struct {
	BuildId int64
	TS      int64
}

// GetMostRecentBuildID returns the most recent build id and its timestamp.
func (a *API) GetMostRecentBuildID() (int64, int64, error) {
	request := a.service.Build.List().BuildType("submitted").MaxResults(1).Fields("builds(buildId, creationTimestamp)")

	resp, err := request.Do()
	if err != nil {
		sklog.Infof("Call failed: %s", err)
		time.Sleep(SLEEP_DURATION)
		return -1, -1, skerr.Wrap(err)
	}
	sklog.Infof("Got %d items.", len(resp.Builds))
	build := resp.Builds[0]
	// Convert build.BuildId to int64.
	buildId, err := strconv.ParseInt(build.BuildId, 10, 64)
	if err != nil {
		return -1, -1, skerr.Wrapf(err, "Got an invalid buildid %q", build.BuildId)
	}
	timestamp := build.CreationTimestamp / 1000

	return buildId, timestamp, nil
}

// GetBranchFromBuildID returns the branch name for the given build id.
func (a *API) GetBranchFromBuildID(buildID int64) (string, error) {
	request := a.service.Build.List().BuildId(fmt.Sprintf("%d", buildID)).MaxResults(1).Fields("builds/branch")
	resp, err := request.Do()
	if err != nil {
		return "", skerr.Wrap(err)
	}
	if len(resp.Builds) < 1 {
		return "", skerr.Fmt("Did not receive enough results for buildID: %d", buildID)
	}
	return resp.Builds[0].Branch, nil
}
