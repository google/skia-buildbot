package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.skia.org/infra/pinpoint/go/workflows"
	pb "go.skia.org/infra/pinpoint/proto/v1"
	tpr_client_mock "go.skia.org/infra/temporal/go/client/mocks"
	"go.temporal.io/api/enums/v1"
	workflowpb "go.temporal.io/api/workflow/v1"
	"go.temporal.io/api/workflowservice/v1"
	temporal_mocks "go.temporal.io/sdk/mocks"
	"golang.org/x/time/rate"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func newTemporalMock(t *testing.T) (*tpr_client_mock.TemporalProvider, *temporal_mocks.Client) {
	tcm := &temporal_mocks.Client{}
	tcm.Mock.Test(t)

	tpm := tpr_client_mock.NewTemporalProvider(t)
	t.Cleanup(func() {
		tcm.AssertExpectations(t)
		tpm.AssertExpectations(t)
	})
	return tpm, tcm
}

func newWorkflowRunMock(t *testing.T, wid string) *temporal_mocks.WorkflowRun {
	wfm := &temporal_mocks.WorkflowRun{}
	wfm.Test(t)
	wfm.On("GetID").Return(wid)
	t.Cleanup(func() { wfm.AssertExpectations(t) })
	return wfm
}

func TestScheduleBisection_ValidRequest_ReturnJobID(t *testing.T) {
	tpm, tcm := newTemporalMock(t)
	tpm.On("NewClient", mock.Anything, mock.Anything).Return(tcm, func() {}, nil)

	const fakeID = "fake-job-id"
	wfm := newWorkflowRunMock(t, fakeID)
	tcm.On("ExecuteWorkflow", mock.Anything, mock.Anything, workflows.CatapultBisect, mock.Anything).Return(wfm, nil)

	ctx := context.Background()
	svc := New(tpm, rate.NewLimiter(rate.Inf, 0))

	resp, err := svc.ScheduleBisection(ctx, &pb.ScheduleBisectRequest{
		StartGitHash: "fake-start",
		EndGitHash:   "fake-end",
	})
	assert.Equal(t, fakeID, resp.JobId)
	assert.NoError(t, err)
}

func TestScheduleBisection_RateLimitedRequests_ReturnError(t *testing.T) {
	tpm, _ := newTemporalMock(t)
	ctx := context.Background()
	svc := New(tpm, rate.NewLimiter(rate.Every(rateLimit), 1))

	_, err := svc.ScheduleBisection(ctx, &pb.ScheduleBisectRequest{})
	// invalid request should be rate limited.
	assert.ErrorContains(t, err, "git hash is empty")

	resp, err := svc.ScheduleBisection(ctx, &pb.ScheduleBisectRequest{})
	assert.Nil(t, resp)
	assert.ErrorContains(t, err, "unable to fulfill")
}

func TestScheduleBisection_InvalidRequests_ShouldError(t *testing.T) {
	tpm, _ := newTemporalMock(t)

	ctx := context.Background()
	svc := New(tpm, rate.NewLimiter(rate.Inf, 0))

	resp, err := svc.ScheduleBisection(ctx, &pb.ScheduleBisectRequest{})
	assert.Nil(t, resp)
	assert.ErrorContains(t, err, "git hash is empty")

	// TODO(b/322047067): Add requests with invalid fields
}

func TestScheduleCulpritFinder_ValidRequest_ReturnJobID(t *testing.T) {
	tpm, tcm := newTemporalMock(t)
	tpm.On("NewClient", mock.Anything, mock.Anything).Return(tcm, func() {}, nil).Once()

	const fakeID = "fake-job-id"
	wfm := newWorkflowRunMock(t, fakeID)
	tcm.On("ExecuteWorkflow", mock.Anything, mock.Anything, workflows.CulpritFinderWorkflow, mock.Anything).Return(wfm, nil)

	ctx := context.Background()
	svc := New(tpm, rate.NewLimiter(rate.Inf, 0))

	resp, err := svc.ScheduleCulpritFinder(ctx, &pb.ScheduleCulpritFinderRequest{
		StartGitHash:      "fake-start",
		EndGitHash:        "fake-end",
		Benchmark:         "speedometer3",
		Story:             "Speedometer3",
		Chart:             "Score",
		Configuration:     "mac-m1_mini_2020-perf",
		AggregationMethod: "avg", // technically should use mean, but catapult will use avg
	})
	assert.Equal(t, fakeID, resp.JobId)
	assert.NoError(t, err)
}

func TestScheduleCulpritFinder_RateLimitedRequests_ReturnError(t *testing.T) {
	tpm, tcm := newTemporalMock(t)
	tpm.On("NewClient", mock.Anything, mock.Anything).Return(tcm, func() {}, nil).Once()

	const fakeID = "fake-job-id"
	wfm := newWorkflowRunMock(t, fakeID)
	tcm.On("ExecuteWorkflow", mock.Anything, mock.Anything, workflows.CulpritFinderWorkflow, mock.Anything).Return(wfm, nil)

	ctx := context.Background()
	svc := New(tpm, rate.NewLimiter(rate.Every(rateLimit), 1))

	resp, err := svc.ScheduleCulpritFinder(ctx, &pb.ScheduleCulpritFinderRequest{
		StartGitHash:  "fake-start",
		EndGitHash:    "fake-end",
		Benchmark:     "speedometer3",
		Story:         "Speedometer3",
		Chart:         "Score",
		Configuration: "mac-m1_mini_2020-perf",
	})
	assert.Equal(t, fakeID, resp.JobId)
	assert.NoError(t, err)

	resp, err = svc.ScheduleCulpritFinder(ctx, &pb.ScheduleCulpritFinderRequest{})
	assert.Nil(t, resp)
	assert.ErrorContains(t, err, "unable to fulfill")
}

func TestScheduleCulpritFinder_BadRequestParams_JobBlocked(t *testing.T) {
	test := func(name string, req *pb.ScheduleCulpritFinderRequest, errMsg string) {
		t.Run(name, func(t *testing.T) {
			tpm, _ := newTemporalMock(t)

			ctx := context.Background()
			svc := New(tpm, rate.NewLimiter(rate.Inf, 0))

			resp, err := svc.ScheduleCulpritFinder(ctx, req)
			assert.Nil(t, resp)
			assert.ErrorContains(t, err, errMsg)
		})
	}

	req := &pb.ScheduleCulpritFinderRequest{
		StartGitHash:  "fake-start",
		EndGitHash:    "fake-end",
		Benchmark:     "",
		Story:         "Speedometer3",
		Chart:         "Score",
		Configuration: "mac-m1_mini_2020-perf",
	}
	test("empty benchmark", req, "benchmark is empty")

	req = &pb.ScheduleCulpritFinderRequest{
		StartGitHash:  "",
		EndGitHash:    "fake-end",
		Benchmark:     "speedometer3",
		Story:         "Speedometer3",
		Chart:         "Score",
		Configuration: "mac-m1_mini_2020-perf",
	}
	test("empty git hash", req, "git hash is empty")

	req = &pb.ScheduleCulpritFinderRequest{
		StartGitHash:  "fake-start",
		EndGitHash:    "fake-end",
		Benchmark:     "speedometer3",
		Story:         "",
		Chart:         "Score",
		Configuration: "mac-m1_mini_2020-perf",
	}
	test("empty story", req, "story is empty")

	req = &pb.ScheduleCulpritFinderRequest{
		StartGitHash:  "fake-start",
		EndGitHash:    "fake-end",
		Benchmark:     "speedometer3",
		Story:         "Speedometer3",
		Chart:         "",
		Configuration: "mac-m1_mini_2020-perf",
	}
	test("empty chart", req, "chart is empty")

	req = &pb.ScheduleCulpritFinderRequest{
		StartGitHash:  "fake-start",
		EndGitHash:    "fake-end",
		Benchmark:     "speedometer3",
		Story:         "Speedometer3",
		Chart:         "Score",
		Configuration: "",
	}
	test("empty config", req, "configuration (aka the device name) is empty")

	req = &pb.ScheduleCulpritFinderRequest{
		StartGitHash:  "fake-start",
		EndGitHash:    "fake-end",
		Benchmark:     "speedometer3",
		Story:         "Speedometer3",
		Chart:         "Score",
		Configuration: "android-pixel-fold-perf",
	}
	test("invalid bot", req, "bot (android-pixel-fold-perf) is currently unsupported")

	req = &pb.ScheduleCulpritFinderRequest{
		StartGitHash:      "fake-start",
		EndGitHash:        "fake-end",
		Benchmark:         "speedometer3",
		Story:             "Speedometer3",
		Chart:             "Score",
		Configuration:     "mac-m1_mini_2020-perf",
		AggregationMethod: "fake-aggregation-method",
	}
	test("invalid aggregation method", req, "aggregation method (fake-aggregation-method) is not available")
}

func TestQueryBisection_ExistingJob_ShouldReturnDetails(t *testing.T) {
	tpm, _ := newTemporalMock(t)
	ctx := context.Background()
	svc := New(tpm, rate.NewLimiter(rate.Inf, 0))

	expect := func(req *pb.QueryBisectRequest, want *pb.BisectExecution, desc string) {
		resp, err := svc.QueryBisection(ctx, req)
		// TODO(b/322047067): remove this once implemented, err should be nil
		assert.ErrorContains(t, err, "not implemented")

		// TODO(b/322047067): the response should match the expected
		assert.Nil(t, resp, desc)
	}

	// TODO(b/322047067): Add more combinations of query request and fix expected responses.
	expect(&pb.QueryBisectRequest{
		JobId: "TBD ID",
	}, nil, "should return job status")

}

func TestQueryBisection_NonExistingJob_ShouldError(t *testing.T) {
	tpm, _ := newTemporalMock(t)
	ctx := context.Background()
	svc := New(tpm, rate.NewLimiter(rate.Inf, 0))

	resp, err := svc.QueryBisection(ctx, &pb.QueryBisectRequest{
		JobId: "non-exist ID",
	})
	// TODO(b/322047067): change this to correct error message
	assert.ErrorContains(t, err, "not implemented", "Error should indicate job doesn't exist.")
	assert.Nil(t, resp, "Non-existed Job ID shouldn't contain any response.")

	resp, err = svc.QueryBisection(ctx, &pb.QueryBisectRequest{})
	// TODO(b/322047067): change this to correct error message
	assert.ErrorContains(t, err, "not implemented", "Empty Job ID should error.")
	assert.Nil(t, resp, "Empty Job ID shouldn't contain any response.")
}

func TestCancelJob_InvalidInput_ReturnError(t *testing.T) {
	tpm, _ := newTemporalMock(t)
	ctx := context.Background()
	svc := New(tpm, rate.NewLimiter(rate.Every(time.Hour), 1))

	_, err := svc.CancelJob(ctx, &pb.CancelJobRequest{JobId: "job-id"})
	assert.ErrorContains(t, err, "bad request: missing Reason")

	_, err = svc.CancelJob(ctx, &pb.CancelJobRequest{Reason: "cancel reason"})
	assert.ErrorContains(t, err, "bad request: missing JobId")
}

func TestCancelJob_JobCancelFailed_ReturnError(t *testing.T) {
	tpm, tcm := newTemporalMock(t)
	tpm.On("NewClient", mock.Anything, mock.Anything).Return(tcm, func() {}, nil)

	tcm.On("CancelWorkflow", mock.Anything, mock.Anything, mock.Anything).Return(errors.New("internal error"))

	ctx := context.Background()
	svc := New(tpm, rate.NewLimiter(rate.Every(time.Hour), 1))

	resp, err := svc.CancelJob(ctx, &pb.CancelJobRequest{JobId: "job-id", Reason: "cancel reason"})
	assert.Nil(t, resp)
	assert.ErrorContains(t, err, "Unable to cancel workflow")
	assert.ErrorContains(t, err, "internal error")
}

func TestCancelJob_JobCancelled_ReturnSucceed(t *testing.T) {
	tpm, tcm := newTemporalMock(t)
	tpm.On("NewClient", mock.Anything, mock.Anything).Return(tcm, func() {}, nil)

	tcm.On("CancelWorkflow", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	ctx := context.Background()
	svc := New(tpm, rate.NewLimiter(rate.Every(time.Hour), 1))

	resp, err := svc.CancelJob(ctx, &pb.CancelJobRequest{JobId: "job-id", Reason: "cancel reason"})
	assert.Nil(t, err)
	assert.Equal(t, resp.JobId, "job-id")
	assert.Equal(t, resp.State, "Cancelled")
}

func TestQueryPairwise_InvalidRequest_ReturnsInternalError(t *testing.T) {
	tpm, _ := newTemporalMock(t)
	ctx := context.Background()
	svc := New(tpm, rate.NewLimiter(rate.Inf, 0))

	resp, err := svc.QueryPairwise(ctx, &pb.QueryPairwiseRequest{JobId: ""})
	assert.Nil(t, resp)
	st, ok := status.FromError(err)
	assert.True(t, ok)
	assert.Equal(t, codes.InvalidArgument, st.Code())
	assert.Contains(t, st.Message(), "invalid request")
}

func TestQueryPairwise_TemporalClientError_ReturnsClientError(t *testing.T) {
	tpm, _ := newTemporalMock(t)
	ctx := context.Background()
	svc := New(tpm, rate.NewLimiter(rate.Inf, 0))

	expectedErr := errors.New("temporal client failed")
	tpm.On("NewClient", mock.Anything, mock.Anything).Return(nil, func() {}, expectedErr)

	resp, err := svc.QueryPairwise(ctx, &pb.QueryPairwiseRequest{JobId: "189ee4a7-fe14-4472-81eb-d201b17ddd9b"})
	assert.Nil(t, resp)
	st, ok := status.FromError(err)
	assert.True(t, ok)
	assert.Equal(t, codes.Internal, st.Code())
	assert.Contains(t, st.Message(), "Unable to connect to Temporal")
	assert.Contains(t, st.Message(), expectedErr.Error())
}

func TestQueryPairwise_DescribeWorkflow_ReturnsInternalError(t *testing.T) {
	tpm, tcm := newTemporalMock(t)
	ctx := context.Background()
	svc := New(tpm, rate.NewLimiter(rate.Inf, 0))

	jobID := "189ee4a7-fe14-4472-81eb-d201b17ddd9b"
	expectedErr := errors.New("describe workflow failed")

	tpm.On("NewClient", mock.Anything, mock.Anything).Return(tcm, func() {}, nil)
	tcm.On("DescribeWorkflowExecution", mock.Anything, jobID, "").Return(nil, expectedErr)

	resp, err := svc.QueryPairwise(ctx, &pb.QueryPairwiseRequest{JobId: jobID})
	assert.Nil(t, resp)
	st, ok := status.FromError(err)
	assert.True(t, ok)
	assert.Equal(t, codes.Internal, st.Code())
	assert.Contains(t, st.Message(), "Pairwise workflow execution returned unknown status")
}

func TestQueryPairwise_StatusCompleted_ReturnsCompleted(t *testing.T) {
	tpm, tcm := newTemporalMock(t)
	wfrMock := &temporal_mocks.WorkflowRun{}
	wfrMock.Test(t)

	ctx := context.Background()
	svc := New(tpm, rate.NewLimiter(rate.Inf, 0))

	jobID := "189ee4a7-fe14-4472-81eb-d201b17ddd9b"
	expectedExecution := &pb.PairwiseExecution{
		JobId: jobID,
		Results: map[string]*pb.PairwiseExecution_WilcoxonResult{
			"load_time": {
				PValue:                   0.04,
				ConfidenceIntervalLower:  100.0,
				ConfidenceIntervalHigher: 150.0,
				ControlMedian:            120.0,
				TreatmentMedian:          125.0,
			},
		},
	}

	tpm.On("NewClient", mock.Anything, mock.Anything).Return(tcm, func() {}, nil)
	describeResp := &workflowservice.DescribeWorkflowExecutionResponse{
		WorkflowExecutionInfo: &workflowpb.WorkflowExecutionInfo{
			Status: enums.WORKFLOW_EXECUTION_STATUS_COMPLETED,
		},
	}
	tcm.On("DescribeWorkflowExecution", mock.Anything, jobID, "").Return(describeResp, nil)
	tcm.On("GetWorkflow", mock.Anything, jobID, "").Return(wfrMock)

	wfrMock.On("Get", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		arg := args.Get(1).(*pb.PairwiseExecution)
		*arg = pb.PairwiseExecution{
			JobId: jobID,
			Results: map[string]*pb.PairwiseExecution_WilcoxonResult{
				"load_time": {
					PValue:                   0.04,
					ConfidenceIntervalLower:  100.0,
					ConfidenceIntervalHigher: 150.0,
					ControlMedian:            120.0,
					TreatmentMedian:          125.0,
				},
			},
		}
	}).Return(nil)

	resp, err := svc.QueryPairwise(ctx, &pb.QueryPairwiseRequest{JobId: jobID})
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, pb.PairwiseJobStatus_PAIRWISE_JOB_STATUS_COMPLETED, resp.Status)
	assert.Equal(t, expectedExecution, resp.Execution)

	wfrMock.AssertExpectations(t)
}

func TestQueryPairwise_StatusCompleted_ReturnsInternalError(t *testing.T) {
	tpm, tcm := newTemporalMock(t)
	wfrMock := &temporal_mocks.WorkflowRun{}
	wfrMock.Test(t)
	ctx := context.Background()
	svc := New(tpm, rate.NewLimiter(rate.Inf, 0))

	jobID := "189ee4a7-fe14-4472-81eb-d201b17ddd9b"
	getError := errors.New("failed to get workflow results")

	tpm.On("NewClient", mock.Anything, mock.Anything).Return(tcm, func() {}, nil)
	describeResp := &workflowservice.DescribeWorkflowExecutionResponse{
		WorkflowExecutionInfo: &workflowpb.WorkflowExecutionInfo{
			Status: enums.WORKFLOW_EXECUTION_STATUS_COMPLETED,
		},
	}
	tcm.On("DescribeWorkflowExecution", mock.Anything, jobID, "").Return(describeResp, nil)
	tcm.On("GetWorkflow", mock.Anything, jobID, "").Return(wfrMock)
	wfrMock.On("Get", mock.Anything, mock.Anything).Return(getError)

	resp, err := svc.QueryPairwise(ctx, &pb.QueryPairwiseRequest{JobId: jobID})
	assert.Nil(t, resp)
	st, ok := status.FromError(err)
	assert.True(t, ok)
	assert.Equal(t, codes.Internal, st.Code())
	assert.Contains(t, st.Message(), "Pairwise workflow completed, but failed to get results")
	assert.Contains(t, st.Message(), getError.Error())

	wfrMock.AssertExpectations(t)
}

func TestQueryPairwise_OtherStatuses_ReturnsCorrectJobStatus(t *testing.T) {
	tests := []struct {
		name           string
		workflowStatus enums.WorkflowExecutionStatus
		expectedStatus pb.PairwiseJobStatus
	}{
		{"Failed", enums.WORKFLOW_EXECUTION_STATUS_FAILED, *pb.PairwiseJobStatus_PAIRWISE_JOB_STATUS_FAILED.Enum()},
		{"TimedOut", enums.WORKFLOW_EXECUTION_STATUS_TIMED_OUT, *pb.PairwiseJobStatus_PAIRWISE_JOB_STATUS_FAILED.Enum()},
		{"Terminated", enums.WORKFLOW_EXECUTION_STATUS_TERMINATED, *pb.PairwiseJobStatus_PAIRWISE_JOB_STATUS_FAILED.Enum()},
		{"Canceled", enums.WORKFLOW_EXECUTION_STATUS_CANCELED, *pb.PairwiseJobStatus_PAIRWISE_JOB_STATUS_CANCELED.Enum()},
		{"Running", enums.WORKFLOW_EXECUTION_STATUS_RUNNING, pb.PairwiseJobStatus_PAIRWISE_JOB_STATUS_RUNNING},
		{"ContinuedAsNew", enums.WORKFLOW_EXECUTION_STATUS_CONTINUED_AS_NEW, pb.PairwiseJobStatus_PAIRWISE_JOB_STATUS_RUNNING},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tpm, tcm := newTemporalMock(t)
			ctx := context.Background()
			svc := New(tpm, rate.NewLimiter(rate.Inf, 0))

			jobID := "189ee4a7-fe14-4472-81eb-d201b17ddd9b"

			tpm.On("NewClient", mock.Anything, mock.Anything).Return(tcm, func() {}, nil)
			describeResp := &workflowservice.DescribeWorkflowExecutionResponse{
				WorkflowExecutionInfo: &workflowpb.WorkflowExecutionInfo{
					Status: tc.workflowStatus,
				},
			}
			tcm.On("DescribeWorkflowExecution", mock.Anything, jobID, "").Return(describeResp, nil)

			resp, err := svc.QueryPairwise(ctx, &pb.QueryPairwiseRequest{JobId: jobID})
			assert.NoError(t, err)
			assert.NotNil(t, resp)
			assert.Equal(t, tc.expectedStatus, resp.Status)
			assert.Nil(t, resp.Execution)
		})
	}
}

func TestQueryPairwise_TemporalWorkflowStatusIsUnspecified_ReturnsInternalError(t *testing.T) {
	tpm, tcm := newTemporalMock(t)
	ctx := context.Background()
	svc := New(tpm, rate.NewLimiter(rate.Inf, 0))

	jobID := "189ee4a7-fe14-4472-81eb-d201b17ddd9b"

	tpm.On("NewClient", mock.Anything, mock.Anything).Return(tcm, func() {}, nil)
	describeResp := &workflowservice.DescribeWorkflowExecutionResponse{
		WorkflowExecutionInfo: &workflowpb.WorkflowExecutionInfo{
			Status: enums.WORKFLOW_EXECUTION_STATUS_UNSPECIFIED,
		},
	}
	tcm.On("DescribeWorkflowExecution", mock.Anything, jobID, "").Return(describeResp, nil)

	resp, err := svc.QueryPairwise(ctx, &pb.QueryPairwiseRequest{JobId: jobID})
	assert.Nil(t, resp)
	st, ok := status.FromError(err)
	assert.True(t, ok)
	assert.Equal(t, codes.Internal, st.Code())
	assert.Contains(t, st.Message(), "Pairwise workflow execution returned unknown status")
}
