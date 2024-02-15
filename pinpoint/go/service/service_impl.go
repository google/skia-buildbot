package service

import (
	"context"
	"net/http"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"

	"go.skia.org/infra/go/skerr"
	pb "go.skia.org/infra/pinpoint/proto/v1"
)

type server struct {
	pb.UnimplementedPinpointServer
}

func New() pb.PinpointServer {
	return &server{}
}

func NewJSONHandler(ctx context.Context, srv pb.PinpointServer) (http.Handler, error) {
	m := runtime.NewServeMux()
	if err := pb.RegisterPinpointHandlerServer(ctx, m, srv); err != nil {
		return nil, skerr.Wrapf(err, "unable to register pinpoint handler")
	}
	return m, nil
}

// TODO(b/322047067)
// embbed pb.UnimplementedPinpointServer will throw errors if those are not implemented.
// Uncomment to implememt
// func (s *server) ScheduleBisection(ctx context.Context, in *pb.ScheduleBisectRequest) (*pb.BisectExecution, error) {
// }

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
