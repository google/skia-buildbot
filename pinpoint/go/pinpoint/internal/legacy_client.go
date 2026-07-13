package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"

	"golang.org/x/oauth2/google"
	"google.golang.org/protobuf/types/known/timestamppb"

	"go.skia.org/infra/go/auth"
	pb "go.skia.org/infra/pinpoint/proto/v1"
)

const (
	pinpointLegacyBaseURL         = "https://pinpoint-dot-chromeperf.appspot.com"
	pinpointLegacyNewJobURL       = pinpointLegacyBaseURL + "/api/new"
	pinpointLegacyJobsURL         = pinpointLegacyBaseURL + "/api/jobs"
	pinpointLegacyJobStateURL     = pinpointLegacyBaseURL + "/api/job"
	pinpointLegacyConfigURL       = pinpointLegacyBaseURL + "/api/config"
	pinpointLegacyBuildsURL       = pinpointLegacyBaseURL + "/api/builds"
	pinpointLegacyCommitURL       = pinpointLegacyBaseURL + "/api/commit"
	pinpointLegacyCancelJobURL    = pinpointLegacyBaseURL + "/api/job/cancel"
	DefaultCancellationReason     = "No reason provided"
	contentType                   = "application/json"
	tryJobComparisonMode          = "try"
	chromeperfLegacyBaseURL       = "https://chromeperf.appspot.com"
	chromeperfLegacyBisectURL     = chromeperfLegacyBaseURL + "/pinpoint/new/bisect"
	chromeperfLegacyTestSuitesURL = chromeperfLegacyBaseURL + "/api/test_suites"
	chromeperfLegacyDescribeURL   = chromeperfLegacyBaseURL + "/api/describe"
	legacyCreatedTimeLayout       = "2006-01-02T15:04:05.999999" // Layout used to parse legacy Pinpoint job creation time.
)

// Some benchmarks are not returned by the legacy Pinpoint because they are hardcoded in the Web UI.
// Hardcode these benchmarks here for now until we switch to the new backend.
var missingBenchmarks = []string{
	"blink-ai.crossbench",
	"devtools_frontend.crossbench",
	"gma.embedder.crossbench",
	"jetstream-main.crossbench",
	"jetstream2.0.crossbench",
	"jetstream2.1.crossbench",
	"jetstream2.2.crossbench",
	"jetstream3.0.crossbench",
	"jetstream3.crossbench",
	"loadline2_tablet.crossbench",
	"motionmark1.0.crossbench",
	"motionmark1.1.crossbench",
	"motionmark1.2.crossbench",
	"motionmark1.3.1.crossbench",
	"speedometer-main.crossbench",
	"webai.crossbench",
}

type LegacyClient struct {
	httpClient                 *http.Client
	createBisectCalled         metrics2.Counter
	createBisectFailed         metrics2.Counter
	createTryJobCalled         metrics2.Counter
	createTryJobFailed         metrics2.Counter
	fetchJobStateCalled        metrics2.Counter
	fetchJobStateFailed        metrics2.Counter
	queryJobListCalled         metrics2.Counter
	queryJobListFailed         metrics2.Counter
	createPinpointTryJobCalled metrics2.Counter
	createPinpointTryJobFailed metrics2.Counter
	listBotsCalled             metrics2.Counter
	listBotsFailed             metrics2.Counter
	listBenchmarksCalled       metrics2.Counter
	listBenchmarksFailed       metrics2.Counter
	getBenchmarkCalled         metrics2.Counter
	getBenchmarkFailed         metrics2.Counter
	listRecentBuildsCalled     metrics2.Counter
	listRecentBuildsFailed     metrics2.Counter
	cancelJobCalled            metrics2.Counter
	cancelJobFailed            metrics2.Counter
}

// New returns a new LegacyClient instance.
func NewLegacyClient(ctx context.Context) (*LegacyClient, error) {
	tokenSource, err := google.DefaultTokenSource(ctx, auth.ScopeUserinfoEmail)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to create pinpoint client.")
	}

	client := httputils.DefaultClientConfig().WithTokenSource(tokenSource).Client()
	return &LegacyClient{
		httpClient:         client,
		createBisectCalled: metrics2.GetCounter("pinpoint_create_bisect_called"),
		createBisectFailed: metrics2.GetCounter("pinpoint_create_bisect_failed"),
		// Two metrics below refence to calling Pinpoint via Chromeperf.
		createTryJobCalled: metrics2.GetCounter("pinpoint_create_try_job_called"),
		createTryJobFailed: metrics2.GetCounter("pinpoint_create_try_job_failed"),

		// Metrics for calling the legacy Pinpoint API from the gateway.
		fetchJobStateCalled:        metrics2.GetCounter("pinpoint_fetch_job_state_called"),
		fetchJobStateFailed:        metrics2.GetCounter("pinpoint_fetch_job_state_failed"),
		queryJobListCalled:         metrics2.GetCounter("pinpoint_query_job_list_called"),
		queryJobListFailed:         metrics2.GetCounter("pinpoint_query_job_list_failed"),
		createPinpointTryJobCalled: metrics2.GetCounter("pinpoint_create_pinpoint_try_job_called"),
		createPinpointTryJobFailed: metrics2.GetCounter("pinpoint_create_pinpoint_try_job_failed"),
		listBotsCalled:             metrics2.GetCounter("pinpoint_list_bots_called"),
		listBotsFailed:             metrics2.GetCounter("pinpoint_list_bots_failed"),
		listBenchmarksCalled:       metrics2.GetCounter("pinpoint_list_benchmarks_called"),
		listBenchmarksFailed:       metrics2.GetCounter("pinpoint_list_benchmarks_failed"),
		getBenchmarkCalled:         metrics2.GetCounter("pinpoint_get_benchmark_called"),
		getBenchmarkFailed:         metrics2.GetCounter("pinpoint_get_benchmark_failed"),
		listRecentBuildsCalled:     metrics2.GetCounter("pinpoint_list_recent_builds_called"),
		listRecentBuildsFailed:     metrics2.GetCounter("pinpoint_list_recent_builds_failed"),
		cancelJobCalled:            metrics2.GetCounter("pinpoint_cancel_job_called"),
		cancelJobFailed:            metrics2.GetCounter("pinpoint_cancel_job_failed"),
	}, nil
}

// CreateTryJob calls the Chromeperf API to create a try job.
func (pc *LegacyClient) CreateTryJob(
	ctx context.Context,
	req *TryJobCreateRequest,
) (resp *CreatePinpointResponse, err error) {
	pc.createTryJobCalled.Inc(1)
	defer func() { trackError(pc.createTryJobFailed, err) }()

	requestURL, err := buildTryJobRequestURL(req)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to generate Pinpoint request URL.")
	}

	httpResp, err := pc.doPostRequest(ctx, requestURL)
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	body, err := pc.readResponseBody(httpResp)
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	if err = json.Unmarshal(body, &resp); err != nil {
		return nil, skerr.Wrapf(err, "Failed to parse pinpoint response body.")
	}
	return resp, nil
}

// CreateBisect calls Chromeperf API to create bisect job.
func (pc *LegacyClient) CreateBisect(
	ctx context.Context,
	req *BisectJobCreateRequest,
	isNewAnomaly bool,
) (resp *CreatePinpointResponse, err error) {
	pc.createBisectCalled.Inc(1)
	defer func() { trackError(pc.createBisectFailed, err) }()

	requestURL := buildBisectJobRequestURL(req, isNewAnomaly)
	httpResp, err := pc.doPostRequest(ctx, requestURL)
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	body, err := pc.readResponseBody(httpResp)
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	if err = json.Unmarshal(body, &resp); err != nil {
		return nil, skerr.Wrapf(err, "Failed to parse pinpoint response body.")
	}
	return resp, nil
}

// FetchJobState queries the legacy pinpoint API to retrieve job details.
func (pc *LegacyClient) FetchJobState(
	ctx context.Context,
	req FetchJobStateRequest,
) (resp *FetchJobStateResponse, err error) {
	pc.fetchJobStateCalled.Inc(1)
	defer func() { trackError(pc.fetchJobStateFailed, err) }()

	requestURL := fmt.Sprintf(
		"%s/%s?o=STATE",
		pinpointLegacyJobStateURL,
		url.PathEscape(req.JobID),
	)
	httpResp, err := pc.doGetRequest(ctx, requestURL)
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	body, err := pc.readResponseBody(httpResp)
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	if err = json.Unmarshal(body, &resp); err != nil {
		return nil, skerr.Wrapf(err, "Failed to parse pinpoint response body.")
	}
	return resp, skerr.Wrap(err)
}

// QueryJobList queries the legacy Pinpoint API to retrieve jobs matching
// filters and pagination.
func (pc *LegacyClient) QueryJobList(
	ctx context.Context,
	req *pb.QueryJobListRequest,
) (resp *pb.QueryJobListResponse, err error) {
	pc.queryJobListCalled.Inc(1)
	defer func() { trackError(pc.queryJobListFailed, err) }()

	requestURL := buildQueryJobListRequestURL(req)

	httpResp, err := pc.doGetRequest(ctx, requestURL)
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	body, err := pc.readResponseBody(httpResp)
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	legacyResp := &LegacyQueryJobListResponse{}
	if err = json.Unmarshal(body, legacyResp); err != nil {
		return nil, skerr.Wrapf(err, "Failed to parse pinpoint response body.")
	}

	return parseQueryJobListResponse(legacyResp), nil
}

// extractField returns the field from the top-level struct if populated,
// otherwise falls back to the arguments map.
func extractField(job *LegacyJobSummary, fieldName string) string {
	fieldName = strings.ToLower(strings.TrimSpace(fieldName))
	switch fieldName {
	case "job_id":
		if job.JobID != "" {
			return job.JobID
		}
	case "name":
		if job.Name != "" {
			return job.Name
		}
	case "benchmark":
		if job.Benchmark != "" {
			return job.Benchmark
		}
	case "configuration":
		if job.Configuration != "" {
			return job.Configuration
		}
	case "story":
		if job.Story != "" {
			return job.Story
		}
	case "user":
		if job.User != "" {
			return job.User
		}
	case "comparison_mode":
		if job.ComparisonMode != "" {
			return job.ComparisonMode
		}
	case "bug_id":
		if job.BugID != nil {
			return strconv.FormatInt(*job.BugID, 10)
		}
	}

	if job.Arguments != nil {
		if val, ok := job.Arguments[fieldName]; ok && val != "" {
			return val
		}
	}
	return ""
}

func trackError(counter metrics2.Counter, err error) {
	if err != nil {
		counter.Inc(1)
	}
}

func parseJobStatus(status string) pb.JobStatus {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "queued":
		return pb.JobStatus_JOB_STATUS_QUEUED
	case "running":
		return pb.JobStatus_JOB_STATUS_RUNNING
	case "completed":
		return pb.JobStatus_JOB_STATUS_COMPLETED
	case "failed":
		return pb.JobStatus_JOB_STATUS_FAILED
	case "cancelled":
		return pb.JobStatus_JOB_STATUS_CANCELLED
	default:
		return pb.JobStatus_JOB_STATUS_UNSPECIFIED
	}
}

func parseJobType(comparisonMode string) pb.JobType {
	switch strings.ToLower(strings.TrimSpace(comparisonMode)) {
	case "try":
		return pb.JobType_JOB_TYPE_TRY
	case "performance", "functional":
		return pb.JobType_JOB_TYPE_BISECT
	default:
		return pb.JobType_JOB_TYPE_UNSPECIFIED
	}
}

func parseQueryJobListResponse(legacyResp *LegacyQueryJobListResponse) *pb.QueryJobListResponse {
	resp := &pb.QueryJobListResponse{
		Jobs: make([]*pb.JobSummary, len(legacyResp.Jobs)),
		Pagination: &pb.Pagination{
			PrevCursor: legacyResp.PrevCursor,
			NextCursor: legacyResp.NextCursor,
			HasPrev:    &legacyResp.Prev,
			HasNext:    &legacyResp.Next,
		},
	}

	for i := range legacyResp.Jobs {
		legacyJob := &legacyResp.Jobs[i]
		var createdTime *timestamppb.Timestamp
		if legacyJob.Created != "" {
			t, err := time.Parse(legacyCreatedTimeLayout, legacyJob.Created)
			if err == nil {
				createdTime = timestamppb.New(t)
			}
		}

		jobStatus := parseJobStatus(legacyJob.Status)
		jobType := parseJobType(extractField(legacyJob, "comparison_mode"))

		var bugId *int64
		if bugIdStr := extractField(legacyJob, "bug_id"); bugIdStr != "" {
			if bid, err := strconv.ParseInt(bugIdStr, 10, 64); err == nil {
				bugId = &bid
			}
		}

		resp.Jobs[i] = &pb.JobSummary{
			JobId:         extractField(legacyJob, "job_id"),
			Name:          extractField(legacyJob, "name"),
			Benchmark:     extractField(legacyJob, "benchmark"),
			Configuration: extractField(legacyJob, "configuration"),
			Story:         extractField(legacyJob, "story"),
			JobType:       jobType,
			User:          extractField(legacyJob, "user"),
			Created:       createdTime,
			JobStatus:     jobStatus,
			BugId:         bugId,
		}
	}

	return resp
}

func buildQueryJobListParams(req *pb.QueryJobListRequest) url.Values {
	params := url.Values{}
	if req.User != "" {
		params.Add("filter", "user="+req.User)
	}
	if req.Configuration != "" {
		params.Add("filter", "configuration="+req.Configuration)
	}
	if req.JobType != pb.JobType_JOB_TYPE_UNSPECIFIED {
		switch req.JobType {
		case pb.JobType_JOB_TYPE_TRY:
			params.Add("filter", "comparison_mode=try")
		case pb.JobType_JOB_TYPE_BISECT:
			params.Add("filter", "comparison_mode=performance")
		}
	}
	if req.Pagination != nil {
		if req.Pagination.PrevCursor != "" {
			params.Set("prev_cursor", req.Pagination.PrevCursor)
		}
		if req.Pagination.NextCursor != "" {
			params.Set("next_cursor", req.Pagination.NextCursor)
		}
	}
	return params
}

func buildQueryJobListRequestURL(req *pb.QueryJobListRequest) string {
	params := buildQueryJobListParams(req)
	requestURL := pinpointLegacyJobsURL
	if len(params) > 0 {
		requestURL = fmt.Sprintf("%s?%s", requestURL, params.Encode())
	}
	return requestURL
}

func buildTryJobRequestURL(req *TryJobCreateRequest) (string, error) {
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
	setIfNotEmpty(params, "story", req.Story)
	setIfNotEmpty(params, "extra_test_args", req.ExtraTestArgs)
	setIfNotEmpty(params, "repository", req.Repository)
	setIfNotEmpty(params, "project", req.Project)
	setIfNotEmpty(params, "bug_id", req.BugId)
	setIfNotEmpty(params, "user", req.User)
	params.Set("tags", "{\"origin\":\"skia_perf\"}")

	return fmt.Sprintf("%s?%s", pinpointLegacyNewJobURL, params.Encode()), nil
}

func buildBisectJobRequestURL(req *BisectJobCreateRequest, isNewAnomaly bool) string {
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
	setIfNotEmpty(params, "extra_test_args", req.ExtraTestArgs)
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

func setIfNotEmpty(params url.Values, key, value string) {
	if value != "" {
		params.Set(key, value)
	}
}

func (pc *LegacyClient) doPostRequest(
	ctx context.Context,
	requestURL string,
) (*http.Response, error) {
	sklog.Debugf("Preparing to send a Pinpoint POST request to: %s", requestURL)
	resp, err := httputils.PostWithContext(ctx, pc.httpClient, requestURL, contentType, nil)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to get pinpoint response.")
	}
	sklog.Debugf("Got response from Pinpoint service: %+v", resp)
	return resp, nil
}

func (pc *LegacyClient) doGetRequest(
	ctx context.Context,
	requestURL string,
) (*http.Response, error) {
	sklog.Debugf("Preparing to send a Pinpoint GET request to: %s", requestURL)
	resp, err := httputils.GetWithContext(ctx, pc.httpClient, requestURL)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to get pinpoint response.")
	}
	sklog.Debugf("Got response from Pinpoint service: %+v", resp)
	return resp, nil
}

func (pc *LegacyClient) readResponseBody(
	resp *http.Response,
) (body []byte, err error) {
	defer resp.Body.Close()
	if body, err = io.ReadAll(resp.Body); err != nil {
		return nil, skerr.Wrapf(err, "Failed to read body from pinpoint response.")
	}
	if resp.StatusCode != http.StatusOK {
		requestErrorMessage := extractErrorMessage(body)

		// A response must contain a request with a URL. Condition is just to make
		// sure we never panic here.
		url := "Unknown URL"
		if resp.Request != nil && resp.Request.URL != nil {
			url = resp.Request.URL.String()
		}
		sklog.Errorf(
			"Request to %s failed with status code %d and error: %s",
			url,
			resp.StatusCode,
			requestErrorMessage,
		)
		return nil, skerr.Fmt("[Error %d] %s", resp.StatusCode, requestErrorMessage)
	}

	return body, nil
}

// CreatePinpointTryJob calls the legacy pinpoint API to create a try job.
func (pc *LegacyClient) CreatePinpointTryJob(
	ctx context.Context,
	req *pb.CreateTryJobRequest,
) (resp *pb.CreateJobResponse, err error) {
	pc.createPinpointTryJobCalled.Inc(1)
	defer func() { trackError(pc.createPinpointTryJobFailed, err) }()

	requestURL, err := buildCreateTryJobRequestURL(req)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to generate Pinpoint request URL.")
	}

	httpResp, err := pc.doPostRequest(ctx, requestURL)
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	body, err := pc.readResponseBody(httpResp)
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	var legacyResp CreatePinpointResponse
	if err = json.Unmarshal(body, &legacyResp); err != nil {
		return nil, skerr.Wrapf(err, "Failed to parse pinpoint response body.")
	}
	return &pb.CreateJobResponse{
		JobId: legacyResp.JobID,
	}, nil
}

func buildCreateTryJobRequestURL(req *pb.CreateTryJobRequest) (string, error) {
	if req.Benchmark == "" {
		return "", skerr.Fmt("Benchmark must be specified but is empty.")
	}
	if req.Configuration == "" {
		return "", skerr.Fmt("Configuration must be specified but is empty.")
	}
	if req.AttemptCount <= 0 {
		return "", skerr.Fmt("Attempt count should be greater than zero.")
	}
	if req.Base == nil {
		return "", skerr.Fmt("Base variant configuration is required.")
	}
	if req.Experiment == nil {
		return "", skerr.Fmt("Experiment variant configuration is required.")
	}
	if req.User == "" {
		return "", skerr.Fmt("User email must be specified but is empty.")
	}

	params := url.Values{}
	params.Set("comparison_mode", tryJobComparisonMode)
	params.Set("benchmark", req.Benchmark)
	params.Set("configuration", req.Configuration)
	// The legacy Pinpoint API requires Story or Story Tags to be set even if they are empty.
	params.Set("story", req.Story)
	params.Set("story_tags", req.StoryTags)
	params.Set("initial_attempt_count", strconv.Itoa(int(req.AttemptCount)))

	if req.BugId != nil {
		if *req.BugId <= 0 {
			return "", skerr.Fmt("Bug ID should be greater than zero.")
		}
		params.Set("bug_id", strconv.FormatInt(*req.BugId, 10))
	}

	params.Set("base_git_hash", req.Base.Commit)
	params.Set("end_git_hash", req.Experiment.Commit)

	setIfNotEmpty(params, "base_patch", req.Base.Patch)
	setIfNotEmpty(params, "experiment_patch", req.Experiment.Patch)

	if baseExtra := getExtraArgsString(req.Base.ExtraArgs, req.Benchmark); baseExtra != "" {
		params.Set("base_extra_args", baseExtra)
	}
	if expExtra := getExtraArgsString(req.Experiment.ExtraArgs, req.Benchmark); expExtra != "" {
		params.Set("experiment_extra_args", expExtra)
	}

	params.Set("user", req.User)

	if req.JobName != "" {
		params.Set("name", req.JobName)
	} else {
		params.Set("name", fmt.Sprintf("Try job on %s/%s", req.Configuration, req.Benchmark))
	}

	params.Set("tags", "{\"origin\":\"New Pinpoint\"}")

	return fmt.Sprintf("%s?%s", pinpointLegacyNewJobURL, params.Encode()), nil
}

func getExtraArgsString(extraArgs *pb.ExtraArgs, benchmark string) string {
	if extraArgs == nil {
		return ""
	}

	extraBrowserArgs := []string{extraArgs.ExtraBrowserArgs}
	if extraArgs.JsFlags != "" {
		extraBrowserArgs = append(
			extraBrowserArgs,
			fmt.Sprintf("--js-flags=%s", extraArgs.JsFlags),
		)
	}
	if extraArgs.EnableFeatures != "" {
		extraBrowserArgs = append(
			extraBrowserArgs,
			fmt.Sprintf("--enable-features=%s", extraArgs.EnableFeatures),
		)
	}
	if extraArgs.DisableFeatures != "" {
		extraBrowserArgs = append(
			extraBrowserArgs,
			fmt.Sprintf("--disable-features=%s", extraArgs.DisableFeatures),
		)
	}

	return strings.TrimSpace(fmt.Sprintf(
		"%s %s",
		extraArgs.BenchmarkRunnerArgs,
		combineExtraBrowserArgs(extraBrowserArgs, benchmark),
	))
}

func combineExtraBrowserArgs(extraBrowserArgs []string, benchmark string) string {
	args := strings.TrimSpace(strings.Join(extraBrowserArgs, " "))
	if !strings.HasSuffix(benchmark, ".crossbench") && args != "" {
		return fmt.Sprintf("--extra-browser-args=%q", args)
	}
	return args
}

// ListBotConfigurations queries the legacy Pinpoint API to retrieve available bots.
func (pc *LegacyClient) ListBotConfigurations(ctx context.Context) (resp []string, err error) {
	pc.listBotsCalled.Inc(1)
	defer func() { trackError(pc.listBotsFailed, err) }()

	httpResp, err := pc.doPostRequest(ctx, pinpointLegacyConfigURL)
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	body, err := pc.readResponseBody(httpResp)
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	var legacyResp BotConfigurationsResponse
	if err = json.Unmarshal(body, &legacyResp); err != nil {
		return nil, skerr.Wrapf(err, "Failed to parse pinpoint response body.")
	}
	return legacyResp.Configurations, nil
}

// ListBenchmarks queries the legacy Pinpoint/Chromeperf API to retrieve available benchmarks.
func (pc *LegacyClient) ListBenchmarks(ctx context.Context) (resp []string, err error) {
	pc.listBenchmarksCalled.Inc(1)
	defer func() { trackError(pc.listBenchmarksFailed, err) }()

	httpResp, err := pc.doPostRequest(ctx, chromeperfLegacyTestSuitesURL)
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	body, err := pc.readResponseBody(httpResp)
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	var benchmarks []string
	if err = json.Unmarshal(body, &benchmarks); err != nil {
		return nil, skerr.Wrapf(err, "Failed to parse benchmarks response body.")
	}

	benchmarks = append(benchmarks, missingBenchmarks...)
	slices.Sort(benchmarks)
	return slices.Compact(benchmarks), nil
}

type legacyDescribeResponse struct {
	CaseTags map[string][]string `json:"caseTags"`
	Cases    []string            `json:"cases"`
}

// GetBenchmarkInfo queries the legacy Chromeperf API to retrieve available stories and story tags.
func (pc *LegacyClient) GetBenchmarkInfo(
	ctx context.Context,
	benchmark string,
) (info *BenchmarkInfo, err error) {
	pc.getBenchmarkCalled.Inc(1)
	defer func() { trackError(pc.getBenchmarkFailed, err) }()

	params := url.Values{}
	params.Set("test_suite", benchmark)
	params.Set("master", "ChromiumPerf")
	requestURL := fmt.Sprintf("%s?%s", chromeperfLegacyDescribeURL, params.Encode())

	httpResp, err := pc.doPostRequest(ctx, requestURL)
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	body, err := pc.readResponseBody(httpResp)
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	var describeResp legacyDescribeResponse
	if err = json.Unmarshal(body, &describeResp); err != nil {
		return nil, skerr.Wrapf(err, "Failed to parse describe response body.")
	}

	storyTags := slices.Collect(maps.Keys(describeResp.CaseTags))
	slices.Sort(storyTags)
	slices.Sort(describeResp.Cases)

	return &BenchmarkInfo{
		Benchmark: benchmark,
		Stories:   describeResp.Cases,
		StoryTags: storyTags,
	}, nil
}

// ListRecentBuilds queries the legacy Pinpoint API to retrieve recent builds.
func (pc *LegacyClient) ListRecentBuilds(
	ctx context.Context,
	configuration string,
) (resp []*pb.BuildInfo, err error) {
	pc.listRecentBuildsCalled.Inc(1)
	defer func() { trackError(pc.listRecentBuildsFailed, err) }()

	requestURL := fmt.Sprintf("%s/%s", pinpointLegacyBuildsURL, url.PathEscape(configuration))
	httpResp, err := pc.doGetRequest(ctx, requestURL)
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	body, err := pc.readResponseBody(httpResp)
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	var legacyResp BuildsResponse
	if err = json.Unmarshal(body, &legacyResp); err != nil {
		return nil, skerr.Wrapf(err, "Failed to parse pinpoint builds response body.")
	}

	commits := make([]*pb.BuildInfo, 0, len(legacyResp.Builds))
	for _, build := range legacyResp.Builds {
		if strings.EqualFold(build.Status, "success") && build.Input.GitilesCommit.ID != "" {
			var createdTime *timestamppb.Timestamp
			if build.CreateTime != "" {
				if t, err := time.Parse(time.RFC3339, build.CreateTime); err == nil {
					createdTime = timestamppb.New(t)
				} else {
					sklog.Warningf("Failed to parse build createTime %q: %s", build.CreateTime, err)
				}
			}
			commits = append(commits, &pb.BuildInfo{
				GitHash:     build.Input.GitilesCommit.ID,
				BuildNumber: int64(build.Number),
				Created:     createdTime,
			})
		}
	}

	return commits, nil
}

// GetCommit queries the legacy Pinpoint API to retrieve information about a commit.
func (pc *LegacyClient) GetCommit(
	ctx context.Context,
	commit string,
) (resp *pb.GetCommitResponse, err error) {
	params := url.Values{}
	params.Set("repository", "chromium")
	params.Set("git_hash", commit)
	requestURL := fmt.Sprintf("%s?%s", pinpointLegacyCommitURL, params.Encode())

	httpResp, err := pc.doPostRequest(ctx, requestURL)
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	body, err := pc.readResponseBody(httpResp)
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	var commitResp pb.GetCommitResponse
	if err = json.Unmarshal(body, &commitResp); err != nil {
		return nil, skerr.Wrapf(err, "Failed to parse pinpoint commit response body.")
	}

	return &commitResp, nil
}

// CancelJob calls the legacy Pinpoint API to cancel a job.
func (pc *LegacyClient) CancelJob(
	ctx context.Context,
	req *pb.CancelPinpointJobRequest,
) (resp *pb.CancelPinpointJobResponse, err error) {
	pc.cancelJobCalled.Inc(1)
	defer func() { trackError(pc.cancelJobFailed, err) }()

	if req.JobId == "" {
		return nil, skerr.Fmt("JobId must be specified but is empty.")
	}

	params := url.Values{}
	params.Set("job_id", req.JobId)
	params.Set("reason", DefaultCancellationReason)

	requestURL := fmt.Sprintf("%s?%s", pinpointLegacyCancelJobURL, params.Encode())
	httpResp, err := pc.doPostRequest(ctx, requestURL)
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	body, err := pc.readResponseBody(httpResp)
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	var legacyResp struct {
		JobID string `json:"job_id"`
		State string `json:"state"`
	}
	if err = json.Unmarshal(body, &legacyResp); err != nil {
		return nil, skerr.Wrapf(err, "Failed to parse pinpoint cancel response body.")
	}
	if legacyResp.JobID != req.JobId {
		return nil, skerr.Fmt("Mismatched Job ID in legacy cancel response: requested %q but got %q", req.JobId, legacyResp.JobID)
	}
	if parseJobStatus(legacyResp.State) != pb.JobStatus_JOB_STATUS_CANCELLED {
		return nil, skerr.Fmt("Unexpected job state in legacy cancel response: expected %q but got %q", "Cancelled", legacyResp.State)
	}
	return &pb.CancelPinpointJobResponse{}, nil
}
