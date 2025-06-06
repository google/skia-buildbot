package pinpoint

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"golang.org/x/oauth2/google"
)

const (
	// Refered to as comparison_mode for Pinpoint, one of bisect or try.
	// bisect refers to culprit detection, or in other words, trying to
	// find the culprit change that's caused a regression in a given range
	// of commits.
	BisectCommandName = "bisect"

	// Try refers to the performance comparison of two static changes.
	PairwiseCommandName = "try"

	// Legacy Pinpoint, or Chromeperf Pinpoint instance.
	LegacyPinpointUrl = "https://pinpoint-dot-chromeperf.appspot.com"

	// Endpoint for new bisect/try jobs.
	LegacyPinpointApiNew = "/api/new"

	// New Pinpoint. The service handler is implemented in FE, so we
	// target public perf instance to ensure that Chrome requests are
	// handled within Chrome instances.
	PinpointUrl = "https://perf.luci.app"

	// Endpoint for scheduling Pinpoint workflows through Temporal.
	PinpointV1Schedule = "/pinpoint/v1/schedule"

	// Content type header application/json.
	contentType = "application/json"
)

// Pinpoint response format for newly triggered jobs.
type PinpointResponse struct {
	// A unique identifier for the Pinpoint job triggered.
	// A hash (in legacy) and UUID (in new)
	JobID string `json:"jobId"`

	// URL to the job that's triggered. For new Pinpoint jobs, the
	// route will return a 404 until it's complete.
	// Note: New Pinpoint try jobs do not support writeback to the UI
	// just yet.
	JobURL string `json:"jobUrl"`
}

// Lightweight client object.
type PinpointClient struct {
	targetNewPinpoint bool
	args              map[string]any

	Url string
}

// defaultHttpClient returns a HTTP client handler configured w/ default
// https://www.googleapis.com/auth/userinfo.email scope.
func defaultHttpClient(ctx context.Context) (*http.Client, error) {
	tokenSource, err := google.DefaultTokenSource(ctx, auth.ScopeUserinfoEmail)
	if err != nil {
		return nil, skerr.Wrapf(err, "failed to create pinpoint client.")
	}

	return httputils.DefaultClientConfig().WithTokenSource(tokenSource).Client(), nil
}

// NewPinpointClient returns a client with the URL pointing to legacy
// or new Pinpoint depending on the arguments provided.
func NewPinpointClient(args map[string]any) *PinpointClient {
	targetNewPinpoint := args[TargetNewPinpoint]
	// default to legacy
	if targetNewPinpoint == nil {
		return &PinpointClient{
			targetNewPinpoint: false,
			Url:               LegacyPinpointUrl,
			args:              args,
		}

	}

	targetVal := targetNewPinpoint.(bool)
	url := LegacyPinpointUrl
	if targetVal {
		url = PinpointUrl
	}
	return &PinpointClient{
		targetNewPinpoint: targetVal,
		Url:               url,
		args:              args,
	}
}

// LegacyTryRequestUrl formulates the POST request URL to /api/new
// for a Pinpoint job.
func (pc *PinpointClient) LegacyRequestUrl() string {
	params := url.Values{}

	sklog.Debug(pc.args)

	params.Set("comparison_mode", PairwiseCommandName)
	params.Set("name", "[Test] Auto Triggered Try Job")
	params.Set("tags", "{\"origin\":\"gemini\"}")

	if pc.args[BaseGitHashFlagName] != nil {
		params.Set("base_git_hash", pc.args[BaseGitHashFlagName].(string))
	}
	if pc.args[ExperimentGitHashFlagName] != nil {
		params.Set("end_git_hash", pc.args[ExperimentGitHashFlagName].(string))
	}
	if pc.args[BotConfigurationFlagName] != nil {
		params.Set("configuration", pc.args[BotConfigurationFlagName].(string))
	}
	if pc.args[BenchmarkFlagName] != nil {
		params.Set("benchmark", pc.args[BenchmarkFlagName].(string))
	}
	if pc.args[StoryFlagName] != nil {
		params.Set("story", pc.args[StoryFlagName].(string))
	}

	url := fmt.Sprintf("%s%s?%s", pc.Url, LegacyPinpointApiNew, params.Encode())
	sklog.Debugf("Target URL for Pinpoint Try Job: %s", url)
	return url
}

// TryJob curates the POST request to /api/new or /pinpoint/v1/schedule
// based on the arguments provided and sends the request.
// Returns a PinpointResponse, containing the JobiD and the JobURL.
func (pc *PinpointClient) TryJob(ctx context.Context, c *http.Client) (*PinpointResponse, error) {
	if pc.targetNewPinpoint {
		// TODO(fill non legacy format)
		return nil, nil
	}

	reqUrl := pc.LegacyRequestUrl()
	resp, err := httputils.PostWithContext(ctx, c, reqUrl, contentType, nil)
	if err != nil {
		return nil, skerr.Wrapf(err, "failed to execute Pinpoint call")
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed with request %d", resp.StatusCode)
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, skerr.Wrapf(err, "failed to read body of response")
	}

	res := &PinpointResponse{}
	err = json.Unmarshal([]byte(respBody), &res)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to parse pinpoint response body.")
	}

	return res, nil
}
