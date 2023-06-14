package pinpoint

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"

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
	ComparisonMode      string `json:"comparison_mode"`
	StartGitHash        string `json:"start_git_hash"`
	EndGitHash          string `json:"end_git_hash"`
	Configuration       string `json:"configuration"`
	Benchmark           string `json:"benchmark"`
	Story               string `json:"story"`
	Chart               string `json:"chart"`
	Statistic           string `json:"statistic"`
	ComparisonMagnitude string `json:"comparison_magnitude"`
	Pin                 string `json:"pin"`
	Project             string `json:"project"`
	BugId               string `json:"bug_id"`
	User                string `json:"user"`
}

type CreateBisectResponse struct {
	JobID  string `json:"jobId"`
	JobURL string `json:"jobUrl"`
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

	requestURL := buildPinpointRequestURL(createBisectRequest)
	sklog.Debugf("Preparing to call this Pinpoint service URL: %s", requestURL)

	httpResponse, err := httputils.PostWithContext(ctx, pc.httpClient, requestURL, contentType, nil)
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
	if httpResponse.StatusCode != http.StatusOK {
		errMsg := fmt.Sprintf("The request failed with status code %d and body: %s", httpResponse.StatusCode, string(respBody))
		err = errors.New(errMsg)
		return nil, skerr.Wrapf(err, "Pinpoint request failed.")
	}

	resp := CreateBisectResponse{}
	err = json.Unmarshal([]byte(respBody), &resp)
	if err != nil {
		pc.createBisectFailed.Inc(1)
		return nil, skerr.Wrapf(err, "Failed to parse pinpoint response body.")
	}

	return &resp, nil
}

func buildPinpointRequestURL(createBisectRequest CreateBisectRequest) string {
	params := url.Values{}
	if createBisectRequest.ComparisonMode != "" {
		params.Set("comparison_mode", createBisectRequest.ComparisonMode)
	}
	if createBisectRequest.StartGitHash != "" {
		params.Set("start_git_hash", createBisectRequest.StartGitHash)
	}
	if createBisectRequest.EndGitHash != "" {
		params.Set("end_git_hash", createBisectRequest.EndGitHash)
	}
	if createBisectRequest.Configuration != "" {
		params.Set("configuration", createBisectRequest.Configuration)
	}
	if createBisectRequest.Benchmark != "" {
		params.Set("benchmark", createBisectRequest.Benchmark)
	}
	if createBisectRequest.Story != "" {
		params.Set("story", createBisectRequest.Story)
	}
	if createBisectRequest.Chart != "" {
		params.Set("chart", createBisectRequest.Chart)
	}
	if createBisectRequest.Statistic != "" {
		params.Set("statistic", createBisectRequest.Statistic)
	}
	if createBisectRequest.ComparisonMagnitude != "" {
		params.Set("comparison_magnitude", createBisectRequest.ComparisonMagnitude)
	}
	if createBisectRequest.Pin != "" {
		params.Set("pin", createBisectRequest.Pin)
	}
	if createBisectRequest.Project != "" {
		params.Set("project", createBisectRequest.Project)
	}
	if createBisectRequest.BugId != "" {
		params.Set("bug_id", createBisectRequest.BugId)
	}
	if createBisectRequest.User != "" {
		params.Set("user", createBisectRequest.User)
	}

	params.Set("tags", "{\"origin\":\"skia_perf\"}")

	return fmt.Sprintf("%s?%s", pinpointURL, params.Encode())
}
