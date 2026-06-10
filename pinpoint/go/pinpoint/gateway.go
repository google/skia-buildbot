package pinpoint

import (
	"context"
	"net/http"
	"strings"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc/metadata"

	"go.skia.org/infra/go/skerr"
	pb "go.skia.org/infra/pinpoint/proto/v1"
)

type pinpointClient interface {
	QueryJobList(ctx context.Context, req *pb.QueryJobListRequest) (*pb.QueryJobListResponse, error)
	CreatePinpointTryJob(ctx context.Context, req *pb.CreateTryJobRequest) (*pb.CreateJobResponse, error)
	ListBotConfigurations(ctx context.Context) ([]string, error)
	ListBenchmarks(ctx context.Context) ([]string, error)
}

type gatewayServer struct {
	pb.UnimplementedPinpointGatewayServer
	client pinpointClient
}

// NewGatewayJSONHandler registers the http handlers for the PinpointGateway
// service and returns the handler.
func NewGatewayJSONHandler(ctx context.Context, client *Client) (http.Handler, error) {
	srv := &gatewayServer{
		client: client,
	}
	m := runtime.NewServeMux(
		runtime.WithIncomingHeaderMatcher(func(key string) (string, bool) {
			if strings.EqualFold(key, "x-webauth-user") {
				return "x-webauth-user", true
			}
			return runtime.DefaultHeaderMatcher(key)
		}),
	)
	if err := pb.RegisterPinpointGatewayHandlerServer(ctx, m, srv); err != nil {
		return nil, skerr.Wrapf(err, "unable to register pinpoint gateway handler")
	}
	return m, nil
}

func (s *gatewayServer) QueryJobList(
	ctx context.Context,
	req *pb.QueryJobListRequest,
) (*pb.QueryJobListResponse, error) {
	resp, err := s.client.QueryJobList(ctx, req)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return resp, nil
}

func getEmailFromContext(ctx context.Context) string {
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		// When the request is from HTTP, like the Angular Web UI.
		if emails := md.Get("grpcmetadata-x-webauth-user"); len(emails) > 0 {
			return emails[0]
		}
		// A fallback in case the request is from a direct gRPC call.
		if emails := md.Get("x-webauth-user"); len(emails) > 0 {
			return emails[0]
		}
	}
	return ""
}

func (s *gatewayServer) GetUserInfo(
	ctx context.Context,
	req *pb.GetUserInfoRequest,
) (*pb.GetUserInfoResponse, error) {
	return &pb.GetUserInfoResponse{
		Email: getEmailFromContext(ctx),
	}, nil
}

func (s *gatewayServer) CreateTryJob(
	ctx context.Context,
	req *pb.CreateTryJobRequest,
) (*pb.CreateJobResponse, error) {
	if req.User == "" {
		req.User = getEmailFromContext(ctx)
	}

	resp, err := s.client.CreatePinpointTryJob(ctx, req)
	if err != nil {
		// Unwrap the error because it may be displayed to the user.
		return nil, skerr.Unwrap(err)
	}
	return resp, nil
}

func (s *gatewayServer) ListBotConfigurations(
	ctx context.Context,
	req *pb.ListBotConfigurationsRequest,
) (*pb.ListBotConfigurationsResponse, error) {
	bots, err := s.client.ListBotConfigurations(ctx)
	if err != nil {
		// Unwrap the error because it may be displayed to the user.
		return nil, skerr.Unwrap(err)
	}
	return &pb.ListBotConfigurationsResponse{
		Configurations: bots,
	}, nil
}

func (s *gatewayServer) ListBenchmarks(
	ctx context.Context,
	req *pb.ListBenchmarksRequest,
) (*pb.ListBenchmarksResponse, error) {
	benchmarks, err := s.client.ListBenchmarks(ctx)
	if err != nil {
		// Unwrap the error because it may be displayed to the user.
		return nil, skerr.Unwrap(err)
	}
	return &pb.ListBenchmarksResponse{
		Benchmarks: benchmarks,
	}, nil
}
