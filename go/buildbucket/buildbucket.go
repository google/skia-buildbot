// Package buildbucket provides tools for interacting with the buildbucket API.
package buildbucket

import (
	"bytes"
	"encoding/json"
	"fmt"
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

type buildBucketParameters struct {
	BuilderName string               `json:"builder_name"`
	Changes     []*buildBucketChange `json:"changes"`
	Properties  map[string]string    `json:"properties"`
}

type buildBucketRequest struct {
	Bucket         string `json:"bucket"`
	ParametersJSON string `json:"parameters_json"`
}

type buildBucketResponse struct {
	Build *Build `json:"build"`
	Kind  string `json:"kind"`
	Etag  string `json:"etag"`
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

// RequestBuild adds a request for the given build.
func (c *Client) RequestBuild(builder, master, commit, repo, author string) (*Build, error) {
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
		return nil, fmt.Errorf("Response code is %s", resp.Status)
	}
	defer util.Close(resp.Body)
	var res buildBucketResponse
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, fmt.Errorf("Failed to decode response body: %v", err)
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
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return result.Build, nil
}
