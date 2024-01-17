package build_chrome

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	buildbucketpb "go.chromium.org/luci/buildbucket/proto"
	swarmingV1 "go.chromium.org/luci/common/api/swarming/swarming/v1"

	"google.golang.org/protobuf/types/known/timestamppb"

	"go.skia.org/infra/bisection/go/backends"
	"go.skia.org/infra/bisection/go/backends/mocks"
	"go.skia.org/infra/go/testutils"
)

func TestSearchBuild(t *testing.T) {
	for i, test := range []struct {
		name              string
		builder           string
		mockResp          interface{}
		expected          int64
		expectedErrorDeps bool
		expectedErrorCI   bool
	}{
		{
			name:              "buildbucket search error",
			builder:           "builder",
			expected:          0,
			expectedErrorDeps: true,
			expectedErrorCI:   false,
		},
		{
			name:    "build found",
			builder: "builder",
			mockResp: &buildbucketpb.Build{
				Id:      1,
				Status:  buildbucketpb.Status_SUCCESS,
				EndTime: timestamppb.Now(),
				Input: &buildbucketpb.Build_Input{
					GerritChanges: []*buildbucketpb.GerritChange{},
				},
			},
			expected:          1,
			expectedErrorDeps: false,
			expectedErrorCI:   false,
		},
		{
			name:    "build found through CI counterpart",
			builder: "builder",
			mockResp: &buildbucketpb.Build{
				Id:      1,
				Status:  buildbucketpb.Status_FAILURE,
				EndTime: timestamppb.Now(),
				Input: &buildbucketpb.Build_Input{
					GerritChanges: []*buildbucketpb.GerritChange{},
				},
			},
			expected:          1,
			expectedErrorDeps: true,
			expectedErrorCI:   false,
		},
		{
			name:              "build not found",
			builder:           "builder",
			expected:          0,
			expectedErrorDeps: false,
			expectedErrorCI:   false,
		},
	} {
		t.Run(fmt.Sprintf("[%d] %s", i, test.name), func(t *testing.T) {
			ctx := context.Background()

			mb := &mocks.Buildbucket{}
			bc := &BuildChrome{
				Builder: test.builder,
				Client:  mb,
				Commit:  "commit",
			}

			if test.expectedErrorDeps {
				mb.On("GetBuildWithPatches", testutils.AnyContext, bc.Builder, backends.DefaultBucket, bc.Commit, bc.Patch).Return(nil, fmt.Errorf("random error"))
			} else {
				mb.On("GetBuildWithPatches", testutils.AnyContext, bc.Builder, backends.DefaultBucket, bc.Commit, bc.Patch).Return(test.mockResp, nil)
			}

			if test.expectedErrorCI {
				mb.On("GetBuildFromWaterfall", testutils.AnyContext, bc.Builder, bc.Commit).Return(nil, fmt.Errorf("random error"))
			} else {
				mb.On("GetBuildFromWaterfall", testutils.AnyContext, bc.Builder, bc.Commit).Return(test.mockResp, nil)
			}

			id, err := bc.searchBuild(ctx, test.builder)
			if (test.expectedErrorDeps && !test.expectedErrorCI) || (test.expectedErrorDeps && test.expectedErrorCI) {
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
		mockResp      buildbucketpb.Status
		expected      buildbucketpb.Status
		expectedError bool
	}{
		{
			name:          "build success",
			mockResp:      buildbucketpb.Status_SUCCESS,
			expected:      buildbucketpb.Status_SUCCESS,
			expectedError: false,
		},
		{
			name:          "build failed",
			mockResp:      buildbucketpb.Status_FAILURE,
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
			buildID := int64(0)

			mb := &mocks.Buildbucket{}
			bc := &BuildChrome{
				Client: mb,
			}

			if test.expectedError {
				mb.On("GetBuildStatus", testutils.AnyContext, buildID).Return(buildbucketpb.Status_STATUS_UNSPECIFIED, fmt.Errorf("some error"))
			} else {
				mb.On("GetBuildStatus", testutils.AnyContext, buildID).Return(test.mockResp, nil)
			}

			status, err := bc.CheckBuildStatus(ctx, buildID)
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

	mb := &mocks.Buildbucket{}
	bc := BuildChrome{
		Client: mb,
		Device: "non-existent device",
		Target: "target",
		Commit: "commit",
	}

	jID := "1"
	id, err := bc.Run(ctx, jID)
	assert.Error(t, err)
	assert.Zero(t, id)
}

func TestBuildFound(t *testing.T) {
	mockResp := &buildbucketpb.Build{
		Id:      1,
		Status:  buildbucketpb.Status_SUCCESS,
		EndTime: timestamppb.Now(),
		Input: &buildbucketpb.Build_Input{
			GerritChanges: []*buildbucketpb.GerritChange{},
		},
	}
	expected := int64(1)

	ctx := context.Background()
	mb := &mocks.Buildbucket{}
	bc := BuildChrome{
		Client: mb,
		Device: "linux-perf",
		Target: "target",
		Commit: "commit",
	}

	mb.On("GetBuildWithPatches", testutils.AnyContext, "Linux Builder Perf", backends.DefaultBucket, bc.Commit, bc.Patch).Return(mockResp, nil)

	jID := "1"
	id, err := bc.Run(ctx, jID)
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
			mb := &mocks.Buildbucket{}
			bc := BuildChrome{
				Client: mb,
				Device: "linux-perf",
				Target: "target",
				Commit: "commit",
			}

			builder := "Linux Builder Perf"

			mb.On("GetBuildWithPatches", testutils.AnyContext, builder, backends.DefaultBucket, bc.Commit, bc.Patch).Return(nil, nil)
			mb.On("GetBuildFromWaterfall", testutils.AnyContext, builder, bc.Commit).Return(nil, nil)

			if test.expectedError {
				mb.On("StartChromeBuild", testutils.AnyContext, mock.Anything, mock.Anything, builder, bc.Commit, bc.Patch).Return(nil, fmt.Errorf("some error"))
			} else {
				mb.On("StartChromeBuild", testutils.AnyContext, mock.Anything, mock.Anything, builder, bc.Commit, bc.Patch).Return(test.mockResp, nil)
			}

			jID := "1"
			id, err := bc.Run(ctx, jID)
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
		mockResp      *swarmingV1.SwarmingRpcsCASReference
		expected      *swarmingV1.SwarmingRpcsDigest
		expectedError bool
	}{
		{
			name:          "build failed or ongoing",
			expected:      nil,
			expectedError: true,
		},
		{
			name: "retrieve cas success",
			mockResp: &swarmingV1.SwarmingRpcsCASReference{
				CasInstance: backends.DefaultCASInstance,
				Digest: &swarmingV1.SwarmingRpcsDigest{
					Hash:      "hash",
					SizeBytes: 123,
				},
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
			mb := &mocks.Buildbucket{}
			bc := BuildChrome{
				Client: mb,
				Target: "target",
			}
			buildID := int64(0)
			if test.expectedError {
				mb.On("GetCASReference", testutils.AnyContext, buildID, bc.Target).Return(nil, fmt.Errorf("some error"))
			} else {
				mb.On("GetCASReference", testutils.AnyContext, buildID, bc.Target).Return(test.mockResp, nil)
			}

			cas, err := bc.RetrieveCas(ctx, buildID)
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
