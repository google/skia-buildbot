// buildapi allows querying the Android Build API to find buildid's for a specific branch.
package buildapi

import (
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"time"

	androidbuildinternal "go.skia.org/infra/go/androidbuildinternal/v2beta1"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

const (
	// PAGE_SIZE is the number of builds to request per call.
	PAGE_SIZE = 100

	// RETRIES this many times before giving up on a call.
	RETRIES = 5

	// SLEEP_DURATION is the time to sleep between failed calls.
	SLEEP_DURATION = 5 * time.Second
)

// Build represents a single build at the earliest timestamp that it was committed
// to any target. I.e. we find all the timestamps of when the buildid landed in
// all the targets and then take the earliest value.
type Build struct {
	BuildId int64
	TS      int64
}

// API allows finding all the Build's for a given branch.
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

// List returns all buildIDs with Skia commits for 'branch' from latest build back to endBuildId.
//
// The value of endBuildId must not be zero.
func (a *API) List(branch string, endBuildId int64) ([]Build, error) {
	if endBuildId == 0 {
		return nil, fmt.Errorf("endBuildId must be a non-zero value, got %d", endBuildId)
	}
	pageToken := ""
	var err error
	// collect is a map[buildid]timestamp.
	collect := map[int64]int64{}
	for {
		pageToken, err = a.onePage(branch, endBuildId, collect, pageToken)
		if err != nil {
			return nil, err
		}
		// We've reached the last page when no pageToken is returned.
		if pageToken == "" {
			break
		}
	}

	// Turn 'collect' into a []Build, sorted by ascending timestamp.
	ret := []Build{}
	keys := []int64{}
	for key := range collect {
		keys = append(keys, key)
	}
	sort.Sort(util.Int64Slice(keys))
	for _, key := range keys {
		ret = append(ret, Build{
			BuildId: key,
			TS:      collect[key],
		})
	}
	return ret, nil
}

// onePage does the work of reading and parsing one page of the API response to androidbuildinternal.
//
// The 'collect' map[int64]int64, which is a map[buildid]timestamp, is populated with results
// after a successful call into the api.
func (a *API) onePage(branch string, endBuildId int64, collect map[int64]int64, pageToken string) (string, error) {
	for i := 0; i < RETRIES; i++ {
		sklog.Infof("Querying for %q %d", branch, endBuildId)
		request := a.service.Build.List().BuildType("submitted").Branch(branch).MaxResults(PAGE_SIZE).Fields("builds(buildId,creationTimestamp),nextPageToken")
		request.EndBuildId(fmt.Sprintf("%d", endBuildId))
		if pageToken != "" {
			request.PageToken(pageToken)
		}
		resp, err := request.Do()
		if err != nil {
			sklog.Infof("Call failed: %s", err)
			time.Sleep(SLEEP_DURATION)
			continue
		}
		for _, build := range resp.Builds {
			// Convert build.BuildId to int64.
			buildId, err := strconv.ParseInt(build.BuildId, 10, 64)
			if err != nil {
				sklog.Errorf("Got an invalid buildid %q: %s ", build.BuildId, err)
				continue
			}
			if ts, ok := collect[buildId]; !ok {
				collect[buildId] = build.CreationTimestamp / 1000
			} else {
				// Check if timestamp is earlier than ts we've already recorded.
				if ts < build.CreationTimestamp/1000 {
					collect[buildId] = build.CreationTimestamp / 1000
				}
			}
		}
		return resp.NextPageToken, nil
	}
	return "", fmt.Errorf("No valid responses from API after %d requests.", RETRIES)
}
