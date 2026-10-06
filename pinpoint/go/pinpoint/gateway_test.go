package pinpoint

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/timestamppb"

	"go.skia.org/infra/go/alogin"
	"go.skia.org/infra/go/alogin/proxylogin"
	pb "go.skia.org/infra/pinpoint/proto/v1"
)

func TestGetUserInfo(t *testing.T) {
	testCases := []struct {
		name          string
		ctx           context.Context
		expectedEmail string
	}{
		{
			name: "Authenticated User via alogin.Status",
			ctx: alogin.FakeStatus(context.Background(), &alogin.Status{
				EMail: "user-http@google.com",
			}),
			expectedEmail: "user-http@google.com",
		},
		{
			name:          "No Auth Status",
			ctx:           context.Background(),
			expectedEmail: "",
		},
		{
			name: "Ignores Raw gRPC Metadata Keys",
			ctx: metadata.NewIncomingContext(context.Background(), metadata.Pairs(
				"grpcmetadata-x-webauth-user", "spoof-http@google.com",
				"x-webauth-user", "spoof-grpc@google.com",
			)),
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

func TestGatewayIdentity_IgnoresClientMetadataHeaders(t *testing.T) {
	const (
		attackerEmail = "attacker@google.com"
		victimEmail   = "victim@google.com"
		iterations    = 50
	)

	testCases := []struct {
		name         string
		applyHeaders func(h http.Header)
	}{
		{
			name: "Grpc-Metadata-Grpcmetadata-X-Webauth-User",
			applyHeaders: func(h http.Header) {
				h.Set("X-WEBAUTH-USER", attackerEmail)
				h.Set("Grpc-Metadata-Grpcmetadata-X-Webauth-User", victimEmail)
			},
		},
		{
			name: "Grpc-Metadata-X-Webauth-User",
			applyHeaders: func(h http.Header) {
				h.Set("X-WEBAUTH-USER", attackerEmail)
				h.Set("Grpc-Metadata-X-Webauth-User", victimEmail)
			},
		},
		{
			name: "lowercase grpc-metadata-grpcmetadata-x-webauth-user",
			applyHeaders: func(h http.Header) {
				h.Set("X-WEBAUTH-USER", attackerEmail)
				h.Set("grpc-metadata-grpcmetadata-x-webauth-user", victimEmail)
				h["grpc-metadata-grpcmetadata-x-webauth-user"] = []string{victimEmail}
			},
		},
		{
			name: "lowercase grpc-metadata-x-webauth-user",
			applyHeaders: func(h http.Header) {
				h.Set("X-WEBAUTH-USER", attackerEmail)
				h.Set("grpc-metadata-x-webauth-user", victimEmail)
				h["grpc-metadata-x-webauth-user"] = []string{victimEmail}
			},
		},
		{
			name: "duplicated X-WEBAUTH-USER headers",
			applyHeaders: func(h http.Header) {
				h.Set("X-WEBAUTH-USER", attackerEmail)
				h.Add("X-WEBAUTH-USER", victimEmail)
			},
		},
	}

	tryJobPayload, err := json.Marshal(&pb.CreateTryJobRequest{
		Benchmark:     "speedometer3",
		Configuration: "linux-perf",
		Story:         "Story1",
		AttemptCount:  10,
		Base:          &pb.VariantConfig{Commit: "base-sha"},
		Experiment:    &pb.VariantConfig{Commit: "exp-sha"},
		User:          victimEmail,
	})
	require.NoError(t, err)

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			for i := 0; i < iterations; i++ {
				var capturedUser string
				client := &mockPinpointClient{
					createPinpointTryJobFunc: func(ctx context.Context, r *pb.CreateTryJobRequest) (*pb.CreateJobResponse, error) {
						capturedUser = r.User
						return &pb.CreateJobResponse{JobId: "job-1"}, nil
					},
				}

				handler, err := newGatewayJSONHandler(context.Background(), client, proxylogin.NewWithDefaults())
				require.NoError(t, err)

				userReq := httptest.NewRequest(http.MethodGet, "/pinpoint/v1/user", http.NoBody)
				tc.applyHeaders(userReq.Header)
				userRec := httptest.NewRecorder()
				handler.ServeHTTP(userRec, userReq)

				require.Equal(t, http.StatusOK, userRec.Code)
				var userResp pb.GetUserInfoResponse
				require.NoError(t, json.Unmarshal(userRec.Body.Bytes(), &userResp))
				require.Equal(t, attackerEmail, userResp.Email, "iteration %d: GET /pinpoint/v1/user", i)

				newReq := httptest.NewRequest(http.MethodPost, "/pinpoint/v1/new", bytes.NewReader(tryJobPayload))
				newReq.Header.Set("Content-Type", "application/json")
				tc.applyHeaders(newReq.Header)
				newRec := httptest.NewRecorder()
				handler.ServeHTTP(newRec, newReq)

				require.Equal(t, http.StatusOK, newRec.Code)
				require.Equal(t, attackerEmail, capturedUser, "iteration %d: POST /pinpoint/v1/new", i)
			}
		})
	}
}

type mockPinpointClient struct {
	queryJobListFunc          func(ctx context.Context, req *pb.QueryJobListRequest) (*pb.QueryJobListResponse, error)
	createPinpointTryJobFunc  func(ctx context.Context, req *pb.CreateTryJobRequest) (*pb.CreateJobResponse, error)
	listBotConfigurationsFunc func(ctx context.Context) ([]string, error)
	listBenchmarksFunc        func(ctx context.Context) ([]string, error)
	getBenchmarkInfoFunc      func(ctx context.Context, benchmark string) (*BenchmarkInfo, error)
	listRecentBuildsFunc      func(ctx context.Context, configuration string) ([]*pb.BuildInfo, error)
	getCommitFunc             func(ctx context.Context, commit string) (*pb.GetCommitResponse, error)
	getPatchFunc              func(ctx context.Context, req *pb.GetPatchRequest) (*pb.GetPatchResponse, error)
	cancelJobFunc             func(ctx context.Context, req *pb.CancelPinpointJobRequest) (*pb.CancelPinpointJobResponse, error)
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

func (m *mockPinpointClient) GetBenchmarkInfo(
	ctx context.Context,
	benchmark string,
) (*BenchmarkInfo, error) {
	if m.getBenchmarkInfoFunc != nil {
		return m.getBenchmarkInfoFunc(ctx, benchmark)
	}
	return nil, nil
}

func (m *mockPinpointClient) ListRecentBuilds(
	ctx context.Context,
	configuration string,
) ([]*pb.BuildInfo, error) {
	if m.listRecentBuildsFunc != nil {
		return m.listRecentBuildsFunc(ctx, configuration)
	}
	return nil, nil
}

func (m *mockPinpointClient) GetCommit(
	ctx context.Context,
	commit string,
) (*pb.GetCommitResponse, error) {
	if m.getCommitFunc != nil {
		return m.getCommitFunc(ctx, commit)
	}
	return nil, nil
}

func (m *mockPinpointClient) GetPatch(
	ctx context.Context,
	req *pb.GetPatchRequest,
) (*pb.GetPatchResponse, error) {
	if m.getPatchFunc != nil {
		return m.getPatchFunc(ctx, req)
	}
	return nil, nil
}

func (m *mockPinpointClient) CancelJob(
	ctx context.Context,
	req *pb.CancelPinpointJobRequest,
) (*pb.CancelPinpointJobResponse, error) {
	if m.cancelJobFunc != nil {
		return m.cancelJobFunc(ctx, req)
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

	t.Run("overwrites user email specified in request with context email", func(t *testing.T) {
		req := validReq()
		req.User = "somebody@google.com"

		client := &mockPinpointClient{
			createPinpointTryJobFunc: func(ctx context.Context, r *pb.CreateTryJobRequest) (*pb.CreateJobResponse, error) {
				assert.Equal(t, "user-http@google.com", r.User)
				return &pb.CreateJobResponse{JobId: "try-job-123"}, nil
			},
		}
		srv := &gatewayServer{client: client}

		ctx := alogin.FakeStatus(context.Background(), &alogin.Status{
			EMail: "user-http@google.com",
		})

		resp, err := srv.CreateTryJob(ctx, req)
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

		ctx := alogin.FakeStatus(context.Background(), &alogin.Status{
			EMail: "user-http@google.com",
		})

		resp, err := srv.CreateTryJob(ctx, req)
		require.NoError(t, err)
		assert.Equal(t, "try-job-456", resp.JobId)
	})

	t.Run("without user email, ignores raw gRPC metadata keys", func(t *testing.T) {
		req := validReq()

		client := &mockPinpointClient{
			createPinpointTryJobFunc: func(ctx context.Context, r *pb.CreateTryJobRequest) (*pb.CreateJobResponse, error) {
				assert.Empty(t, r.User)
				return &pb.CreateJobResponse{JobId: "try-job-789"}, nil
			},
		}
		srv := &gatewayServer{client: client}

		ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(
			"grpcmetadata-x-webauth-user", "spoof-http@google.com",
			"x-webauth-user", "spoof-grpc@google.com",
		))

		resp, err := srv.CreateTryJob(ctx, req)
		require.NoError(t, err)
		assert.Equal(t, "try-job-789", resp.JobId)
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

		resp, err := srv.ListBotConfigurations(
			context.Background(),
			&pb.ListBotConfigurationsRequest{},
		)
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

		resp, err := srv.ListBotConfigurations(
			context.Background(),
			&pb.ListBotConfigurationsRequest{},
		)
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

		resp, err := srv.GetBenchmark(
			context.Background(),
			&pb.GetBenchmarkInfoRequest{Benchmark: "speedometer3"},
		)
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

		resp, err := srv.GetBenchmark(
			context.Background(),
			&pb.GetBenchmarkInfoRequest{Benchmark: "speedometer3"},
		)
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

		resp, err := srv.ListRecentBuilds(
			context.Background(),
			&pb.ListRecentBuildsRequest{Configuration: "win-11-perf"},
		)
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

		resp, err := srv.ListRecentBuilds(
			context.Background(),
			&pb.ListRecentBuildsRequest{Configuration: "win-11-perf"},
		)
		require.Error(t, err)
		assert.Nil(t, resp)
		assert.Contains(t, err.Error(), "failed to list recent builds")
	})
}

func TestGetCommitGateway(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		expectedResp := &pb.GetCommitResponse{
			Repository:     "chromium",
			GitHash:        "91a446671c3d1518f6d8cb5349e8155ec3a4f3d8",
			Url:            "https://chromium.googlesource.com/chromium/src/+/91a446671c3d1518f6d8cb5349e8155ec3a4f3d8",
			Author:         "neis@chromium.org",
			Created:        "2026-06-12T12:58:40",
			Subject:        "ash: Remove BestResultCategoryRanker and FtrlRanker category ranking",
			Message:        "ash: Remove BestResultCategoryRanker and FtrlRanker category ranking\n\n...",
			CommitBranch:   "refs/heads/main",
			CommitPosition: 1645883,
			ReviewUrl:      "https://chromium-review.googlesource.com/c/chromium/src/+/7930566",
			ChangeId:       "I27a4ae239808ad400480ebfda8a2016e634b3aa5",
		}
		client := &mockPinpointClient{
			getCommitFunc: func(ctx context.Context, commit string) (*pb.GetCommitResponse, error) {
				assert.Equal(t, "91a446671c3d1518f6d8cb5349e8155ec3a4f3d8", commit)
				return expectedResp, nil
			},
		}
		srv := &gatewayServer{client: client}

		resp, err := srv.GetCommit(context.Background(), &pb.GetCommitRequest{
			Commit: "91a446671c3d1518f6d8cb5349e8155ec3a4f3d8",
		})
		require.NoError(t, err)
		assert.Equal(t, expectedResp, resp)
	})

	t.Run("client returns error", func(t *testing.T) {
		client := &mockPinpointClient{
			getCommitFunc: func(ctx context.Context, commit string) (*pb.GetCommitResponse, error) {
				return nil, errors.New("failed to get commit info")
			},
		}
		srv := &gatewayServer{client: client}

		resp, err := srv.GetCommit(context.Background(), &pb.GetCommitRequest{
			Commit: "91a446671c3d1518f6d8cb5349e8155ec3a4f3d8",
		})
		require.Error(t, err)
		assert.Nil(t, resp)
		assert.Contains(t, err.Error(), "failed to get commit info")
	})
}

func TestGetPatchGateway(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		expectedResp := &pb.GetPatchResponse{
			Host:     "https://chromium-review.googlesource.com",
			Change:   5582697,
			Patchset: 3,
			Project:  "chromium/src",
			Author:   "author@chromium.org",
			Subject:  "Commit message",
			Created:  timestamppb.New(time.Date(2026, 6, 12, 12, 58, 40, 0, time.UTC)),
		}
		client := &mockPinpointClient{
			getPatchFunc: func(ctx context.Context, req *pb.GetPatchRequest) (*pb.GetPatchResponse, error) {
				assert.Equal(t, "https://chromium-review.googlesource.com", req.Host)
				assert.Equal(t, int64(5582697), req.Change)
				assert.Equal(t, int64(3), *req.Patchset)
				return expectedResp, nil
			},
		}
		srv := &gatewayServer{client: client}

		patchset := int64(3)
		resp, err := srv.GetPatch(context.Background(), &pb.GetPatchRequest{
			Host:     "https://chromium-review.googlesource.com",
			Change:   5582697,
			Patchset: &patchset,
		})
		require.NoError(t, err)
		assert.Equal(t, expectedResp, resp)
	})

	t.Run("client returns error", func(t *testing.T) {
		client := &mockPinpointClient{
			getPatchFunc: func(ctx context.Context, req *pb.GetPatchRequest) (*pb.GetPatchResponse, error) {
				return nil, errors.New("failed to get patch info")
			},
		}
		srv := &gatewayServer{client: client}

		resp, err := srv.GetPatch(context.Background(), &pb.GetPatchRequest{
			Host:   "https://chromium-review.googlesource.com",
			Change: 5582697,
		})
		require.Error(t, err)
		assert.Nil(t, resp)
		assert.Contains(t, err.Error(), "failed to get patch info")
	})
}

func TestCancelJob(t *testing.T) {
	t.Run("successful cancel", func(t *testing.T) {
		client := &mockPinpointClient{
			cancelJobFunc: func(ctx context.Context, req *pb.CancelPinpointJobRequest) (*pb.CancelPinpointJobResponse, error) {
				assert.Equal(t, "job-123", req.JobId)
				return &pb.CancelPinpointJobResponse{}, nil
			},
		}
		srv := &gatewayServer{client: client}

		resp, err := srv.CancelJob(context.Background(), &pb.CancelPinpointJobRequest{
			JobId: "job-123",
		})
		require.NoError(t, err)
		assert.NotNil(t, resp)
	})

	t.Run("client returns error", func(t *testing.T) {
		client := &mockPinpointClient{
			cancelJobFunc: func(ctx context.Context, req *pb.CancelPinpointJobRequest) (*pb.CancelPinpointJobResponse, error) {
				return nil, errors.New("failed to cancel job")
			},
		}
		srv := &gatewayServer{client: client}

		resp, err := srv.CancelJob(context.Background(), &pb.CancelPinpointJobRequest{
			JobId: "job-123",
		})
		require.Error(t, err)
		assert.Nil(t, resp)
		assert.Contains(t, err.Error(), "failed to cancel job")
	})
}
