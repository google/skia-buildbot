package travisci

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	//"strings"
	//"fmt"
	//"github.com/google/go-github/github"
	//"golang.org/x/oauth2"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/util"
)

const (
	TRAVIS_API_URL = "https://api.travis-ci.org"
)

var ()

// TravisCI is an object used for iteracting with the Travis CI V3 API.
type TravisCI struct {
	client    *http.Client
	ctx       context.Context
	repoOwner string
	repoName  string
	apiURL    string
}

type Builds struct {
	Builds []*Build `json:"builds"`
}

type Build struct {
	Id       int    `json:"id"`
	Number   string `json:"number"`
	Duration int    `json:"duration"`
	State    string `json:"state"`
}

// NewTravisCI returns a new TravisCI instance. If accessToken is empty then
// unauthenticated API calls are made.
func NewTravisCI(ctx context.Context, repoOwner, repoName, accessToken string) (*TravisCI, error) {
	return &TravisCI{
		client:    httputils.NewTimeoutClient(),
		ctx:       ctx,
		repoOwner: repoOwner,
		repoName:  repoName,
		apiURL:    TRAVIS_API_URL,
	}, nil
}

func (t *TravisCI) GetLatestBuildDetails(createdBy, pullNumber string) (*Build, error) {
	params := url.Values{}
	params.Add("limit", "1")
	params.Add("sort_by", "id:desc")
	params.Add("event_type", "pull_request")
	params.Add("created_by", createdBy)
	params.Add("pull_request_number", pullNumber)
	suburl := fmt.Sprintf("/builds?%s", params.Encode())

	builds := &Builds{}
	if err := t.get(suburl, builds); err != nil {
		return nil, fmt.Errorf("API call to %s failed: %s", suburl, err)
	}
	if len(builds.Builds) != 1 {
		return nil, fmt.Errorf("Expected 1 Travis CI build but instead got %d for API call to %s", len(builds.Builds), suburl)
	}
	return builds.Builds[0], nil
}

// get retrieves the given suburl and populates 'rv' with the result.
func (t *TravisCI) get(suburl string, rv interface{}) error {
	getURL := t.apiURL + suburl
	req, err := http.NewRequest("GET", getURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Travis-API-Version", "3")
	req.Header.Set("Authorization", "token L9vMIc-AvTSVLJwkdD1O3A")

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
