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
	"go.skia.org/infra/pinpoint/go/bot_configs"
	"golang.org/x/oauth2/google"
)

const (
	pinpointURL = "https://chromeperf.appspot.com/pinpoint/new/bisect"
	// use the legacy endpoint to run A/B Try jobs (pairwise jobs)
	pinpointLegacyURL    = "https://pinpoint-dot-chromeperf.appspot.com/api/new"
	contentType          = "application/json"
	tryJobComparisonMode = "try"
)

type CreateLegacyTryRequest struct {
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
	AlertIDs            string `json:"alert_ids"`
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
func (pc *Client) CreateTryJob(ctx context.Context, req CreateLegacyTryRequest) (*CreatePinpointResponse, error) {
	pc.createTryJobCalled.Inc(1)

	requestURL, err := buildTryJobRequestURL(req)
	if err != nil {
		pc.createTryJobFailed.Inc(1)
		return nil, skerr.Wrapf(err, "Failed to generate Pinpoint request URL.")
	}
	sklog.Debugf("Preparing to call this Pinpoint service URL: %s", requestURL)

	httpResponse, err := httputils.PostWithContext(ctx, pc.httpClient, requestURL, contentType, nil)
	if err != nil {
		pc.createTryJobFailed.Inc(1)
		return nil, skerr.Wrapf(err, "Failed to get pinpoint response.")
	}
	sklog.Debugf("Got response from Pinpoint service: %+v", *httpResponse)

	respBody, err := io.ReadAll(httpResponse.Body)
	if err != nil {
		pc.createTryJobFailed.Inc(1)
		return nil, skerr.Wrapf(err, "Failed to read body from pinpoint response.")
	}
	if httpResponse.StatusCode != http.StatusOK {
		errMsg := fmt.Sprintf("The try job request failed with status code %d and body: %s", httpResponse.StatusCode, string(respBody))
		err = errors.New(errMsg)
		pc.createTryJobFailed.Inc(1)
		return nil, skerr.Wrapf(err, "Pinpoint request failed.")
	}

	resp := CreatePinpointResponse{}
	err = json.Unmarshal([]byte(respBody), &resp)
	if err != nil {
		pc.createTryJobFailed.Inc(1)
		return nil, skerr.Wrapf(err, "Failed to parse pinpoint response body.")
	}

	return &resp, nil
}

func buildTryJobRequestURL(req CreateLegacyTryRequest) (string, error) {
	if req.Benchmark == "" {
		return "", skerr.Fmt("Benchmark must be specified but is empty.")
	}
	if req.Configuration == "" {
		return "", skerr.Fmt("Configuration must be specified but is empty.")
	}
	target, err := bot_configs.GetIsolateTarget(req.Configuration, req.Benchmark)
	if err != nil {
		return "", skerr.Wrapf(err, "Could not find target of bot %s and benchmark %s", req.Configuration, req.Benchmark)
	}

	params := url.Values{}
	// Pinpoint try jobs always use comparison mode try
	params.Set("comparison_mode", tryJobComparisonMode)
	if req.Name != "" {
		params.Set("name", req.Name)
	}
	if req.BaseGitHash != "" {
		params.Set("base_git_hash", req.BaseGitHash)
	}
	if req.EndGitHash != "" {
		params.Set("end_git_hash", req.EndGitHash)
	}
	if req.BasePatch != "" {
		params.Set("base_patch", req.BasePatch)
	}
	if req.ExperimentPatch != "" {
		params.Set("experiment_patch", req.ExperimentPatch)
	}
	if req.Configuration != "" {
		params.Set("configuration", req.Configuration)
	}
	if req.Benchmark != "" {
		params.Set("benchmark", req.Benchmark)
	}
	if req.Story != "" {
		params.Set("story", req.Story)
	}
	if req.ExtraTestArgs != "" {
		params.Set("extra_test_args", req.ExtraTestArgs)
	}
	if req.Repository != "" {
		params.Set("repository", req.Repository)
	}
	if req.BugId != "" {
		params.Set("bug_id", req.BugId)
	}
	if req.User != "" {
		params.Set("user", req.User)
	}
	params.Set("target", target)
	params.Set("tags", "{\"origin\":\"skia_perf\"}")

	return fmt.Sprintf("%s?%s", pinpointLegacyURL, params.Encode()), nil
}

// CreateBisect calls pinpoint API to create bisect job.
func (pc *Client) CreateBisect(ctx context.Context, createBisectRequest CreateBisectRequest) (*CreatePinpointResponse, error) {
	pc.createBisectCalled.Inc(1)

	requestURL := buildBisectRequestURL(createBisectRequest)
	sklog.Debugf("Preparing to call this Pinpoint service URL: %s", requestURL)

	httpResponse, err := httputils.PostWithContext(ctx, pc.httpClient, requestURL, contentType, nil)
	if err != nil {
		pc.createBisectFailed.Inc(1)
		return nil, skerr.Wrapf(err, "Failed to get pinpoint response.")
	}
	sklog.Debugf("Got response from Pinpoint service: %+v", *httpResponse)

	respBody, err := io.ReadAll(httpResponse.Body)
	if err != nil {
		pc.createBisectFailed.Inc(1)
		return nil, skerr.Wrapf(err, "Failed to read body from pinpoint response.")
	}
	if httpResponse.StatusCode != http.StatusOK {
		errMsg := fmt.Sprintf("The bisect request failed with status code %d and body: %s", httpResponse.StatusCode, string(respBody))
		err = errors.New(errMsg)
		pc.createBisectFailed.Inc(1)
		return nil, skerr.Wrapf(err, "Pinpoint request failed.")
	}

	resp := CreatePinpointResponse{}
	err = json.Unmarshal([]byte(respBody), &resp)
	if err != nil {
		pc.createBisectFailed.Inc(1)
		return nil, skerr.Wrapf(err, "Failed to parse pinpoint response body.")
	}

	return &resp, nil
}

func buildBisectRequestURL(createBisectRequest CreateBisectRequest) string {
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
	if createBisectRequest.AlertIDs != "" {
		params.Set("alert_ids", createBisectRequest.AlertIDs)
	}

	params.Set("tags", "{\"origin\":\"skia_perf\"}")

	// test_path is needed by the API, which is a legacy key from
	// Chromeperf. The format is
	// {master}/{bot}/{benchmark}/{chart}/{story}
	// and will cut off at the lowest available piece.
	test_path_parts := []string{"ChromiumPerf"}
	required_pieces := []string{
		createBisectRequest.Configuration,
		createBisectRequest.Benchmark,
		createBisectRequest.Chart,
		createBisectRequest.Story,
	}
	for _, val := range required_pieces {
		if val == "" {
			break
		}
		test_path_parts = append(test_path_parts, val)
	}
	params.Set("test_path", strings.Join(test_path_parts, "/"))

	return fmt.Sprintf("%s?%s", pinpointURL, params.Encode())
}
