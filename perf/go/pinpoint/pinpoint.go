package pinpoint

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"strings"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"golang.org/x/oauth2/google"
)

const (
	pinpointURL = "https://pinpoint-dot-chromeperf.appspot.com/api/new"
	contentType = "application/json"
)

type CreateBisectRequest struct {
	ComparisonMode      string   `json:"comparison_mode"`
	Target              string   `json:"target"`
	StartGitHash        string   `json:"start_git_hash"`
	EndGitHash          string   `json:"end_git_hash"`
	Configuration       string   `json:"configuration"`
	Benchmark           string   `json:"benchmark"`
	Story               string   `json:"story"`
	StoryTags           []string `json:"story_tags"`
	Chart               string   `json:"chart"`
	Statistic           string   `json:"statistic"`
	ComparisonMagnitude string   `json:"comparison_magnitude"`
	Pin                 string   `json:"pin"`
	Project             string   `json:"project"`
	BugId               string   `json:"bug_id"`
	BatchId             string   `json:"batch_id"`
	User                string   `json:"user"`
}

type CreateBisectResponse struct {
	JobID  string `json:"job_id"`
	JobURL string `json:"job_url"`
}

type Client struct {
	httpClient         *http.Client
	createBisectCalled metrics2.Counter
	createBisectFailed metrics2.Counter
}

// New returns a new PinpointClient instance.
func New(ctx context.Context) (*Client, error) {
	tokenSource, err := google.DefaultTokenSource(ctx, auth.ScopeAllCloudAPIs)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to create pinpoint client.")
	}

	client := httputils.DefaultClientConfig().WithTokenSource(tokenSource).Client()
	return &Client{
		httpClient:         client,
		createBisectCalled: metrics2.GetCounter("pinpoint_create_bisect_called"),
		createBisectFailed: metrics2.GetCounter("pinpoint_create_bisect_failed"),
	}, nil
}

// CreateBisect calls pinpoint API to create bisect job.
func (pc *Client) CreateBisect(ctx context.Context, createBisectRequest CreateBisectRequest) (*CreateBisectResponse, error) {
	pc.createBisectCalled.Inc(1)

	requestBodyJSONStr, err := json.Marshal(createBisectRequest)
	if err != nil {
		pc.createBisectFailed.Inc(1)
		return nil, skerr.Wrapf(err, "Failed to create pinpoint request.")
	}

	httpResponse, err := httputils.PostWithContext(ctx, pc.httpClient, pinpointURL, contentType, strings.NewReader(string(requestBodyJSONStr)))
	if err != nil {
		pc.createBisectFailed.Inc(1)
		return nil, skerr.Wrapf(err, "Failed to get pinpoint response.")
	}
	sklog.Debugf("Got response from Pinpoint service: %+v", *httpResponse)

	respBody, err := ioutil.ReadAll(httpResponse.Body)
	if err != nil {
		pc.createBisectFailed.Inc(1)
		return nil, skerr.Wrapf(err, "Failed to read body from pinpoint response.")
	}

	resp := CreateBisectResponse{}
	err = json.Unmarshal([]byte(respBody), &resp)
	if err != nil {
		pc.createBisectFailed.Inc(1)
		return nil, skerr.Wrapf(err, "Failed to parse pinpoint response body.")
	}

	return &resp, nil
}
