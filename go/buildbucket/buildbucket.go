// Package buildbucket provides tools for interacting with the buildbucket API.
package buildbucket

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"

	"go.skia.org/infra/go/jsonutils"
	"go.skia.org/infra/go/util"
)

const (
	apiUrl = "https://cr-buildbucket.appspot.com/api/buildbucket/v1"
)

var (
	DEFAULT_SCOPES = []string{
		"https://www.googleapis.com/auth/userinfo.email",
		"https://www.googleapis.com/auth/userinfo.profile",
	}
)

const (
	// Possible values for the Build.Status field.
	// See: https://github.com/luci/luci-go/blob/master/common/api/buildbucket/buildbucket/v1/buildbucket-gen.go#L156
	STATUS_COMPLETED = "COMPLETED"
	STATUS_SCHEDULED = "SCHEDULED"
	STATUS_STARTED   = "STARTED"

	// Possible values for the Build.Result field.
	// See:https://github.com/luci/luci-go/blob/master/common/api/buildbucket/buildbucket/v1/buildbucket-gen.go#L146
	RESULT_CANCELED = "CANCELED"
	RESULT_FAILURE  = "FAILURE"
	RESULT_SUCCESS  = "SUCCESS"
)

type Author struct {
	Email string `json:"email"`
}

type Change struct {
	Author     *Author `json:"author"`
	Repository string  `json:"repo_url"`
	Revision   string  `json:"revision"`
}

type Error struct {
	Message string `json:"message"`
	Reason  string `json:"reason"`
}

type Properties struct {
	AttemptStartTs   int64            `json:"attempt_start_ts"`
	Category         string           `json:"category"`
	Gerrit           string           `json:"patch_gerrit_url"`
	GerritIssue      jsonutils.Number `json:"patch_issue"`
	GerritPatchset   string           `json:"patch_ref"`
	Master           string           `json:"master"`
	PatchProject     string           `json:"patch_project"`
	PatchStorage     string           `json:"patch_storage"`
	Reason           string           `json:"reason"`
	Revision         string           `json:"revision,omitempty"`
	Rietveld         string           `json:"rietveld"`
	RietveldIssue    jsonutils.Number `json:"issue"`
	RietveldPatchset jsonutils.Number `json:"patchset"`
}

type Parameters struct {
	BuilderName string     `json:"builder_name"`
	Changes     []*Change  `json:"changes"`
	Properties  Properties `json:"properties"`
	Swarming    *swarming  `json:"swarming,omitempty"`
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
	Build *Build `json:"build"`
	Error *Error `json:"error"`
	Kind  string `json:"kind"`
	Etag  string `json:"etag"`
}

// Build is a struct containing information about a build in BuildBucket.
type Build struct {
	Bucket            string         `json:"bucket"`
	Completed         jsonutils.Time `json:"completed_ts"`
	CreatedBy         string         `json:"created_by"`
	Created           jsonutils.Time `json:"created_ts"`
	FailureReason     string         `json:"failure_reason"`
	Id                string         `json:"id"`
	Url               string         `json:"url"`
	ParametersJson    string         `json:"parameters_json"`
	Parameters        *Parameters    `json:"-"`
	Result            string         `json:"result"`
	ResultDetailsJson string         `json:"result_details_json"`
	Status            string         `json:"status"`
	StatusChanged     jsonutils.Time `json:"status_changed_ts"`
	Updated           jsonutils.Time `json:"updated_ts"`
	UtcNow            jsonutils.Time `json:"utcnow_ts"`
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
	p := Parameters{
		BuilderName: builder,
		Changes: []*Change{
			{
				Author: &Author{
					Email: author,
				},
				Repository: repo,
				Revision:   commit,
			},
		},
		Properties: Properties{
			Reason: "Triggered by SkiaScheduler",
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
	defer util.Close(resp.Body)
	if resp.StatusCode != http.StatusOK {
		b, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("Failed to schedule build (code %s); couldn't read response body: %v", resp.Status, err)
		}
		return nil, fmt.Errorf("Response code is %s. Response body:\n%s", resp.Status, string(b))
	}
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
	defer util.Close(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Request got response %s", resp.Status)
	}
	var result struct {
		Build *Build `json:"build"`
	}
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
	defer util.Close(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("Request got response %s", resp.Status)
	}
	var result struct {
		Builds     []*Build `json:"builds"`
		NextCursor string   `json:"next_cursor"`
	}
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
func (c *Client) GetTrybotsForCL(issueID, patchsetID int64, patchStorage, crUrl string) ([]*Build, error) {
	u, err := url.Parse(crUrl)
	if err != nil {
		return nil, err
	}
	host := u.Host
	q := url.Values{"tag": []string{fmt.Sprintf("buildset:patch/%s/%s/%d/%d", patchStorage, host, issueID, patchsetID)}}
	url := apiUrl + "/search?" + q.Encode()

	builds, err := c.Search(url)
	if err != nil {
		return nil, err
	}

	// Parse the parameters.
	for _, build := range builds {
		build.Parameters = &Parameters{}
		if err := json.Unmarshal([]byte(build.ParametersJson), build.Parameters); err != nil {
			return nil, fmt.Errorf("Unable to decode parameters in build: %s", err)
		}
	}

	return builds, nil
}
