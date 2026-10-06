package pinpoint

import (
	"context"
	"net/http"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"

	"go.skia.org/infra/go/alogin"
	"go.skia.org/infra/go/alogin/proxylogin"
	"go.skia.org/infra/go/skerr"
	pb "go.skia.org/infra/pinpoint/proto/v1"
)

type pinpointClient interface {
	QueryJobList(ctx context.Context, req *pb.QueryJobListRequest) (*pb.QueryJobListResponse, error)
	CreatePinpointTryJob(ctx context.Context, req *pb.CreateTryJobRequest) (*pb.CreateJobResponse, error)
	ListBotConfigurations(ctx context.Context) ([]string, error)
	ListBenchmarks(ctx context.Context) ([]string, error)
	GetBenchmarkInfo(ctx context.Context, benchmark string) (*BenchmarkInfo, error)
	ListRecentBuilds(ctx context.Context, configuration string) ([]*pb.BuildInfo, error)
	GetCommit(ctx context.Context, commit string) (*pb.GetCommitResponse, error)
	GetPatch(ctx context.Context, req *pb.GetPatchRequest) (*pb.GetPatchResponse, error)
	CancelJob(ctx context.Context, req *pb.CancelPinpointJobRequest) (*pb.CancelPinpointJobResponse, error)
}

type gatewayServer struct {
	pb.UnimplementedPinpointGatewayServer
	client pinpointClient
}

// NewGatewayJSONHandler registers the http handlers for the PinpointGateway
// service and returns the handler.
func NewGatewayJSONHandler(ctx context.Context, client *Client) (http.Handler, error) {
	return newGatewayJSONHandler(ctx, client, proxylogin.NewWithDefaults())
}

func newGatewayJSONHandler(ctx context.Context, client pinpointClient, login alogin.Login) (http.Handler, error) {
	srv := &gatewayServer{
		client: client,
	}
	m := runtime.NewServeMux()
	if err := pb.RegisterPinpointGatewayHandlerServer(ctx, m, srv); err != nil {
		return nil, skerr.Wrapf(err, "unable to register pinpoint gateway handler")
	}
	return alogin.StatusMiddleware(login)(m), nil
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

func (s *gatewayServer) GetUserInfo(
	ctx context.Context,
	req *pb.GetUserInfoRequest,
) (*pb.GetUserInfoResponse, error) {
	return &pb.GetUserInfoResponse{
		Email: alogin.GetStatus(ctx).EMail.String(),
	}, nil
}

func (s *gatewayServer) CreateTryJob(
	ctx context.Context,
	req *pb.CreateTryJobRequest,
) (*pb.CreateJobResponse, error) {
	req.User = alogin.GetStatus(ctx).EMail.String()

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

func (s *gatewayServer) GetBenchmark(
	ctx context.Context,
	req *pb.GetBenchmarkInfoRequest,
) (*pb.GetBenchmarkInfoResponse, error) {
	info, err := s.client.GetBenchmarkInfo(ctx, req.Benchmark)
	if err != nil {
		// Unwrap the error because it may be displayed to the user.
		return nil, skerr.Unwrap(err)
	}
	return &pb.GetBenchmarkInfoResponse{
		Benchmark: info.Benchmark,
		Stories:   info.Stories,
		StoryTags: info.StoryTags,
	}, nil
}

func (s *gatewayServer) ListRecentBuilds(
	ctx context.Context,
	req *pb.ListRecentBuildsRequest,
) (*pb.ListRecentBuildsResponse, error) {
	builds, err := s.client.ListRecentBuilds(ctx, req.Configuration)
	if err != nil {
		// Unwrap the error because it may be displayed to the user.
		return nil, skerr.Unwrap(err)
	}
	return &pb.ListRecentBuildsResponse{
		Builds: builds,
	}, nil
}

func (s *gatewayServer) GetCommit(
	ctx context.Context,
	req *pb.GetCommitRequest,
) (*pb.GetCommitResponse, error) {
	resp, err := s.client.GetCommit(ctx, req.Commit)
	if err != nil {
		return nil, skerr.Unwrap(err)
	}
	return resp, nil
}

func (s *gatewayServer) GetPatch(
	ctx context.Context,
	req *pb.GetPatchRequest,
) (*pb.GetPatchResponse, error) {
	resp, err := s.client.GetPatch(ctx, req)
	if err != nil {
		return nil, skerr.Unwrap(err)
	}
	return resp, nil
}

func (s *gatewayServer) CancelJob(
	ctx context.Context,
	req *pb.CancelPinpointJobRequest,
) (*pb.CancelPinpointJobResponse, error) {
	resp, err := s.client.CancelJob(ctx, req)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return resp, nil
}
