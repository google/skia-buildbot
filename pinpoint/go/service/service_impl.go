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
