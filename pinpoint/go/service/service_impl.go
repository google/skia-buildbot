package service

import (
	"context"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/temporal"
	"golang.org/x/time/rate"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/pinpoint/go/workflows"
	pb "go.skia.org/infra/pinpoint/proto/v1"
	tpr_client "go.skia.org/infra/temporal/go/client"
	enumspb "go.temporal.io/api/enums/v1"
)

type server struct {
	pb.UnimplementedPinpointServer

	// Local rate limiter to only limit the traffic for migration temporarilly.
	limiter *rate.Limiter

	temporal tpr_client.TemporalProvider
}

const (
	// Those should be configurable for each instance.
	hostPort  = "localhost:7233"
	namespace = "perf-internal"
	taskQueue = "perf.perf-chrome-public.bisect"
	// TODO(b/352631333): Replace the rate limit with an actual queueing system
	rateLimit = 30 * time.Minute // accept 1 request every 30 minutes
	// arbitrary timeouts to ensure workflows will not run forever. Note that it is possible
	// for a job to naturally timeout due to long running time. Consider updating these
	// if too many jobs time out.
	pairwiseTimeoutDuration = 2 * time.Hour
	bisectionTimeout        = 12 * time.Hour
	culpritFinderTimeout    = 16 * time.Hour
)

func New(t tpr_client.TemporalProvider, l *rate.Limiter) *server {
	if l == nil {
		// 1 token every 30 minutes, this allow some buffer to drain the hot spots in the bots pool.
		l = rate.NewLimiter(rate.Every(rateLimit), 1)
	}
	if t == nil {
		t = tpr_client.DefaultTemporalProvider{}
	}
	return &server{
		limiter:  l,
		temporal: t,
	}
}

func NewJSONHandler(ctx context.Context, srv pb.PinpointServer) (http.Handler, error) {
	m := runtime.NewServeMux()
	if err := pb.RegisterPinpointHandlerServer(ctx, m, srv); err != nil {
		return nil, skerr.Wrapf(err, "unable to register pinpoint handler")
	}
	return m, nil
}

func (s *server) CancelJob(ctx context.Context, req *pb.CancelJobRequest) (*pb.CancelJobResponse, error) {
	sklog.Infof("Receiving cancel job request: %v", req)
	if req.JobId == "" {
		return nil, status.Errorf(codes.InvalidArgument, "bad request: missing JobId")
	}

	if req.Reason == "" {
		return nil, status.Errorf(codes.InvalidArgument, "bad request: missing Reason")
	}

	c, cleanUp, err := s.temporal.NewClient(hostPort, namespace)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Unable to connect to Temporal (%v).", err)
	}

	defer cleanUp()

	err = c.CancelWorkflow(ctx, req.JobId, "")
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Unable to cancel workflow (%v).", err)
	}
	return &pb.CancelJobResponse{JobId: req.JobId, State: "Cancelled"}, nil
}

func (s *server) LegacyJobQuery(ctx context.Context, req *pb.LegacyJobRequest) (*pb.LegacyJobResponse, error) {
	qresp, err := s.QueryBisection(ctx, &pb.QueryBisectRequest{
		JobId: req.GetJobId(),
	})
	if err != nil {
		// We don't skerr.Wrap here because we expect to populate err with status.code back to
		// the client, this is automatic conversion to REST API status code when this is exposed
		// via grpc-gateway.
		// Note this API is only intermediate and will be gone, this is not considered to be
		// best practise.
		return nil, err
	}

	// TODO(b/318864009): convert BisectExecution to LegacyJobResponse
	// This should be just a copy.
	resp := &pb.LegacyJobResponse{
		JobId: qresp.GetJobId(),
	}
	return resp, nil
}

func (s *server) ScheduleBisection(ctx context.Context, req *pb.ScheduleBisectRequest) (*pb.BisectExecution, error) {
	// Those logs are used to test traffic from existing services in catapult, shall be removed.
	sklog.Infof("Receiving bisection request: %v", req)
	if !s.limiter.Allow() {
		sklog.Infof("The request is dropped due to rate limiting.")
		return nil, status.Errorf(codes.ResourceExhausted, "unable to fulfill the request due to rate limiting, dropping")
	}

	// TODO(b/318864009): Remove this function once Pinpoint migration is completed.
	req = updateFieldsForCatapult(req)

	if err := validateBisectRequest(req); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	c, cleanUp, err := s.temporal.NewClient(hostPort, namespace)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Unable to connect to Temporal (%v).", err)
	}

	defer cleanUp()

	wo := client.StartWorkflowOptions{
		ID:                       uuid.New().String(),
		TaskQueue:                taskQueue,
		WorkflowExecutionTimeout: bisectionTimeout,
		RetryPolicy: &temporal.RetryPolicy{
			// We will only attempt to run the workflow exactly once as we expect any failure will be
			// not retry-recoverable failure.
			MaximumAttempts: 1,
		},
	}
	wf, err := c.ExecuteWorkflow(ctx, wo, workflows.CatapultBisect, &workflows.BisectParams{
		Request:    req,
		Production: true,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Unable to start workflow (%v).", err)
	}
	return &pb.BisectExecution{JobId: wf.GetID()}, nil
}

func (s *server) ScheduleCulpritFinder(ctx context.Context, req *pb.ScheduleCulpritFinderRequest) (*pb.CulpritFinderExecution, error) {
	// Those logs are used to test traffic from existing services in catapult, shall be removed.
	sklog.Infof("Receiving culprit finder request: %v", req)
	if !s.limiter.Allow() {
		sklog.Infof("The request is dropped due to rate limiting.")
		return nil, status.Errorf(codes.ResourceExhausted, "unable to fulfill the request due to rate limiting, dropping")
	}

	// TODO(b/318864009): Remove this function once Pinpoint migration is completed.
	req = updateCulpritFinderFieldsForCatapult(req)

	if err := validateCulpritFinderRequest(req); err != nil {
		sklog.Warningf("the request failed validation due to %v", err.Error())
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	c, cleanUp, err := s.temporal.NewClient(hostPort, namespace)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Unable to connect to Temporal (%v).", err)
	}

	defer cleanUp()

	wo := client.StartWorkflowOptions{
		ID:        uuid.New().String(),
		TaskQueue: taskQueue,
		// arbitrary timeout to assure it's not going forever. Set to a few hours more than
		// bisect timeout.
		WorkflowExecutionTimeout: culpritFinderTimeout,
		RetryPolicy: &temporal.RetryPolicy{
			// We will only attempt to run the workflow exactly once as we expect any failure will be
			// not retry-recoverable failure.
			MaximumAttempts: 1,
		},
	}
	wf, err := c.ExecuteWorkflow(ctx, wo, workflows.CulpritFinderWorkflow, &workflows.CulpritFinderParams{
		Request:    req,
		Production: true,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Unable to start workflow (%v).", err)
	}

	return &pb.CulpritFinderExecution{JobId: wf.GetID()}, nil
}

func (s *server) SchedulePairwise(ctx context.Context, req *pb.SchedulePairwiseRequest) (*pb.PairwiseExecution, error) {
	// TODO(b/391717876) - remove log once migration complete, as these logs can
	// get noisy.
	sklog.Infof("Pairwise (try) request received: %v", req)

	if !s.limiter.Allow() {
		sklog.Infof("The request is dropped due to rate limiting.")
		return nil, status.Errorf(codes.ResourceExhausted, "unable to fulfill the request due to rate limiting, dropping")
	}

	if err := validatePairwiseRequest(req); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid request")
	}

	c, cleanUp, err := s.temporal.NewClient(hostPort, namespace)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Unable to connect to Temporal (%v).", err)
	}

	defer cleanUp()

	workflowOptions := client.StartWorkflowOptions{
		ID:        uuid.New().String(),
		TaskQueue: taskQueue,
		// A pairwise try job SLO is to complete sub 2 hours.
		WorkflowExecutionTimeout: pairwiseTimeoutDuration,
		RetryPolicy: &temporal.RetryPolicy{
			// We will only attempt to run the workflow exactly once as we expect any failure will be
			// not retry-recoverable failure.
			MaximumAttempts: 1,
		},
	}

	workflowRun, err := c.ExecuteWorkflow(ctx, workflowOptions, workflows.PairwiseWorkflow, &workflows.PairwiseParams{
		Request:       req,
		CulpritVerify: false,
	})

	return &pb.PairwiseExecution{
		JobId: workflowRun.GetID(),
	}, nil
}

func (s *server) QueryPairwise(ctx context.Context, req *pb.QueryPairwiseRequest) (*pb.QueryPairwiseResponse, error) {
	sklog.Infof("Query Pairwise (try) request received: %v", req)

	if err := validateQueryPairwiseRequest(req); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid request")
	}

	c, cleanUp, err := s.temporal.NewClient(hostPort, namespace)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Unable to connect to Temporal (%v).", err)
	}

	defer cleanUp()

	resp, err := c.DescribeWorkflowExecution(ctx, req.JobId, "")

	workflowStatus := resp.GetWorkflowExecutionInfo().GetStatus()

	switch workflowStatus {
	case enumspb.WORKFLOW_EXECUTION_STATUS_COMPLETED:

		workflowRun := c.GetWorkflow(ctx, req.JobId, "")

		var pairwiseExecution pb.PairwiseExecution

		errGet := workflowRun.Get(ctx, &pairwiseExecution)
		if errGet != nil {
			return nil, status.Errorf(codes.Internal, "Pairwise workflow completed, but failed to get results (runID: '%s'): %v",
				req.JobId, errGet)
		}

		return &pb.QueryPairwiseResponse{
			Status:    pb.PairwiseJobStatus_PAIRWISE_JOB_STATUS_COMPLETED,
			Execution: &pairwiseExecution,
		}, nil

	case enumspb.WORKFLOW_EXECUTION_STATUS_FAILED,
		enumspb.WORKFLOW_EXECUTION_STATUS_TIMED_OUT,
		enumspb.WORKFLOW_EXECUTION_STATUS_TERMINATED:

		return &pb.QueryPairwiseResponse{
			Status:    pb.PairwiseJobStatus_PAIRWISE_JOB_STATUS_FAILED,
			Execution: nil,
		}, nil

	case enumspb.WORKFLOW_EXECUTION_STATUS_CANCELED:
		return &pb.QueryPairwiseResponse{
			Status:    pb.PairwiseJobStatus_PAIRWISE_JOB_STATUS_CANCELED,
			Execution: nil,
		}, nil

	case enumspb.WORKFLOW_EXECUTION_STATUS_RUNNING,
		enumspb.WORKFLOW_EXECUTION_STATUS_CONTINUED_AS_NEW:
		return &pb.QueryPairwiseResponse{
			Status:    pb.PairwiseJobStatus_PAIRWISE_JOB_STATUS_RUNNING,
			Execution: nil,
		}, nil
	}

	return nil, status.Errorf(codes.Internal, "Pairwise workflow execution returned unknown status.")
}
