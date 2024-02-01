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

	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/pinpoint/go/backends"
	"go.skia.org/infra/pinpoint/go/backends/mocks"
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

			mb := &mocks.BuildbucketClient{}
			fakeCommit := "fake-commit"
			var patches []*buildbucketpb.GerritChange = nil
			deps := map[string]interface{}{}
			bc := &buildChromeImpl{
				client: mb,
			}

			if test.expectedErrorDeps {
				mb.On("GetSingleBuild", testutils.AnyContext, test.builder, backends.DefaultBucket, fakeCommit, deps, patches).Return(nil, fmt.Errorf("random error"))
			} else {
				mb.On("GetSingleBuild", testutils.AnyContext, test.builder, backends.DefaultBucket, fakeCommit, deps, patches).Return(test.mockResp, nil)
			}

			if test.expectedErrorCI {
				mb.On("GetBuildFromWaterfall", testutils.AnyContext, test.builder, fakeCommit).Return(nil, fmt.Errorf("random error"))
			} else {
				mb.On("GetBuildFromWaterfall", testutils.AnyContext, test.builder, fakeCommit).Return(test.mockResp, nil)
			}

			id, err := bc.searchBuild(ctx, test.builder, fakeCommit, deps, patches)
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

			mb := &mocks.BuildbucketClient{}
			bc := &buildChromeImpl{
				client: mb,
			}

			if test.expectedError {
				mb.On("GetBuildStatus", testutils.AnyContext, buildID).Return(buildbucketpb.Status_STATUS_UNSPECIFIED, fmt.Errorf("some error"))
			} else {
				mb.On("GetBuildStatus", testutils.AnyContext, buildID).Return(test.mockResp, nil)
			}

			status, err := bc.GetStatus(ctx, buildID)
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

	mb := &mocks.BuildbucketClient{}
	bc := buildChromeImpl{
		client: mb,
	}

	id, err := bc.SearchOrBuild(ctx, "fake-jID", "fake-commit", "non-existent device", "fake-target", nil, nil)
	assert.ErrorContains(t, err, "was not found")
	assert.Zero(t, id)
}

func TestBuildFound(t *testing.T) {
	expected := int64(1)
	mockResp := &buildbucketpb.Build{
		Id:      expected,
		Status:  buildbucketpb.Status_SUCCESS,
		EndTime: timestamppb.Now(),
		Input: &buildbucketpb.Build_Input{
			GerritChanges: []*buildbucketpb.GerritChange{},
		},
	}

	ctx := context.Background()
	mb := &mocks.BuildbucketClient{}
	bc := buildChromeImpl{
		client: mb,
	}
	device := "linux-perf"
	fakeCommit := "fake-commit"
	var patches []*buildbucketpb.GerritChange = nil

	mb.On("GetSingleBuild", testutils.AnyContext, "Linux Builder Perf", backends.DefaultBucket, "fake-commit", mock.Anything, patches).Return(mockResp, nil)

	id, err := bc.SearchOrBuild(ctx, "fake-jID", fakeCommit, device, "fake-target", map[string]interface{}{}, patches)
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
			mb := &mocks.BuildbucketClient{}
			bc := buildChromeImpl{
				client: mb,
			}
			device := "linux-perf"
			target := "fake-target"
			commit := "fake-commit"
			var patches []*buildbucketpb.GerritChange = nil

			builder := "Linux Builder Perf"

			mb.On("GetSingleBuild", testutils.AnyContext, builder, backends.DefaultBucket, commit, mock.Anything, patches).Return(nil, nil)
			mb.On("GetBuildFromWaterfall", testutils.AnyContext, builder, commit).Return(nil, nil)

			if test.expectedError {
				mb.On("StartChromeBuild", testutils.AnyContext, mock.Anything, mock.Anything, builder, commit, mock.Anything, patches).Return(nil, fmt.Errorf("some error"))
			} else {
				mb.On("StartChromeBuild", testutils.AnyContext, mock.Anything, mock.Anything, builder, commit, mock.Anything, patches).Return(test.mockResp, nil)
			}

			id, err := bc.SearchOrBuild(ctx, "fake-jID", commit, device, target, map[string]interface{}{}, patches)
			if test.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, test.expected, id)
		})
	}
}

func TestRetrieveCAS(t *testing.T) {
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
			mb := &mocks.BuildbucketClient{}
			bc := buildChromeImpl{
				client: mb,
			}
			buildID := int64(1)
			target := "fake-target"
			if test.expectedError {
				mb.On("GetCASReference", testutils.AnyContext, buildID, target).Return(nil, fmt.Errorf("some error"))
			} else {
				mb.On("GetCASReference", testutils.AnyContext, buildID, target).Return(test.mockResp, nil)
			}

			cas, err := bc.RetrieveCAS(ctx, buildID, target)
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
