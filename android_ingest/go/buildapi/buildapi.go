package buildapi

import (
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"time"

	"github.com/skia-dev/glog"

	androidbuildinternal "go.skia.org/infra/go/androidbuildinternal/v2beta1"
	"go.skia.org/infra/go/util"
)

const (
	RETRIES        = 5
	PAGE_SIZE      = 100
	SLEEP_DURATION = 5 * time.Second
)

type Build struct {
	BuildId int64
	TS      int64
}

// Get the last buildid stored.
// Scan backwards using the API until that buildid is found.
// Roll up a list of all new buildids, in order.
// Add each one to the repo.

type API struct {
	service *androidbuildinternal.Service
}

func NewAPI(client *http.Client) (*API, error) {
	service, err := androidbuildinternal.New(client)
	if err != nil {
		return nil, fmt.Errorf("Failed to build API: %s", err)
	}

	return &API{
		service: service,
	}, nil
}

// Find all buildIDs with Skia commits from latest build back to endBuildID.
// Pass in the empty string for endBuildID if you just want a range of IDs.
func (a *API) List(branch string, endBuildID int64) ([]Build, error) {
	pageToken := ""
	var err error
	// collect is a map[buildid]timestamp.
	collect := map[int64]int64{}
	for {
		pageToken, err = a.onePage(branch, endBuildID, collect, pageToken)
		if err != nil {
			return nil, err
		}
		glog.Infof("%#v", collect)
		// We've reached the last page when no pageToken is returned.
		if pageToken == "" {
			break
		}
	}
	ret := []Build{}
	keys := []int64{}
	for key, _ := range collect {
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

func (a *API) onePage(branch string, endBuildId int64, collect map[int64]int64, pageToken string) (string, error) {
	if endBuildId == 0 {
		return "", fmt.Errorf("endBuildId must be a non-zero value, got %d", endBuildId)
	}
	for i := 0; i < RETRIES; i++ {
		glog.Infof("Querying for %q %d", branch, endBuildId)
		request := a.service.Build.List().BuildType("submitted").Branch(branch).MaxResults(PAGE_SIZE) // .Fields("builds(buildId,creationTimestamp),nextPageToken")
		request.EndBuildId(fmt.Sprintf("%d", endBuildId))
		if pageToken != "" {
			request.PageToken(pageToken)
		}
		resp, err := request.Do()
		if err != nil {
			glog.Infof("Call failed: %s", err)
			time.Sleep(SLEEP_DURATION)
			continue
		}
		if len(resp.Builds) == 0 {
			glog.Infof("No builds in response.")
			time.Sleep(SLEEP_DURATION)
			continue
		}
		for _, build := range resp.Builds {
			// Convert build.BuildId to int64.
			buildId, err := strconv.ParseInt(build.BuildId, 10, 64)
			if err != nil {
				glog.Errorf("Got an invalid buildid %q: %s ", build.BuildId, err)
				continue
			}
			if ts, ok := collect[buildId]; !ok {
				collect[buildId] = build.CreationTimestamp
			} else {
				// Check if timestamp is earlier than ts we've already recorded.
				if ts < build.CreationTimestamp {
					collect[buildId] = build.CreationTimestamp
				}
			}
		}
		glog.Infof("%#v", collect)
		return resp.NextPageToken, nil
	}
	return "", fmt.Errorf("No valid responses from API after %d requests.", RETRIES)
}
