package pinpoint

import (
	"context"
	"net/http"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc/metadata"

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

func (s *gatewayServer) GetUserInfo(
	ctx context.Context,
	req *pb.GetUserInfoRequest,
) (*pb.GetUserInfoResponse, error) {
	email := ""
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		// When the request is from HTTP, like the Angular Web UI.
		if emails := md.Get("grpcmetadata-x-webauth-user"); len(emails) > 0 {
			email = emails[0]
			// A fallback in case the request is from a direct gRPC call.
		} else if emails := md.Get("x-webauth-user"); len(emails) > 0 {
			email = emails[0]
		}
	}
	return &pb.GetUserInfoResponse{
		Email: email,
	}, nil
}
