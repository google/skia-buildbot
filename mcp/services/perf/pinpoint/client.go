package pinpoint

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/mcp/services/common"
	"go.skia.org/infra/pinpoint/go/backends"
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
)

// Pinpoint response format for newly triggered jobs.
type PinpointJobResponse struct {
	// A unique identifier for the Pinpoint job triggered.
	// A hash (in legacy) and UUID (in new)
	JobID string `json:"jobId"`

	// URL to the job that's triggered. For new Pinpoint jobs, the
	// route will return a 404 until it's complete.
	// Note: New Pinpoint try jobs do not support writeback to the UI
	// just yet.
	JobURL string `json:"jobUrl"`
}

type PinpointConfigurationResponse struct {
	Configurations []string `json:"configurations"`
}

// Lightweight client object.
type PinpointClient struct {
	targetNewPinpoint bool
	args              map[string]any

	Url string
}

// NewPinpointClient returns a client with the URL pointing to legacy
// or new Pinpoint depending on the arguments provided.
func NewPinpointClient(args map[string]any) *PinpointClient {
	// default to legacy
	if args == nil || args[TargetNewPinpoint] == nil {
		return &PinpointClient{
			targetNewPinpoint: false,
			Url:               LegacyPinpointUrl,
			args:              args,
		}

	}

	targetVal := args[TargetNewPinpoint].(bool)
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

// legacyRequestUrl formulates the POST request URL to /api/new
// for a Pinpoint job.
func (pc *PinpointClient) legacyRequestUrl(comparisonMode string, bisectKey string) string {
	params := url.Values{}

	sklog.Debug(pc.args)

	params.Set("comparison_mode", comparisonMode)
	params.Set("name", fmt.Sprintf("[Beta] Pinpoint Job for %s", comparisonMode))
	params.Set("tags", "{\"origin\":\"gemini\"}")

	// legacy uses a different bisect key for try and bisect.
	if pc.args[BaseGitHashFlagName] != nil {
		params.Set(bisectKey, pc.args[BaseGitHashFlagName].(string))
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
	if pc.args[IterationFlagName] != nil {
		params.Set("iterations", pc.args[IterationFlagName].(string))
	}

	url := fmt.Sprintf("%s%s?%s", pc.Url, LegacyPinpointApiNew, params.Encode())
	sklog.Debugf("Target URL for Pinpoint Job: %s", url)
	return url
}

// LegacyTryRequestUrl formulates the URL w/ comparison_mode: try
func (pc *PinpointClient) LegacyTryRequestUrl() string {
	return pc.legacyRequestUrl(PairwiseCommandName, "base_git_hash")
}

// LegacyBisectRequestUrl formulates the URL w/ comparison_mode: bisect
func (pc *PinpointClient) LegacyBisectRequestUrl() string {
	return pc.legacyRequestUrl("performance", "start_git_hash")
}

// TryJob curates the POST request to /api/new or /pinpoint/v1/schedule
// based on the arguments provided, specific to a Try Job (meaning comparison mode
// is try). Returns a PinpointResponse, containing the JobID and JobURL,
// both strings.
func (pc *PinpointClient) TryJob(ctx context.Context, c *http.Client) (*PinpointJobResponse, error) {
	if pc.targetNewPinpoint {
		// TODO(fill non legacy format)
		return nil, errors.New("tool unsupported yet for new pinpoint")
	}

	reqUrl := pc.LegacyTryRequestUrl()
	resp, err := httputils.PostWithContext(ctx, c, reqUrl, common.ContentType, nil)
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

	res := &PinpointJobResponse{}
	err = json.Unmarshal([]byte(respBody), &res)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to parse pinpoint response body.")
	}

	return res, nil
}

// setGitHashFromRevision is a helper function to determine whether
// git hash or revision is provided, for both base and experiment.
// if both are provided, the git hash is used.
// this function will modify args and return it.
func setGitHashFromRevision(ctx context.Context, args map[string]any, crrevClient *backends.CrrevClientImpl) (map[string]any, error) {
	isEmpty := func(key string) bool {
		return args[key] == nil || args[key].(string) == ""
	}
	// base case, where both are unset.
	if args == nil ||
		(isEmpty(BaseGitHashFlagName) && isEmpty(BaseRevisionFlagName)) ||
		(isEmpty(ExperimentGitHashFlagName) && isEmpty(ExperimentRevisionFlagName)) {
		return nil, errors.New("one of git hash or revision for both base and experiment is not set")
	}

	// if git hash is not set, but revision is, use crrev to figure out the hash and set it.
	if isEmpty(BaseGitHashFlagName) && !isEmpty(BaseRevisionFlagName) {
		resp, err := crrevClient.GetCommitInfo(ctx, args[BaseRevisionFlagName].(string))
		if err != nil {
			return nil, skerr.Wrapf(err, "failed to translate reivison to git hash")
		}
		args[BaseGitHashFlagName] = resp.GitHash
	}
	if isEmpty(ExperimentGitHashFlagName) && !isEmpty(ExperimentRevisionFlagName) {
		resp, err := crrevClient.GetCommitInfo(ctx, args[ExperimentRevisionFlagName].(string))
		if err != nil {
			return nil, skerr.Wrapf(err, "failed to translate reivison to git hash")
		}
		args[ExperimentGitHashFlagName] = resp.GitHash
	}
	return args, nil
}

// Bisect curates the POST request to /api/new or /pinpoint/v1/schedule
// based on the arguments provided, specific to a Bisect (meaning comparison mode
// is bisect). Returns a PinpointResponse, containing the JobID and JobURL,
// both strings.
func (pc *PinpointClient) Bisect(ctx context.Context, c *http.Client, crrevClient *backends.CrrevClientImpl) (*PinpointJobResponse, error) {
	if pc.targetNewPinpoint {
		// TODO(fill non legacy format)
		return nil, errors.New("tool unsupported yet for new pinpoint")
	}

	updatedArgs, err := setGitHashFromRevision(ctx, pc.args, crrevClient)
	if err != nil {
		return nil, err
	}
	pc.args = updatedArgs

	reqUrl := pc.LegacyBisectRequestUrl()
	resp, err := httputils.PostWithContext(ctx, c, reqUrl, common.ContentType, nil)
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

	res := &PinpointJobResponse{}
	err = json.Unmarshal([]byte(respBody), &res)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to parse pinpoint response body.")
	}

	return res, nil
}
