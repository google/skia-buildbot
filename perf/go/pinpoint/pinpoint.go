package pinpoint

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/perf/go/config"
	"golang.org/x/oauth2/google"
)

const (
	pinpointLegacyURL         = "https://pinpoint-dot-chromeperf.appspot.com/api/new"
	contentType               = "application/json"
	tryJobComparisonMode      = "try"
	chromeperfLegacyBisectURL = "https://chromeperf.appspot.com/pinpoint/new/bisect"
)

type TryJobCreateRequest struct {
	Name        string `json:"name"`
	BaseGitHash string `json:"base_git_hash"`
	// although "experiment" makes more sense in this context, the legacy Pinpoint API
	// explicitly defines the experiment commit as "end_git_hash" and defines
	// the experiment patch as "experiment_patch"
	EndGitHash      string `json:"end_git_hash"`
	BasePatch       string `json:"base_patch"`
	ExperimentPatch string `json:"experiment_patch"`
	Configuration   string `json:"configuration"`
	Benchmark       string `json:"benchmark"`
	Story           string `json:"story"`
	ExtraTestArgs   string `json:"extra_test_args"`
	Repository      string `json:"repository"`
	BugId           string `json:"bug_id"`
	User            string `json:"user"`
}

type BisectJobCreateRequest struct {
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
	AlertIDs            string `json:"alert_ids"`
	TestPath            string `json:"test_path"`
}

type CreatePinpointResponse struct {
	JobID  string `json:"jobId"`
	JobURL string `json:"jobUrl"`
}

type Client struct {
	httpClient         *http.Client
	createBisectCalled metrics2.Counter
	createBisectFailed metrics2.Counter
	createTryJobCalled metrics2.Counter
	createTryJobFailed metrics2.Counter
}

// New returns a new PinpointClient instance.
func New(ctx context.Context) (*Client, error) {
	tokenSource, err := google.DefaultTokenSource(ctx, auth.ScopeUserinfoEmail)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to create pinpoint client.")
	}

	client := httputils.DefaultClientConfig().WithTokenSource(tokenSource).Client()
	return &Client{
		httpClient:         client,
		createBisectCalled: metrics2.GetCounter("pinpoint_create_bisect_called"),
		createBisectFailed: metrics2.GetCounter("pinpoint_create_bisect_failed"),
		createTryJobCalled: metrics2.GetCounter("pinpoint_create_try_job_called"),
		createTryJobFailed: metrics2.GetCounter("pinpoint_create_try_job_failed"),
	}, nil
}

// CreateTryJob calls the legacy pinpoint API to create a try job.
func (pc *Client) CreateTryJob(ctx context.Context, req TryJobCreateRequest) (*CreatePinpointResponse, error) {
	pc.createTryJobCalled.Inc(1)

	requestURL, err := buildTryJobRequestURL(req)
	if err != nil {
		pc.createTryJobFailed.Inc(1)
		return nil, skerr.Wrapf(err, "Failed to generate Pinpoint request URL.")
	}
	sklog.Debugf("Preparing to call this Pinpoint service URL: %s", requestURL)

	resp, err := pc.doPostRequest(ctx, requestURL)
	if err != nil {
		pc.createTryJobFailed.Inc(1)
		return nil, err
	}
	return resp, nil
}

func buildTryJobRequestURL(req TryJobCreateRequest) (string, error) {
	if req.Benchmark == "" {
		return "", skerr.Fmt("Benchmark must be specified but is empty.")
	}
	if req.Configuration == "" {
		return "", skerr.Fmt("Configuration must be specified but is empty.")
	}

	params := url.Values{}
	// Pinpoint try jobs always use comparison mode try
	params.Set("comparison_mode", tryJobComparisonMode)
	setIfNotEmpty(params, "name", req.Name)
	setIfNotEmpty(params, "base_git_hash", req.BaseGitHash)
	setIfNotEmpty(params, "end_git_hash", req.EndGitHash)
	setIfNotEmpty(params, "base_patch", req.BasePatch)
	setIfNotEmpty(params, "experiment_patch", req.ExperimentPatch)
	setIfNotEmpty(params, "configuration", req.Configuration)
	setIfNotEmpty(params, "benchmark", req.Benchmark)
	// TODO(b/485841164): Replace with the unescaped name when it is available.
	setIfNotEmpty(params, "story", dotify(req.Story))
	setIfNotEmpty(params, "extra_test_args", req.ExtraTestArgs)
	setIfNotEmpty(params, "repository", req.Repository)
	setIfNotEmpty(params, "bug_id", req.BugId)
	setIfNotEmpty(params, "user", req.User)
	params.Set("tags", "{\"origin\":\"skia_perf\"}")

	return fmt.Sprintf("%s?%s", pinpointLegacyURL, params.Encode()), nil
}

// CreateBisect calls pinpoint API to create bisect job.
func (pc *Client) CreateBisect(ctx context.Context, req BisectJobCreateRequest) (*CreatePinpointResponse, error) {
	pc.createBisectCalled.Inc(1)

	requestURL := buildBisectJobRequestURL(req, config.Config.FetchAnomaliesFromSql)
	sklog.Debugf("Preparing to call this Pinpoint service URL: %s", requestURL)

	resp, err := pc.doPostRequest(ctx, requestURL)
	if err != nil {
		pc.createBisectFailed.Inc(1)
		return nil, err
	}
	return resp, nil
}

func buildBisectJobRequestURL(req BisectJobCreateRequest, isNewAnomaly bool) string {
	params := url.Values{}
	setIfNotEmpty(params, "comparison_mode", req.ComparisonMode)
	setIfNotEmpty(params, "start_git_hash", req.StartGitHash)
	setIfNotEmpty(params, "end_git_hash", req.EndGitHash)
	setIfNotEmpty(params, "configuration", req.Configuration)
	setIfNotEmpty(params, "benchmark", req.Benchmark)
	setIfNotEmpty(params, "story", req.Story)
	setIfNotEmpty(params, "chart", req.Chart)
	setIfNotEmpty(params, "statistic", req.Statistic)
	setIfNotEmpty(params, "comparison_magnitude", req.ComparisonMagnitude)
	setIfNotEmpty(params, "pin", req.Pin)
	setIfNotEmpty(params, "project", req.Project)
	setIfNotEmpty(params, "user", req.User)
	setIfNotEmpty(params, "bug_id", req.BugId)
	if !isNewAnomaly {
		setIfNotEmpty(params, "alert_ids", req.AlertIDs)
	}
	// Bug ID must present otherwise chromeperf returns an error.
	params.Set("bug_id", req.BugId)
	params.Set("test_path", req.TestPath)
	return fmt.Sprintf("%s?%s", chromeperfLegacyBisectURL, params.Encode())
}

func extractErrorMessage(responseBody []byte) string {
	var errorResponse struct {
		Error string `json:"error"`
	}
	err := json.Unmarshal(responseBody, &errorResponse)
	if err == nil && errorResponse.Error != "" {
		return errorResponse.Error
	}
	return string(responseBody)
}

func dotify(input string) string {
	return strings.ReplaceAll(input, "_", ".")
}

func setIfNotEmpty(params url.Values, key, value string) {
	if value != "" {
		params.Set(key, value)
	}
}

func (pc *Client) doPostRequest(ctx context.Context, requestURL string) (*CreatePinpointResponse, error) {
	httpResponse, err := httputils.PostWithContext(ctx, pc.httpClient, requestURL, contentType, nil)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to get pinpoint response.")
	}
	defer httpResponse.Body.Close()

	sklog.Debugf("Got response from Pinpoint service: %+v", *httpResponse)

	respBody, err := io.ReadAll(httpResponse.Body)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to read body from pinpoint response.")
	}
	if httpResponse.StatusCode != http.StatusOK {
		requestErrorMessage := extractErrorMessage(respBody)
		errMsg := fmt.Sprintf("Request to %s failed with status code %d and error: %s", requestURL, httpResponse.StatusCode, requestErrorMessage)
		// TODO(b/483366834): Refactor other error messages displaying to the user.
		return nil, errors.New(errMsg)
	}

	resp := CreatePinpointResponse{}
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, skerr.Wrapf(err, "Failed to parse pinpoint response body.")
	}

	return &resp, nil
}
