// Package buildbucket provides tools for interacting with the buildbucket API.
package buildbucket

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"go.skia.org/infra/go/util"
)

const (
	apiUrl = "https://cr-buildbucket.appspot.com/_ah/api/buildbucket/v1"
)

var (
	DEFAULT_SCOPES = []string{
		"https://www.googleapis.com/auth/userinfo.email",
		"https://www.googleapis.com/auth/userinfo.profile",
	}
)

type buildBucketAuthor struct {
	Email string `json:"email"`
}

type buildBucketChange struct {
	Author     *buildBucketAuthor `json:"author"`
	Repository string             `json:"repo_url"`
	Revision   string             `json:"revision"`
}

type buildBucketError struct {
	Message string `json:"message"`
	Reason  string `json:"reason"`
}

type buildBucketParameters struct {
	BuilderName string               `json:"builder_name"`
	Changes     []*buildBucketChange `json:"changes"`
	Properties  map[string]string    `json:"properties"`
	Swarming    *swarming            `json:"swarming,omitempty"`
}

type swarming struct {
	OverrideBuilderCfg swarmingOverrides `json:"override_builder_cfg"`
}

type swarmingOverrides struct {
	Dimensions []string `json:"dimensions"`
}

type buildBucketRequest struct {
	Bucket         string `json:"bucket"`
	ParametersJSON string `json:"parameters_json"`
}

type buildBucketResponse struct {
	Build *Build            `json:"build"`
	Error *buildBucketError `json:"error"`
	Kind  string            `json:"kind"`
	Etag  string            `json:"etag"`
}

// Build is a struct containing information about a build in BuildBucket.
type Build struct {
	Bucket                 string `json:"bucket"`
	CompletedTimestamp     string `json:"completed_ts"`
	CreatedBy              string `json:"created_by"`
	CreatedTimestamp       string `json:"created_ts"`
	FailureReason          string `json:"failure_reason"`
	Id                     string `json:"id"`
	Url                    string `json:"url"`
	ParametersJson         string `json:"parameters_json"`
	Result                 string `json:"result"`
	ResultDetailsJson      string `json:"result_details_json"`
	Status                 string `json:"status"`
	StatusChangedTimestamp string `json:"status_changed_ts"`
	UpdatedTimestamp       string `json:"updated_ts"`
	UtcNowTimestamp        string `json:"utcnow_ts"`
}

// Client is used for interacting with the BuildBucket API.
type Client struct {
	*http.Client
}

// NewClient returns an authenticated Client instance.
func NewClient(c *http.Client) *Client {
	return &Client{c}
}

// RequestBuild adds a request for the given build. The swarmingBotId parameter
// may be the empty string, in which case the build may run on any bot.
func (c *Client) RequestBuild(builder, master, commit, repo, author, swarmingBotId string) (*Build, error) {
	p := buildBucketParameters{
		BuilderName: builder,
		Changes: []*buildBucketChange{
			&buildBucketChange{
				Author: &buildBucketAuthor{
					Email: author,
				},
				Repository: repo,
				Revision:   commit,
			},
		},
		Properties: map[string]string{
			"reason": "Triggered by SkiaScheduler",
		},
	}
	if swarmingBotId != "" {
		p.Swarming = &swarming{
			OverrideBuilderCfg: swarmingOverrides{
				Dimensions: []string{
					fmt.Sprintf("id:%s", swarmingBotId),
				},
			},
		}
	}
	jsonParams, err := json.Marshal(p)
	if err != nil {
		return nil, err
	}
	body := buildBucketRequest{
		Bucket:         fmt.Sprintf("master.%s", master),
		ParametersJSON: string(jsonParams),
	}
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	url := apiUrl + "/builds"
	req, err := http.NewRequest("PUT", url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		defer util.Close(resp.Body)
		b, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("Failed to schedule build (code %s); couldn't read response body: %v", resp.Status, err)
		}
		return nil, fmt.Errorf("Response code is %s. Response body:\n%s", resp.Status, string(b))
	}
	defer util.Close(resp.Body)
	var res buildBucketResponse
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, fmt.Errorf("Failed to decode response body: %v", err)
	}
	if res.Error != nil {
		return nil, fmt.Errorf("Failed to schedule build: %s", res.Error.Message)
	}
	return res.Build, nil
}

// GetBuild retrieves the build with the given ID.
func (c *Client) GetBuild(buildId string) (*Build, error) {
	url := apiUrl + "/builds/" + buildId + "?alt=json"
	resp, err := c.Get(url)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Request got response %s", resp.Status)
	}
	var result struct {
		Build *Build `json:"build"`
		Etag  string `json:"etag"`
		Kind  string `json:"kind"`
	}
	defer util.Close(resp.Body)
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return result.Build, nil
}

// getOnePage retrieves one page of search results.
func (c *Client) getOnePage(url string) ([]*Build, string, error) {
	resp, err := c.Get(url)
	if err != nil {
		return nil, "", err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("Request got response %s", resp.Status)
	}
	var result struct {
		Builds     []*Build `json:"builds"`
		Etag       string   `json:"etag"`
		Kind       string   `json:"kind"`
		NextCursor string   `json:"next_cursor"`
	}
	defer util.Close(resp.Body)
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, "", err
	}
	return result.Builds, result.NextCursor, nil
}

// Search retrieves results based on the given criteria.
func (c *Client) Search(url string) ([]*Build, error) {
	rv := []*Build{}
	cursor := ""
	for {
		newUrl := url
		if cursor != "" {
			newUrl += fmt.Sprintf("&start_cursor=%s", cursor)
		}
		var builds []*Build
		var err error
		builds, cursor, err = c.getOnePage(newUrl)
		if err != nil {
			return nil, err
		}
		rv = append(rv, builds...)
		if cursor == "" {
			break
		}
	}
	return rv, nil
}

// GetTrybotsForCL retrieves trybot results for the given CL.
func (c *Client) GetTrybotsForCL(issueID int64, patchsetID int64) ([]*Build, error) {
	url := fmt.Sprintf("%s/search?tag=buildset%%3Apatch%%2Frietveld%%2Fcodereview.chromium.org%%2F%d%%2F%d", apiUrl, issueID, patchsetID)
	return c.Search(url)
}
