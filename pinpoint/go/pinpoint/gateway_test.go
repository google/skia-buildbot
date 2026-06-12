package pinpoint

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/metadata"

	pb "go.skia.org/infra/pinpoint/proto/v1"
)

func TestGetUserInfo(t *testing.T) {
	testCases := []struct {
		name          string
		ctx           context.Context
		expectedEmail string
	}{
		{
			name: "Gateway Path (HTTP)",
			ctx: metadata.NewIncomingContext(context.Background(), metadata.Pairs(
				"grpcmetadata-x-webauth-user", "user-http@google.com",
			)),
			expectedEmail: "user-http@google.com",
		},
		{
			name: "Direct gRPC Path",
			ctx: metadata.NewIncomingContext(context.Background(), metadata.Pairs(
				"x-webauth-user", "user-grpc@google.com",
			)),
			expectedEmail: "user-grpc@google.com",
		},
		{
			name:          "No Auth Header",
			ctx:           context.Background(),
			expectedEmail: "",
		},
	}

	srv := &gatewayServer{}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := srv.GetUserInfo(tc.ctx, &pb.GetUserInfoRequest{})
			require.NoError(t, err)
			assert.Equal(t, tc.expectedEmail, resp.Email)
		})
	}
}

func TestNewGatewayJSONHandler_GetUserInfo(t *testing.T) {
	ctx := context.Background()
	handler, err := NewGatewayJSONHandler(ctx, nil)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/pinpoint/v1/user", http.NoBody)
	req.Header.Set("X-WEBAUTH-USER", "user@google.com")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), `"email":"user@google.com"`)
}

type mockPinpointClient struct {
	queryJobListFunc          func(ctx context.Context, req *pb.QueryJobListRequest) (*pb.QueryJobListResponse, error)
	createPinpointTryJobFunc  func(ctx context.Context, req *pb.CreateTryJobRequest) (*pb.CreateJobResponse, error)
	listBotConfigurationsFunc func(ctx context.Context) ([]string, error)
	listBenchmarksFunc        func(ctx context.Context) ([]string, error)
	getBenchmarkInfoFunc      func(ctx context.Context, benchmark string) (*BenchmarkInfo, error)
	listRecentBuildsFunc      func(ctx context.Context, configuration string) ([]*pb.BuildInfo, error)
}

func (m *mockPinpointClient) QueryJobList(
	ctx context.Context,
	req *pb.QueryJobListRequest,
) (*pb.QueryJobListResponse, error) {
	if m.queryJobListFunc != nil {
		return m.queryJobListFunc(ctx, req)
	}
	return nil, nil
}

func (m *mockPinpointClient) CreatePinpointTryJob(
	ctx context.Context,
	req *pb.CreateTryJobRequest,
) (*pb.CreateJobResponse, error) {
	if m.createPinpointTryJobFunc != nil {
		return m.createPinpointTryJobFunc(ctx, req)
	}
	return nil, nil
}

func (m *mockPinpointClient) ListBotConfigurations(ctx context.Context) ([]string, error) {
	if m.listBotConfigurationsFunc != nil {
		return m.listBotConfigurationsFunc(ctx)
	}
	return nil, nil
}

func (m *mockPinpointClient) ListBenchmarks(ctx context.Context) ([]string, error) {
	if m.listBenchmarksFunc != nil {
		return m.listBenchmarksFunc(ctx)
	}
	return nil, nil
}

func (m *mockPinpointClient) GetBenchmarkInfo(ctx context.Context, benchmark string) (*BenchmarkInfo, error) {
	if m.getBenchmarkInfoFunc != nil {
		return m.getBenchmarkInfoFunc(ctx, benchmark)
	}
	return nil, nil
}

func (m *mockPinpointClient) ListRecentBuilds(ctx context.Context, configuration string) ([]*pb.BuildInfo, error) {
	if m.listRecentBuildsFunc != nil {
		return m.listRecentBuildsFunc(ctx, configuration)
	}
	return nil, nil
}

func TestCreateTryJob(t *testing.T) {
	validReq := func() *pb.CreateTryJobRequest {
		return &pb.CreateTryJobRequest{
			Benchmark:     "testBenchmark",
			Configuration: "testConfig",
			Story:         "testStory",
			AttemptCount:  30,
			Base: &pb.VariantConfig{
				Commit: "baseCommit",
			},
			Experiment: &pb.VariantConfig{
				Commit: "expCommit",
			},
		}
	}

	t.Run("with user email specified in request", func(t *testing.T) {
		req := validReq()
		req.User = "somebody@google.com"

		client := &mockPinpointClient{
			createPinpointTryJobFunc: func(ctx context.Context, r *pb.CreateTryJobRequest) (*pb.CreateJobResponse, error) {
				assert.Equal(t, "somebody@google.com", r.User)
				return &pb.CreateJobResponse{JobId: "try-job-123"}, nil
			},
		}
		srv := &gatewayServer{client: client}

		resp, err := srv.CreateTryJob(context.Background(), req)
		require.NoError(t, err)
		assert.Equal(t, "try-job-123", resp.JobId)
	})

	t.Run("without user email, gets user email from context", func(t *testing.T) {
		req := validReq()

		client := &mockPinpointClient{
			createPinpointTryJobFunc: func(ctx context.Context, r *pb.CreateTryJobRequest) (*pb.CreateJobResponse, error) {
				assert.Equal(t, "user-http@google.com", r.User)
				return &pb.CreateJobResponse{JobId: "try-job-456"}, nil
			},
		}
		srv := &gatewayServer{client: client}

		// Inject HTTP header in context
		ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(
			"grpcmetadata-x-webauth-user", "user-http@google.com",
		))

		resp, err := srv.CreateTryJob(ctx, req)
		require.NoError(t, err)
		assert.Equal(t, "try-job-456", resp.JobId)
	})

	t.Run("client returns error", func(t *testing.T) {
		client := &mockPinpointClient{
			createPinpointTryJobFunc: func(ctx context.Context, r *pb.CreateTryJobRequest) (*pb.CreateJobResponse, error) {
				return nil, errors.New("legacy endpoint failed")
			},
		}
		srv := &gatewayServer{client: client}

		req := validReq()
		req.User = "somebody@google.com"

		resp, err := srv.CreateTryJob(context.Background(), req)
		require.Error(t, err)
		assert.Nil(t, resp)
		assert.Equal(t, "legacy endpoint failed", err.Error())
	})
}

func TestListBotConfigurations(t *testing.T) {
	t.Run("successful list", func(t *testing.T) {
		expectedBots := []string{"bot1", "bot2"}
		client := &mockPinpointClient{
			listBotConfigurationsFunc: func(ctx context.Context) ([]string, error) {
				return expectedBots, nil
			},
		}
		srv := &gatewayServer{client: client}

		resp, err := srv.ListBotConfigurations(context.Background(), &pb.ListBotConfigurationsRequest{})
		require.NoError(t, err)
		assert.Equal(t, expectedBots, resp.Configurations)
	})

	t.Run("client returns error", func(t *testing.T) {
		client := &mockPinpointClient{
			listBotConfigurationsFunc: func(ctx context.Context) ([]string, error) {
				return nil, errors.New("failed to list bots")
			},
		}
		srv := &gatewayServer{client: client}

		resp, err := srv.ListBotConfigurations(context.Background(), &pb.ListBotConfigurationsRequest{})
		require.Error(t, err)
		assert.Nil(t, resp)
		assert.Contains(t, err.Error(), "failed to list bots")
	})
}

func TestListBenchmarks(t *testing.T) {
	t.Run("successful list", func(t *testing.T) {
		expectedBenchmarks := []string{"bench1", "bench2"}
		client := &mockPinpointClient{
			listBenchmarksFunc: func(ctx context.Context) ([]string, error) {
				return expectedBenchmarks, nil
			},
		}
		srv := &gatewayServer{client: client}

		resp, err := srv.ListBenchmarks(context.Background(), &pb.ListBenchmarksRequest{})
		require.NoError(t, err)
		assert.Equal(t, expectedBenchmarks, resp.Benchmarks)
	})

	t.Run("client returns error", func(t *testing.T) {
		client := &mockPinpointClient{
			listBenchmarksFunc: func(ctx context.Context) ([]string, error) {
				return nil, errors.New("failed to list benchmarks")
			},
		}
		srv := &gatewayServer{client: client}

		resp, err := srv.ListBenchmarks(context.Background(), &pb.ListBenchmarksRequest{})
		require.Error(t, err)
		assert.Nil(t, resp)
		assert.Contains(t, err.Error(), "failed to list benchmarks")
	})
}

func TestGetBenchmarkEndpoint(t *testing.T) {
	t.Run("successful get", func(t *testing.T) {
		expectedStories := []string{"story1", "story2"}
		expectedTags := []string{"tag1", "tag2"}
		client := &mockPinpointClient{
			getBenchmarkInfoFunc: func(ctx context.Context, benchmark string) (*BenchmarkInfo, error) {
				assert.Equal(t, "speedometer3", benchmark)
				return &BenchmarkInfo{
					Benchmark: benchmark,
					Stories:   expectedStories,
					StoryTags: expectedTags,
				}, nil
			},
		}
		srv := &gatewayServer{client: client}

		resp, err := srv.GetBenchmark(context.Background(), &pb.GetBenchmarkInfoRequest{Benchmark: "speedometer3"})
		require.NoError(t, err)
		assert.Equal(t, expectedStories, resp.Stories)
		assert.Equal(t, expectedTags, resp.StoryTags)
		assert.Equal(t, "speedometer3", resp.Benchmark)
	})

	t.Run("client returns error", func(t *testing.T) {
		client := &mockPinpointClient{
			getBenchmarkInfoFunc: func(ctx context.Context, benchmark string) (*BenchmarkInfo, error) {
				return nil, errors.New("failed to get benchmark")
			},
		}
		srv := &gatewayServer{client: client}

		resp, err := srv.GetBenchmark(context.Background(), &pb.GetBenchmarkInfoRequest{Benchmark: "speedometer3"})
		require.Error(t, err)
		assert.Nil(t, resp)
		assert.Contains(t, err.Error(), "failed to get benchmark")
	})
}

func TestListRecentCommits(t *testing.T) {
	t.Run("successful list", func(t *testing.T) {
		expectedCommits := []*pb.BuildInfo{
			{GitHash: "commit1", BuildNumber: 1},
			{GitHash: "commit2", BuildNumber: 2},
		}
		client := &mockPinpointClient{
			listRecentBuildsFunc: func(ctx context.Context, configuration string) ([]*pb.BuildInfo, error) {
				assert.Equal(t, "win-11-perf", configuration)
				return expectedCommits, nil
			},
		}
		srv := &gatewayServer{client: client}

		resp, err := srv.ListRecentBuilds(context.Background(), &pb.ListRecentBuildsRequest{Configuration: "win-11-perf"})
		require.NoError(t, err)
		assert.Equal(t, expectedCommits, resp.Builds)
	})

	t.Run("client returns error", func(t *testing.T) {
		client := &mockPinpointClient{
			listRecentBuildsFunc: func(ctx context.Context, configuration string) ([]*pb.BuildInfo, error) {
				return nil, errors.New("failed to list recent builds")
			},
		}
		srv := &gatewayServer{client: client}

		resp, err := srv.ListRecentBuilds(context.Background(), &pb.ListRecentBuildsRequest{Configuration: "win-11-perf"})
		require.Error(t, err)
		assert.Nil(t, resp)
		assert.Contains(t, err.Error(), "failed to list recent builds")
	})
}
