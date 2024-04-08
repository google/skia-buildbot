package build_chrome

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	buildbucketpb "go.chromium.org/luci/buildbucket/proto"
	swarmingV1 "go.chromium.org/luci/common/api/swarming/swarming/v1"

	"google.golang.org/protobuf/types/known/timestamppb"

	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/pinpoint/go/backends"
	"go.skia.org/infra/pinpoint/go/backends/mocks"
)

const (
	fakeCommit  = "fake-commit"
	fakeBuilder = "builder"
)

func TestSearchBuild_BuildbucketError_ReturnsError(t *testing.T) {
	var patches []*buildbucketpb.GerritChange

	ctx := context.Background()

	mb := &mocks.BuildbucketClient{}
	deps := map[string]string{}
	bc := &buildChromeImpl{
		BuildbucketClient: mb,
	}

	mb.On("GetSingleBuild", testutils.AnyContext, fakeBuilder, backends.DefaultBucket, fakeCommit, deps, patches).Return(nil, fmt.Errorf("random error"))
	mb.On("GetBuildFromWaterfall", testutils.AnyContext, fakeBuilder, fakeCommit).Return(nil, nil)
	_, err := bc.searchBuild(ctx, fakeBuilder, fakeCommit, deps, patches)
	require.Error(t, err)
}

func TestSearchBuild_PinpointBuild_ReturnsBuildId(t *testing.T) {
	const expectedBuildId = int64(1)
	const successBuild = buildbucketpb.Status_SUCCESS
	var patches []*buildbucketpb.GerritChange

	ctx := context.Background()

	mb := &mocks.BuildbucketClient{}
	deps := map[string]string{}
	bc := &buildChromeImpl{
		BuildbucketClient: mb,
	}
	mockResp := &buildbucketpb.Build{
		Id:      expectedBuildId,
		Status:  successBuild,
		EndTime: timestamppb.Now(),
		Input: &buildbucketpb.Build_Input{
			GerritChanges: []*buildbucketpb.GerritChange{},
		},
	}

	mb.On("GetSingleBuild", testutils.AnyContext, fakeBuilder, backends.DefaultBucket, fakeCommit, deps, patches).Return(mockResp, nil)
	mb.On("GetBuildFromWaterfall", testutils.AnyContext, fakeBuilder, fakeCommit).Return(mockResp, nil)

	id, err := bc.searchBuild(ctx, fakeBuilder, fakeCommit, deps, patches)
	assert.NoError(t, err)
	assert.Equal(t, expectedBuildId, id)
}

func TestSearchBuild_WaterfallBuild_ReturnsBuildIdCICounterpart(t *testing.T) {
	var patches []*buildbucketpb.GerritChange

	ctx := context.Background()

	mb := &mocks.BuildbucketClient{}
	deps := map[string]string{}
	bc := &buildChromeImpl{
		BuildbucketClient: mb,
	}
	mockResp := &buildbucketpb.Build{
		Id:      1,
		Status:  buildbucketpb.Status_FAILURE,
		EndTime: timestamppb.Now(),
		Input: &buildbucketpb.Build_Input{
			GerritChanges: []*buildbucketpb.GerritChange{},
		},
	}

	mb.On("GetSingleBuild", testutils.AnyContext, fakeBuilder, backends.DefaultBucket, fakeCommit, deps, patches).Return(nil, fmt.Errorf("random error"))
	mb.On("GetBuildFromWaterfall", testutils.AnyContext, fakeBuilder, fakeCommit).Return(mockResp, nil)

	_, err := bc.searchBuild(ctx, fakeBuilder, fakeCommit, deps, patches)
	assert.Error(t, err)
}

func TestSearchBuild_GivenCommitWithNoPriorBuild_ReturnsZero(t *testing.T) {
	// TODO(b/332984841): viditchitkara@ make int64(0) a module level constant that
	// represents unknown build id.
	const expectedBuildId = int64(0)
	var patches []*buildbucketpb.GerritChange

	ctx := context.Background()

	mb := &mocks.BuildbucketClient{}
	deps := map[string]string{}
	bc := &buildChromeImpl{
		BuildbucketClient: mb,
	}

	mb.On("GetSingleBuild", testutils.AnyContext, fakeBuilder, backends.DefaultBucket, fakeCommit, deps, patches).Return(nil, nil)
	mb.On("GetBuildFromWaterfall", testutils.AnyContext, fakeBuilder, fakeCommit).Return(nil, nil)

	id, err := bc.searchBuild(ctx, fakeBuilder, fakeCommit, deps, patches)
	require.NoError(t, err)
	assert.Equal(t, expectedBuildId, id)
}

func TestCheckBuildStatus_SuccessfulBuild_ReturnsSuccessStatus(t *testing.T) {
	const buildID = int64(0)
	const buildSuccess = buildbucketpb.Status_SUCCESS

	ctx := context.Background()

	mb := &mocks.BuildbucketClient{}
	bc := &buildChromeImpl{
		BuildbucketClient: mb,
	}

	mb.On("GetBuildStatus", testutils.AnyContext, buildID).Return(buildSuccess, nil)

	status, err := bc.GetStatus(ctx, buildID)
	require.NoError(t, err)
	assert.Equal(t, buildSuccess, status)
}

func TestCheckBuildStatus_FailedBuild_ReturnsFailureStatus(t *testing.T) {
	const buildID = int64(0)
	const buildFailure = buildbucketpb.Status_FAILURE

	ctx := context.Background()

	mb := &mocks.BuildbucketClient{}
	bc := &buildChromeImpl{
		BuildbucketClient: mb,
	}

	mb.On("GetBuildStatus", testutils.AnyContext, buildID).Return(buildFailure, nil)

	status, err := bc.GetStatus(ctx, buildID)
	require.NoError(t, err)
	assert.Equal(t, buildFailure, status)
}

func TestCheckBuildStatus_BuildbucketThrowsError_ReturnsError(t *testing.T) {
	const buildID = int64(0)

	ctx := context.Background()

	mb := &mocks.BuildbucketClient{}
	bc := &buildChromeImpl{
		BuildbucketClient: mb,
	}

	mb.On("GetBuildStatus", testutils.AnyContext, buildID).Return(buildbucketpb.Status_STATUS_UNSPECIFIED, fmt.Errorf("some error"))

	_, err := bc.GetStatus(ctx, buildID)

	assert.Error(t, err)
}

func TestBuild_WithNonExistentDevice_ReturnsError(t *testing.T) {
	ctx := context.Background()

	mb := &mocks.BuildbucketClient{}
	bc := buildChromeImpl{
		BuildbucketClient: mb,
	}

	_, err := bc.SearchOrBuild(ctx, "fake-jID", "fake-commit", "non-existent device", nil, nil)
	assert.ErrorContains(t, err, "was not found")
}

func TestBuildFound_WithValidParams_ReturnsBuildId(t *testing.T) {
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
		BuildbucketClient: mb,
	}
	const device = "linux-perf"
	const builder = "Linux Builder Perf"
	var patches []*buildbucketpb.GerritChange

	mb.On("GetSingleBuild", testutils.AnyContext, builder, backends.DefaultBucket, fakeCommit, mock.Anything, patches).Return(mockResp, nil)

	id, err := bc.SearchOrBuild(ctx, "fake-jID", fakeCommit, device, map[string]string{}, patches)
	require.NoError(t, err)
	assert.Equal(t, expected, id)
}

func TestNewBuild_WithValidBuildParams_ReturnsBuildId(t *testing.T) {
	ctx := context.Background()
	mb := &mocks.BuildbucketClient{}
	bc := buildChromeImpl{
		BuildbucketClient: mb,
	}
	const device = "linux-perf"
	const builder = "Linux Builder Perf"
	var patches []*buildbucketpb.GerritChange
	const expectedId = int64(8757340904619224433)
	mockResp := &buildbucketpb.Build{
		Id: expectedId,
	}

	mb.On("GetSingleBuild", testutils.AnyContext, builder, backends.DefaultBucket, fakeCommit, mock.Anything, patches).Return(nil, nil)
	mb.On("GetBuildFromWaterfall", testutils.AnyContext, builder, fakeCommit).Return(nil, nil)
	mb.On("StartChromeBuild", testutils.AnyContext, mock.Anything, mock.Anything, builder, fakeCommit, mock.Anything, patches).Return(mockResp, nil)

	id, err := bc.SearchOrBuild(ctx, "fake-jID", fakeCommit, device, map[string]string{}, patches)
	require.NoError(t, err)
	assert.Equal(t, expectedId, id)
}

func TestNewBuild_WithInvalidBuildParams_ReturnsError(t *testing.T) {
	ctx := context.Background()
	mb := &mocks.BuildbucketClient{}
	bc := buildChromeImpl{
		BuildbucketClient: mb,
	}
	const device = "linux-perf"
	const builder = "Linux Builder Perf"
	var patches []*buildbucketpb.GerritChange

	mb.On("GetSingleBuild", testutils.AnyContext, builder, backends.DefaultBucket, fakeCommit, mock.Anything, patches).Return(nil, nil)
	mb.On("GetBuildFromWaterfall", testutils.AnyContext, builder, fakeCommit).Return(nil, nil)
	mb.On("StartChromeBuild", testutils.AnyContext, mock.Anything, mock.Anything, builder, fakeCommit, mock.Anything, patches).Return(nil, fmt.Errorf("some error"))

	_, err := bc.SearchOrBuild(ctx, "fake-jID", fakeCommit, device, map[string]string{}, patches)
	require.Error(t, err)
}

func TestRetrieveCAS_BuildIdAndTarget_ReturnsCasReference(t *testing.T) {
	ctx := context.Background()
	mb := &mocks.BuildbucketClient{}
	bc := buildChromeImpl{
		BuildbucketClient: mb,
	}
	const buildID = int64(1)
	const target = "fake-target"

	mockResp := &swarmingV1.SwarmingRpcsCASReference{
		CasInstance: backends.DefaultCASInstance,
		Digest: &swarmingV1.SwarmingRpcsDigest{
			Hash:      "6e75b9064d8ce9a16c3815af13709f05556da586460587a5155e599aafea4a93",
			SizeBytes: 1294,
		},
	}
	mb.On("GetCASReference", testutils.AnyContext, buildID, target).Return(mockResp, nil)

	cas, err := bc.RetrieveCAS(ctx, buildID, target)
	require.NoError(t, err)
	assert.Equal(t, "6e75b9064d8ce9a16c3815af13709f05556da586460587a5155e599aafea4a93", cas.Digest.Hash)
	assert.Equal(t, int64(1294), cas.Digest.SizeBytes)
}

func TestRetrieveCAS_InvalidBuildIdAndTarget_ReturnsError(t *testing.T) {
	ctx := context.Background()
	mb := &mocks.BuildbucketClient{}
	bc := buildChromeImpl{
		BuildbucketClient: mb,
	}

	const buildID = int64(1)
	const target = "fake-target"
	mb.On("GetCASReference", testutils.AnyContext, buildID, target).Return(nil, fmt.Errorf("some error"))

	_, err := bc.RetrieveCAS(ctx, buildID, target)
	assert.Error(t, err)
}
