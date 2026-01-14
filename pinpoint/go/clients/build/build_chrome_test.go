package build

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/pinpoint/go/backends/mocks"
	"go.skia.org/infra/pinpoint/go/common"
	"go.skia.org/infra/pinpoint/go/workflows"

	buildbucketpb "go.chromium.org/luci/buildbucket/proto"
	swarmingV2 "go.chromium.org/luci/swarming/proto/api_v2"
)

func TestCreateFindBuildRequest_OnlyEssentials_ValidRequest(t *testing.T) {
	device := "win11-perf"
	commit := common.NewCombinedCommit(common.NewChromiumCommit("random_hash"))
	params := workflows.BuildParams{
		Device: device,
		Commit: commit,
	}

	client := &buildChromeClient{
		BuildbucketClient: &mocks.BuildbucketClient{},
	}
	req, err := client.CreateFindBuildRequest(params)
	require.NoError(t, err)
	assert.Equal(t, device, req.Request.(*ChromeFindBuildRequest).Device)
	assert.Equal(t, "random_hash", req.Request.(*ChromeFindBuildRequest).Commit)
	assert.Empty(t, req.Request.(*ChromeFindBuildRequest).Deps)
	assert.Empty(t, req.Request.(*ChromeFindBuildRequest).Patches)
}

func TestCreateFindBuildRequest_MissingCommit_Error(t *testing.T) {
	device := "win11-perf"
	params := workflows.BuildParams{
		Device: device,
	}

	client := &buildChromeClient{
		BuildbucketClient: &mocks.BuildbucketClient{},
	}
	req, err := client.CreateFindBuildRequest(params)
	require.Nil(t, req)
	assert.Error(t, err, "Missing required fields")
}

func TestFindBuild_UnsupportedBotConfig_Error(t *testing.T) {
	ctx := context.Background()
	device := "random-nonexistent-device"
	params := workflows.BuildParams{
		Device: device,
		Commit: common.NewCombinedCommit(common.NewChromiumCommit("random_hash")),
	}
	client := &buildChromeClient{
		BuildbucketClient: &mocks.BuildbucketClient{},
	}
	req, err := client.CreateFindBuildRequest(params)
	require.NoError(t, err)

	resp, err := client.FindBuild(ctx, req)
	require.Nil(t, resp)
	assert.Error(t, err, "Unsupported device value provided")
}

func TestFindBuild_ValidReq_BuildNotFound(t *testing.T) {
	ctx := context.Background()
	device := "linux-r350-perf"
	params := workflows.BuildParams{
		Device: device,
		Commit: common.NewCombinedCommit(common.NewChromiumCommit("random_hash")),
	}

	mockClient := &mocks.BuildbucketClient{}
	client := &buildChromeClient{
		BuildbucketClient: mockClient,
	}
	req, err := client.CreateFindBuildRequest(params)
	require.NoError(t, err)

	mockClient.On("GetSingleBuild", testutils.AnyContext, "Linux Builder Perf", DefaultBucket, "random_hash", mock.Anything, mock.Anything).Return(nil, nil)
	mockClient.On("GetBuildFromWaterfall", testutils.AnyContext, "Linux Builder Perf", "random_hash").Return(nil, nil)

	resp, err := client.FindBuild(ctx, req)
	assert.Nil(t, resp.Response)
	assert.Nil(t, err)
}

func TestFindBuild_ValidReqWithPatch_BuildNotFound(t *testing.T) {
	ctx := context.Background()
	device := "linux-r350-perf"
	params := workflows.BuildParams{
		Device: device,
		Commit: common.NewCombinedCommit(common.NewChromiumCommit("random_hash")),
		Patch: []*buildbucketpb.GerritChange{
			{
				Host:     "chromium-review.googlesource.com",
				Change:   1318136,
				Patchset: 1,
			},
		},
	}

	mockClient := &mocks.BuildbucketClient{}
	client := &buildChromeClient{
		BuildbucketClient: mockClient,
	}
	req, err := client.CreateFindBuildRequest(params)
	require.NoError(t, err)

	mockClient.On("GetSingleBuild", testutils.AnyContext, "Linux Builder Perf", DefaultBucket, "random_hash", mock.Anything, mock.Anything).Return(nil, nil)

	resp, err := client.FindBuild(ctx, req)
	assert.Nil(t, resp.Response)
	assert.Nil(t, err)
}

func TestFindBuild_ValidReq_BuildFound(t *testing.T) {
	ctx := context.Background()
	device := "linux-r350-perf"
	params := workflows.BuildParams{
		Device: device,
		Commit: common.NewCombinedCommit(common.NewChromiumCommit("random_hash")),
	}

	mockClient := &mocks.BuildbucketClient{}
	client := &buildChromeClient{
		BuildbucketClient: mockClient,
	}
	req, err := client.CreateFindBuildRequest(params)
	require.NoError(t, err)

	respBuild := &buildbucketpb.Build{
		Id: int64(12345),
	}

	mockClient.On("GetSingleBuild", testutils.AnyContext, "Linux Builder Perf", "try", "random_hash", mock.Anything, mock.Anything).Return(respBuild, nil)

	resp, err := client.FindBuild(ctx, req)
	require.Nil(t, err)
	assert.Equal(t, int64(12345), resp.BuildID)
}

func TestFindBuild_ValidReq_BuildbucketFailure(t *testing.T) {
	ctx := context.Background()
	device := "linux-r350-perf"
	params := workflows.BuildParams{
		Device: device,
		Commit: common.NewCombinedCommit(common.NewChromiumCommit("random_hash")),
	}

	mockClient := &mocks.BuildbucketClient{}
	client := &buildChromeClient{
		BuildbucketClient: mockClient,
	}
	req, err := client.CreateFindBuildRequest(params)
	require.NoError(t, err)

	mockClient.On("GetSingleBuild", testutils.AnyContext, "Linux Builder Perf", "try", "random_hash", mock.Anything, mock.Anything).Return(nil, skerr.Fmt("an error!"))

	resp, err := client.FindBuild(ctx, req)
	require.Nil(t, resp)
	assert.Error(t, err, "Failed to search for Chrome build")
}

func TestCreateStartBuildRequest_MissingCommit_Error(t *testing.T) {
	device := "win11-perf"
	params := workflows.BuildParams{
		Device: device,
	}

	client := &buildChromeClient{
		BuildbucketClient: &mocks.BuildbucketClient{},
	}
	req, err := client.CreateStartBuildRequest(params)
	require.Nil(t, req)
	assert.Error(t, err, "Missing required fields")
}

func TestCreateStartBuildRequest_UnsupportedBotConfig_Error(t *testing.T) {
	device := "random-nonexistent-device"
	params := workflows.BuildParams{
		Device: device,
		Commit: common.NewCombinedCommit(common.NewChromiumCommit("random_hash")),
	}

	client := &buildChromeClient{
		BuildbucketClient: &mocks.BuildbucketClient{},
	}
	req, err := client.CreateStartBuildRequest(params)
	require.Nil(t, req)
	assert.Error(t, err, "Unsupported device value provided")
}

func TestCreateStartBuildRequest_NoDeps_ValidResponse(t *testing.T) {
	device := "linux-r350-perf"
	workflowId := uuid.New().String()
	params := workflows.BuildParams{
		Device:     device,
		Commit:     common.NewCombinedCommit(common.NewChromiumCommit("random_hash")),
		WorkflowID: workflowId,
	}

	client := &buildChromeClient{
		BuildbucketClient: &mocks.BuildbucketClient{},
	}
	startReq, err := client.CreateStartBuildRequest(params)
	require.NoError(t, err)

	assert.Equal(t, DefaultBucket, startReq.Request.(*buildbucketpb.ScheduleBuildRequest).Builder.Bucket)
	assert.Equal(t, 4, len(startReq.Request.(*buildbucketpb.ScheduleBuildRequest).Properties.Fields))
	pinpointJobIdTag := startReq.Request.(*buildbucketpb.ScheduleBuildRequest).Tags[0]
	assert.Equal(t, workflowId, pinpointJobIdTag.Value)
}

func TestCreateStartBuildRequest_WithPatch_ValidResponse(t *testing.T) {
	device := "linux-r350-perf"
	workflowId := uuid.New().String()
	params := workflows.BuildParams{
		Device: device,
		Commit: common.NewCombinedCommit(common.NewChromiumCommit("random_hash")),
		Patch: []*buildbucketpb.GerritChange{
			{
				Host:     "chromium-review.googlesource.com",
				Change:   1318136,
				Patchset: 1,
			},
		},
		WorkflowID: workflowId,
	}

	client := &buildChromeClient{
		BuildbucketClient: &mocks.BuildbucketClient{},
	}
	startReq, err := client.CreateStartBuildRequest(params)
	require.NoError(t, err)

	assert.Equal(t, DefaultBucket, startReq.Request.(*buildbucketpb.ScheduleBuildRequest).Builder.Bucket)
	assert.Equal(t, 4, len(startReq.Request.(*buildbucketpb.ScheduleBuildRequest).Properties.Fields))
	pinpointJobIdTag := startReq.Request.(*buildbucketpb.ScheduleBuildRequest).Tags[0]
	assert.Equal(t, 1, len(startReq.Request.(*buildbucketpb.ScheduleBuildRequest).GerritChanges))
	assert.Equal(t, params.Patch[0], startReq.Request.(*buildbucketpb.ScheduleBuildRequest).GerritChanges[0])
	assert.Equal(t, workflowId, pinpointJobIdTag.Value)
}

func TestStartBuild_ValidReq_NoError(t *testing.T) {
	ctx := context.Background()
	device := "linux-r350-perf"
	workflowId := uuid.New().String()
	params := workflows.BuildParams{
		Device:     device,
		Commit:     common.NewCombinedCommit(common.NewChromiumCommit("random_hash")),
		WorkflowID: workflowId,
	}

	mockClient := &mocks.BuildbucketClient{}
	client := &buildChromeClient{
		BuildbucketClient: mockClient,
	}

	startReq, err := client.CreateStartBuildRequest(params)
	require.NoError(t, err)

	expectedResp := &buildbucketpb.Build{
		Id: int64(12345),
	}

	mockClient.On("StartBuild", testutils.AnyContext, startReq.Request.(*buildbucketpb.ScheduleBuildRequest)).Return(expectedResp, nil)
	resp, err := client.StartBuild(ctx, startReq)
	require.NoError(t, err)
	assert.Equal(t, expectedResp, resp.Response)
}

func TestGetStatus_ExistingBuild_NoError(t *testing.T) {
	ctx := context.Background()

	buildId := int64(12345)

	mockClient := &mocks.BuildbucketClient{}
	client := &buildChromeClient{
		BuildbucketClient: mockClient,
	}

	status := buildbucketpb.Status_STARTED

	mockClient.On("GetBuildStatus", testutils.AnyContext, buildId).Return(status, nil)

	resp, err := client.GetStatus(ctx, buildId)
	require.NoError(t, err)
	assert.Equal(t, status, resp)
}

func TestGetBuildArtifact_ValidRequest_NoArtifact(t *testing.T) {
	ctx := context.Background()
	req := &GetBuildArtifactRequest{
		BuildID: int64(12345),
		Target:  "performance_test_suite",
	}

	mockClient := &mocks.BuildbucketClient{}
	client := &buildChromeClient{
		BuildbucketClient: mockClient,
	}

	mockClient.On("GetCASReference", testutils.AnyContext, req.BuildID, req.Target).Return(nil, nil)

	resp, err := client.GetBuildArtifact(ctx, req)
	require.NoError(t, err)
	assert.Nil(t, resp.Response)
}

func TestGetBuildArtifact_ValidRequest_ArtifactFound(t *testing.T) {
	ctx := context.Background()
	req := &GetBuildArtifactRequest{
		BuildID: int64(12345),
		Target:  "performance_test_suite",
	}

	mockClient := &mocks.BuildbucketClient{}
	client := &buildChromeClient{
		BuildbucketClient: mockClient,
	}

	casResp := &swarmingV2.CASReference{
		CasInstance: "DefaultCASInstance",
		Digest: &swarmingV2.Digest{
			Hash:      "somerandomhash",
			SizeBytes: int64(1),
		},
	}

	mockClient.On("GetCASReference", testutils.AnyContext, req.BuildID, req.Target).Return(casResp, nil)

	resp, err := client.GetBuildArtifact(ctx, req)
	require.NoError(t, err)
	assert.Equal(t, resp.Response, casResp)
}

func TestCancelBuild_ValidReq_NoError(t *testing.T) {
	ctx := context.Background()

	req := &CancelBuildRequest{
		BuildID: int64(12345),
		Reason:  "i want to cancel",
	}

	mockClient := &mocks.BuildbucketClient{}
	client := &buildChromeClient{
		BuildbucketClient: mockClient,
	}

	mockClient.On("CancelBuild", testutils.AnyContext, req.BuildID, req.Reason).Return(nil)

	err := client.CancelBuild(ctx, req)
	require.NoError(t, err)
}
