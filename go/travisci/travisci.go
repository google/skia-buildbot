package travisci

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"

	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/util"
)

const (
	TRAVIS_API_URL = "https://api.travis-ci.org"

	BUILD_STATE_PASSED = "passed"
	BUILD_STATE_FAILED = "failed"
)

// TravisCI is an object used for iteracting with the Travis CI V3 API.
// Docs are here: https://docs.travis-ci.com/api/#api-v3
type TravisCI struct {
	client      *http.Client
	ctx         context.Context
	repoOwner   string
	repoName    string
	apiURL      string
	accessToken string
}

type Builds struct {
	Builds []*Build `json:"builds"`
}

type Build struct {
	Id                int    `json:"id"`
	Duration          int    `json:"duration"`
	State             string `json:"state"`
	StartedAt         string `json:"started_at"`
	PullRequestNumber int    `json:"pull_request_number"`
}

// NewTravisCI returns a new TravisCI instance. If accessToken is empty then
// unauthenticated API calls are made.
func NewTravisCI(ctx context.Context, repoOwner, repoName, accessToken string) (*TravisCI, error) {
	return &TravisCI{
		client:      httputils.NewTimeoutClient(),
		ctx:         ctx,
		repoOwner:   repoOwner,
		repoName:    repoName,
		apiURL:      TRAVIS_API_URL,
		accessToken: accessToken,
	}, nil
}

func (t *TravisCI) GetPullRequestBuilds(pullNumber int, createdBy string) ([]*Build, error) {
	params := url.Values{}
	params.Add("limit", "10")
	params.Add("sort_by", "id:desc")
	params.Add("event_type", "pull_request")
	params.Add("created_by", createdBy)
	suburl := fmt.Sprintf("/builds?%s", params.Encode())

	builds := &Builds{}
	if err := t.get(suburl, builds); err != nil {
		return nil, fmt.Errorf("API call to %s failed: %s", suburl, err)
	}
	pullRequestBuilds := []*Build{}
	for _, build := range builds.Builds {
		if build.PullRequestNumber == pullNumber {
			pullRequestBuilds = append(pullRequestBuilds, build)
		}
	}
	return pullRequestBuilds, nil
}

// get retrieves the given suburl and populates 'rv' with the result.
func (t *TravisCI) get(suburl string, rv interface{}) error {
	getURL := t.apiURL + suburl
	req, err := http.NewRequest("GET", getURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Travis-API-Version", "3")
	if t.accessToken != "" {
		req.Header.Set("Authorization", fmt.Sprintf("token %s", t.accessToken))
	}

	resp, err := t.client.Do(req)
	if err != nil {
		return fmt.Errorf("Failed to GET %s: %s", getURL, err)
	}
	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("Build not found: %s", getURL)
	}
	if resp.StatusCode >= http.StatusBadRequest {
		return fmt.Errorf("Error retrieving %s: %d %s", getURL, resp.StatusCode, resp.Status)
	}
	defer util.Close(resp.Body)
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("Could not read response body: %s", err)
	}
	if err := json.Unmarshal(body, &rv); err != nil {
		return fmt.Errorf("Failed to decode JSON: %s", err)
	}
	return nil
}
