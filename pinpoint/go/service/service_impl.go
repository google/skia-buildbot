package service

import (
	"context"
	"net/http"
	"time"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"golang.org/x/time/rate"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	pb "go.skia.org/infra/pinpoint/proto/v1"
)

type server struct {
	pb.UnimplementedPinpointServer

	// Local rate limiter to only limit the traffic for migration temporarilly.
	limiter *rate.Limiter
}

func New(l *rate.Limiter) pb.PinpointServer {
	if l == nil {
		// 1 token every 30 minutes, this allow some buffer to drain the hot spots in the bots pool.
		l = rate.NewLimiter(rate.Every(30*time.Minute), 1)
	}
	return &server{
		limiter: l,
	}
}

func NewJSONHandler(ctx context.Context, srv pb.PinpointServer) (http.Handler, error) {
	m := runtime.NewServeMux()
	if err := pb.RegisterPinpointHandlerServer(ctx, m, srv); err != nil {
		return nil, skerr.Wrapf(err, "unable to register pinpoint handler")
	}
	return m, nil
}

// TODO(b/322047067)
//
//	embbed pb.UnimplementedPinpointServer will throw errors if those are not implemented.
func (s *server) ScheduleBisection(ctx context.Context, req *pb.ScheduleBisectRequest) (*pb.BisectExecution, error) {
	// Those logs are used to test traffic from existing services in catapult, shall be removed.
	sklog.Infof("Receiving bisection request: %v", req)
	if !s.limiter.Allow() {
		sklog.Infof("The request is dropped due to rate limiting.")
		return nil, skerr.Fmt("unable to fulfill the request due to rate limiting, dropping")
	}
	return s.UnimplementedPinpointServer.ScheduleBisection(ctx, req)
}

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
