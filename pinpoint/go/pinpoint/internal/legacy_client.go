package internal

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
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
	pinpointLegacyBaseURL     = "https://pinpoint-dot-chromeperf.appspot.com"
	pinpointLegacyURL         = pinpointLegacyBaseURL + "/api/new"
	pinpointLegacyJobsURL     = pinpointLegacyBaseURL + "/api/jobs"
	pinpointLegacyJobStateURL = pinpointLegacyBaseURL + "/api/job"
	contentType               = "application/json"
	tryJobComparisonMode      = "try"
	chromeperfLegacyBisectURL = "https://chromeperf.appspot.com/pinpoint/new/bisect"
	legacyCreatedTimeLayout   = "2006-01-02T15:04:05.999999" // Layout used to parse legacy Pinpoint job creation time.
)

type LegacyClient struct {
	httpClient          *http.Client
	createBisectCalled  metrics2.Counter
	createBisectFailed  metrics2.Counter
	createTryJobCalled  metrics2.Counter
	createTryJobFailed  metrics2.Counter
	fetchJobStateCalled metrics2.Counter
	fetchJobStateFailed metrics2.Counter
	queryJobListCalled  metrics2.Counter
	queryJobListFailed  metrics2.Counter
}

// New returns a new LegacyClient instance.
func NewLegacyClient(ctx context.Context) (*LegacyClient, error) {
	tokenSource, err := google.DefaultTokenSource(ctx, auth.ScopeUserinfoEmail)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to create pinpoint client.")
	}

	client := httputils.DefaultClientConfig().WithTokenSource(tokenSource).Client()
	return &LegacyClient{
		httpClient:          client,
		createBisectCalled:  metrics2.GetCounter("pinpoint_create_bisect_called"),
		createBisectFailed:  metrics2.GetCounter("pinpoint_create_bisect_failed"),
		createTryJobCalled:  metrics2.GetCounter("pinpoint_create_try_job_called"),
		createTryJobFailed:  metrics2.GetCounter("pinpoint_create_try_job_failed"),
		fetchJobStateCalled: metrics2.GetCounter("pinpoint_fetch_job_state_called"),
		fetchJobStateFailed: metrics2.GetCounter("pinpoint_fetch_job_state_failed"),
		queryJobListCalled:  metrics2.GetCounter("pinpoint_query_job_list_called"),
		queryJobListFailed:  metrics2.GetCounter("pinpoint_query_job_list_failed"),
	}, nil
}

// CreateTryJob calls the legacy pinpoint API to create a try job.
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

// CreateBisect calls pinpoint API to create bisect job.
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
	return resp, err
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
	setIfNotEmpty(params, "bug_id", req.BugId)
	setIfNotEmpty(params, "user", req.User)
	params.Set("tags", "{\"origin\":\"skia_perf\"}")

	return fmt.Sprintf("%s?%s", pinpointLegacyURL, params.Encode()), nil
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
		errMsg := fmt.Sprintf(
			"Request to %s failed with status code %d and error: %s",
			url,
			resp.StatusCode,
			requestErrorMessage,
		)
		return nil, skerr.Wrap(errors.New(errMsg))
	}

	return body, err
}
