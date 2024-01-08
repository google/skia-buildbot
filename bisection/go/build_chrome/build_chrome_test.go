package build_chrome

import (
	"context"
	"fmt"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	buildbucketpb "go.chromium.org/luci/buildbucket/proto"
	swarmingV1 "go.chromium.org/luci/common/api/swarming/swarming/v1"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type mockBuildsClient struct {
	shouldErr bool
}

// casOutput is the default the build output
// properties mocked return. The structure is
// rather complex so it's defined here to avoid
// obscuring what the tests do
var casOutput = buildbucketpb.Build_Output{
	Properties: &structpb.Struct{
		Fields: map[string]*structpb.Value{
			"fake_field": {},
			"swarm_hashes_refs": {
				Kind: &structpb.Value_StructValue{
					StructValue: &structpb.Struct{
						Fields: map[string]*structpb.Value{
							"target": {
								Kind: &structpb.Value_StringValue{
									StringValue: "hash/123",
								},
							},
						},
					},
				},
			},
		},
	},
}

func TestSearchBuild(t *testing.T) {
	for i, test := range []struct {
		name          string
		bc            *BuildChrome
		builder       string
		mockResp      *buildbucketpb.SearchBuildsResponse
		expected      int64
		expectedError bool
	}{
		{
			name: "buildbucket search error",
			bc: &BuildChrome{
				Commit: "commit",
			},
			builder:       "builder",
			expected:      0,
			expectedError: true,
		},
		{
			name:    "build found",
			builder: "builder",
			bc: &BuildChrome{
				Commit: "commit",
			},
			mockResp: &buildbucketpb.SearchBuildsResponse{
				Builds: []*buildbucketpb.Build{
					{
						Id:      1,
						Status:  buildbucketpb.Status_SUCCESS,
						EndTime: timestamppb.Now(),
						Input: &buildbucketpb.Build_Input{
							GerritChanges: []*buildbucketpb.GerritChange{},
						},
					},
				},
			},
			expected:      1,
			expectedError: false,
		},
		{
			name:    "build found but failure",
			builder: "builder",
			bc: &BuildChrome{
				Commit: "commit",
			},
			mockResp: &buildbucketpb.SearchBuildsResponse{
				Builds: []*buildbucketpb.Build{
					{
						Id:      1,
						Status:  buildbucketpb.Status_FAILURE,
						EndTime: timestamppb.Now(),
						Input: &buildbucketpb.Build_Input{
							GerritChanges: []*buildbucketpb.GerritChange{},
						},
					},
				},
			},
			expected:      0,
			expectedError: false,
		},
		{
			name:    "build not found",
			builder: "builder",
			bc: &BuildChrome{
				Commit: "commit",
			},
			expected:      0,
			expectedError: false,
		},
	} {
		t.Run(fmt.Sprintf("[%d] %s", i, test.name), func(t *testing.T) {
			ctx := context.Background()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			client := buildbucketpb.NewMockBuildsClient(ctrl)

			if test.expectedError {
				client.EXPECT().SearchBuilds(ctx, gomock.Any()).Return(
					nil, fmt.Errorf("some error"))
			} else {
				client.EXPECT().SearchBuilds(ctx, gomock.Any()).Return(
					test.mockResp, nil)
			}

			test.bc.Client = client

			id, err := test.bc.searchBuild(ctx, test.builder)
			if test.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, test.expected, id)
			}
		})
	}
}

func TestCheckBuildStatus(t *testing.T) {
	for i, test := range []struct {
		name          string
		mockResp      *buildbucketpb.Build
		expected      buildbucketpb.Status
		expectedError bool
	}{
		{
			name: "build success",
			mockResp: &buildbucketpb.Build{
				Status: buildbucketpb.Status_SUCCESS,
			},
			expected:      buildbucketpb.Status_SUCCESS,
			expectedError: false,
		},
		{
			name: "build failed",
			mockResp: &buildbucketpb.Build{
				Status: buildbucketpb.Status_FAILURE,
			},
			expected:      buildbucketpb.Status_FAILURE,
			expectedError: false,
		},
		{
			name:          "GetBuildStatus error",
			expected:      buildbucketpb.Status_STATUS_UNSPECIFIED,
			expectedError: true,
		},
	} {
		t.Run(fmt.Sprintf("[%d] %s", i, test.name), func(t *testing.T) {
			ctx := context.Background()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			client := buildbucketpb.NewMockBuildsClient(ctrl)

			if test.expectedError {
				client.EXPECT().GetBuildStatus(ctx, gomock.Any()).Return(
					test.mockResp, fmt.Errorf("some error"))
			} else {
				client.EXPECT().GetBuildStatus(ctx, gomock.Any()).Return(
					test.mockResp, nil)
			}

			bc := &BuildChrome{
				Client: client,
			}

			status, err := bc.CheckBuildStatus(ctx, 0)
			if test.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, test.expected, status)
		})
	}
}

func TestBuildNonExistentDevice(t *testing.T) {
	ctx := context.Background()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	client := buildbucketpb.NewMockBuildsClient(ctrl)

	bc := BuildChrome{
		Client: client,
		Device: "non-existent device",
		Target: "target",
		Commit: "commit",
	}

	id, err := bc.Run(ctx)
	assert.Error(t, err)
	assert.Zero(t, id)
}

func TestBuildFound(t *testing.T) {
	mockResp := &buildbucketpb.SearchBuildsResponse{
		Builds: []*buildbucketpb.Build{
			{
				Id:      1,
				Status:  buildbucketpb.Status_SUCCESS,
				EndTime: timestamppb.Now(),
				Input: &buildbucketpb.Build_Input{
					GerritChanges: []*buildbucketpb.GerritChange{},
				},
			},
		},
	}
	expected := int64(1)

	ctx := context.Background()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	client := buildbucketpb.NewMockBuildsClient(ctrl)

	client.EXPECT().SearchBuilds(gomock.Any(), gomock.Any()).Return(
		mockResp, nil)

	bc := BuildChrome{
		Client: client,
		Device: "linux-perf",
		Target: "target",
		Commit: "commit",
	}

	id, err := bc.Run(ctx)
	assert.NoError(t, err)
	assert.Equal(t, expected, id)
}

func TestNewBuild(t *testing.T) {
	for i, test := range []struct {
		name          string
		mockResp      *buildbucketpb.Build
		expected      int64
		expectedError bool
	}{
		{
			name: "build success",
			mockResp: &buildbucketpb.Build{
				Id: 1,
			},
			expected:      1,
			expectedError: false,
		},
		{
			name:          "build failed",
			expected:      0,
			expectedError: true,
		},
	} {
		t.Run(fmt.Sprintf("[%d] %s", i, test.name), func(t *testing.T) {
			ctx := context.Background()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			client := buildbucketpb.NewMockBuildsClient(ctrl)

			client.EXPECT().SearchBuilds(ctx, gomock.Any()).Return(
				nil, nil)
			client.EXPECT().SearchBuilds(gomock.Any(), gomock.Any()).Return(
				nil, nil)
			if test.expectedError {
				client.EXPECT().ScheduleBuild(ctx, gomock.Any()).Return(
					nil, fmt.Errorf("some error"))
			} else {
				client.EXPECT().ScheduleBuild(ctx, gomock.Any()).Return(
					test.mockResp, nil)
			}

			bc := BuildChrome{
				Client: client,
				Device: "linux-perf",
				Target: "target",
				Commit: "commit",
			}

			id, err := bc.Run(ctx)
			if test.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, test.expected, id)
		})
	}
}

func TestRetrieveCas(t *testing.T) {
	for i, test := range []struct {
		name          string
		mockResp      *buildbucketpb.Build
		expected      *swarmingV1.SwarmingRpcsDigest
		expectedError bool
	}{
		{
			name: "build failed",
			mockResp: &buildbucketpb.Build{
				Status: buildbucketpb.Status_FAILURE,
			},
			expected:      nil,
			expectedError: true,
		},
		{
			name: "build ongoing",
			mockResp: &buildbucketpb.Build{
				Status: buildbucketpb.Status_STARTED,
			},
			expected:      nil,
			expectedError: true,
		},
		{
			name: "retrieve cas success",
			mockResp: &buildbucketpb.Build{
				Status: buildbucketpb.Status_SUCCESS,
				Output: &casOutput,
			},
			expected: &swarmingV1.SwarmingRpcsDigest{
				Hash:      "hash",
				SizeBytes: 123,
			},
			expectedError: false,
		},
	} {
		t.Run(fmt.Sprintf("[%d] %s", i, test.name), func(t *testing.T) {
			ctx := context.Background()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			client := buildbucketpb.NewMockBuildsClient(ctrl)

			client.EXPECT().GetBuild(ctx, gomock.Any()).Return(
				test.mockResp, nil)

			bc := BuildChrome{
				Client: client,
				Target: "target",
			}

			cas, err := bc.RetrieveCas(ctx, 0)
			if test.expectedError {
				assert.Error(t, err)
				assert.Nil(t, cas)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, test.expected.Hash, cas.Digest.Hash)
				assert.Equal(t, test.expected.SizeBytes, cas.Digest.SizeBytes)
			}
		})
	}
}
