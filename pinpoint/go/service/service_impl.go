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
	"go.skia.org/infra/pinpoint/go/read_values"
	"go.skia.org/infra/pinpoint/go/workflows"
	pb "go.skia.org/infra/pinpoint/proto/v1"
	tpr_client "go.skia.org/infra/temporal/go/client"
)

type server struct {
	pb.UnimplementedPinpointServer

	// Local rate limiter to only limit the traffic for migration temporarilly.
	limiter *rate.Limiter

	temporal tpr_client.TemporalProvider
}

const (
	// Those should be configurable for each instance.
	hostPort  = "temporal.temporal:7233"
	namespace = "perf-internal"
	taskQueue = "perf.perf-chrome-public.bisect"
	// TODO(b/352631333): Replace the rate limit with an actual queueing system
	rateLimit = 30 * time.Minute // accept 1 request every 30 minutes
	// arbitrary timeouts to ensure workflows will not run forever. Note that it is possible
	// for a job to naturally timeout due to long running time. Consider updating these
	// if too many jobs time out.
	bisectionTimeout     = 12 * time.Hour
	culpritFinderTimeout = 16 * time.Hour
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

// updateFieldsForCatapult converts specific catapult Pinpoint arguments
// to their skia Pinpoint counterparts
func updateFieldsForCatapult(req *pb.ScheduleBisectRequest) *pb.ScheduleBisectRequest {
	switch {
	case req.Statistic == "avg":
		req.AggregationMethod = "mean"
	case req.Statistic != "":
		req.AggregationMethod = req.Statistic
	}
	return req
}

func validate(req *pb.ScheduleBisectRequest) error {
	switch {
	case req.StartGitHash == "" || req.EndGitHash == "":
		return skerr.Fmt("git hash is empty")
	case !read_values.IsSupportedAggregation(req.AggregationMethod):
		return skerr.Fmt("aggregation method (%s) is not available", req.AggregationMethod)
	default:
		return nil
	}
}

// updateCulpritFinderFieldsForCatapult converts specific catapult Pinpoint arguments
// to their skia Pinpoint counterparts
func updateCulpritFinderFieldsForCatapult(req *pb.ScheduleCulpritFinderRequest) *pb.ScheduleCulpritFinderRequest {
	switch {
	case req.Statistic == "avg":
		req.AggregationMethod = "mean"
	case req.Statistic != "":
		req.AggregationMethod = req.Statistic
	case req.AggregationMethod == "avg":
		req.AggregationMethod = "mean"
	}
	return req
}

func validateCulpritFinder(req *pb.ScheduleCulpritFinderRequest) error {
	switch {
	case req.StartGitHash == "" || req.EndGitHash == "":
		return skerr.Fmt("git hash is empty")
	case req.Benchmark == "":
		return skerr.Fmt("benchmark is empty")
	case req.Story == "":
		return skerr.Fmt("story is empty")
	case req.Chart == "":
		return skerr.Fmt("chart is empty")
	case req.Configuration == "":
		return skerr.Fmt("configuration (aka the device name) is empty")
	case req.Configuration == "android-pixel-fold-perf" || req.Configuration == "mac-m1-pro-perf":
		return skerr.Fmt("bot (%s) is currently unsupported due to low resources", req.Configuration)
	case !read_values.IsSupportedAggregation(req.AggregationMethod):
		return skerr.Fmt("aggregation method (%s) is not available", req.AggregationMethod)
	}
	return nil
}

func NewJSONHandler(ctx context.Context, srv pb.PinpointServer) (http.Handler, error) {
	m := runtime.NewServeMux()
	if err := pb.RegisterPinpointHandlerServer(ctx, m, srv); err != nil {
		return nil, skerr.Wrapf(err, "unable to register pinpoint handler")
	}
	return m, nil
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

	if err := validate(req); err != nil {
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

	if err := validateCulpritFinder(req); err != nil {
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

// TODO(b/322047067)
//	embbed pb.UnimplementedPinpointServer will throw errors if those are not implemented.
// func (s *server) QueryBisection(ctx context.Context, in *pb.QueryBisectRequest) (*pb.BisectExecution, error) {
// }

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
