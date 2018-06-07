// package travisci provides a library for interacting with the Github app
// Travis CI via it's API: https://docs.travis-ci.com/api/#api-v3
//
// Authentication is done using a travis access token as described in
// https://docs.travis-ci.com/api/#authentication

package travisci

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"

	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/util"
)

const (
	TRAVIS_API_URL     = "https://api.travis-ci.org"
	TRAVIS_API_VERSION = "3"

	BUILD_STATE_CREATED = "created"
	BUILD_STATE_PASSED  = "passed"
	BUILD_STATE_FAILED  = "failed"

	FETCH_PULL_REQUESTS_LIMIT = 100
)

// TravisCI is an object used for iteracting with the Travis CI V3 API.
// Docs are here: https://docs.travis-ci.com/api/#api-v3
type TravisCI struct {
	client    *http.Client
	ctx       context.Context
	repoOwner string
	repoName  string
	apiURL    string
	token     string
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

// NewTravisCI returns a new TravisCI instance.
func NewTravisCI(ctx context.Context, repoOwner, repoName, accessToken string) (*TravisCI, error) {
	return &TravisCI{
		client:    httputils.NewTimeoutClient(),
		ctx:       ctx,
		repoOwner: repoOwner,
		repoName:  repoName,
		apiURL:    TRAVIS_API_URL,
		token:     accessToken,
	}, nil
}

func (t *TravisCI) GetPullRequestBuilds(pullNumber int, createdBy string) ([]*Build, error) {
	params := url.Values{}
	params.Add("limit", strconv.Itoa(FETCH_PULL_REQUESTS_LIMIT))
	params.Add("sort_by", "id:desc")
	params.Add("event_type", "pull_request")
	params.Add("created_by", createdBy)
	repositorySlug := fmt.Sprintf("%s/%s", t.repoOwner, t.repoName)
	suburl := fmt.Sprintf("/repo/%s/builds?%s", url.QueryEscape(repositorySlug), params.Encode())

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

func (t *TravisCI) GetBuildURL(buildID int) string {
	return fmt.Sprintf("https://travis-ci.org/flutter/engine/builds/%d", buildID)
}

// get retrieves the given suburl and populates 'rv' with the result.
func (t *TravisCI) get(suburl string, rv interface{}) error {
	getURL := t.apiURL + suburl
	req, err := http.NewRequest("GET", getURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Travis-API-Version", TRAVIS_API_VERSION)
	req.Header.Set("Authorization", fmt.Sprintf("token %s", t.token))

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
