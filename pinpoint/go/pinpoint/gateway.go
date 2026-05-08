package pinpoint

import (
	"context"
	"net/http"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"

	"go.skia.org/infra/go/skerr"
	pb "go.skia.org/infra/pinpoint/proto/v1"
)

type gatewayServer struct {
	pb.UnimplementedPinpointGatewayServer
	client *Client
}

// NewGatewayJSONHandler registers the http handlers for the PinpointGateway
// service and returns the handler.
func NewGatewayJSONHandler(ctx context.Context, client *Client) (http.Handler, error) {
	srv := &gatewayServer{
		client: client,
	}
	m := runtime.NewServeMux()
	if err := pb.RegisterPinpointGatewayHandlerServer(ctx, m, srv); err != nil {
		return nil, skerr.Wrapf(err, "unable to register pinpoint gateway handler")
	}
	return m, nil
}

func (s *gatewayServer) QueryJobList(
	ctx context.Context,
	req *pb.QueryJobListRequest,
) (*pb.QueryJobListResponse, error) {
	return s.client.QueryJobList(ctx, req)
}
