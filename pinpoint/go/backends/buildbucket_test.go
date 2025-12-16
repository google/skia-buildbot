package backends

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"

	"go.chromium.org/luci/grpc/appstatus"
	"go.skia.org/infra/go/buildbucket"

	bpb "go.chromium.org/luci/buildbucket/proto"
	bgrpcpb "go.chromium.org/luci/buildbucket/proto/grpcpb"
	spb "google.golang.org/protobuf/types/known/structpb"
	timestamppb "google.golang.org/protobuf/types/known/timestamppb"
)

const (
	TestHost = "chromium-review.googlesource.com"
)

func TestBuildbucketClientConfigs_Defaults_Client(t *testing.T) {
	c := DefaultClientConfig()

	assert.Equal(t, buildbucket.DEFAULT_HOST, c.Host)
	assert.Equal(t, DefaultRetries, c.Retries)
	assert.Equal(t, DefaultPerRPCTimeout, c.PerRPCTimeout)
}

func createGerritChange(host, project string, change, patchset int64) *bpb.GerritChange {
	return &bpb.GerritChange{
		Host:     host,
		Project:  project,
		Change:   change,
		Patchset: patchset,
	}
}

func createBuild(id int64, status bpb.Status, endTime *timestamppb.Timestamp, bucket, builder string, patches []*bpb.GerritChange) *bpb.Build {
	return &bpb.Build{
		Id:     id,
		Status: status,
		// This build finished 31 days from runtime, just outside Cas.
		EndTime: endTime,
		Builder: &bpb.BuilderID{
			Project: ChromeProject,
			Bucket:  bucket,
			Builder: builder,
		},
		Input: &bpb.Build_Input{
			GerritChanges: patches,
		},
	}
}

func TestCancelBuild_ValidRequest_NoError(t *testing.T) {
	ctx := context.Background()
	buildID := int64(12345)
	summary := "no longer needed"

	ctl := gomock.NewController(t)
	defer ctl.Finish()

	mbc := bgrpcpb.NewMockBuildsClient(ctl)
	c := NewBuildbucketClient(mbc)

	req := &bpb.CancelBuildRequest{
		Id:              buildID,
		SummaryMarkdown: summary,
	}

	mbc.EXPECT().CancelBuild(ctx, req).Return(nil, nil)

	err := c.CancelBuild(ctx, buildID, summary)
	assert.NoError(t, err)
}

func TestCancelBuild_ValidRequest_Error(t *testing.T) {
	ctx := context.Background()
	buildID := int64(12345)
	summary := "no longer needed"

	ctl := gomock.NewController(t)
	defer ctl.Finish()

	mbc := bgrpcpb.NewMockBuildsClient(ctl)
	c := NewBuildbucketClient(mbc)

	req := &bpb.CancelBuildRequest{
		Id:              buildID,
		SummaryMarkdown: summary,
	}

	resp := appstatus.BadRequest(errors.New("random error"))
	mbc.EXPECT().CancelBuild(ctx, req).Return(nil, resp)

	err := c.CancelBuild(ctx, buildID, summary)
	assert.ErrorContains(t, err, "Failed to cancel build 12345")
}

func TestGetBuildWithPatches_OnePatch_MatchFound(t *testing.T) {
	ctx := context.Background()

	builder := "builder"
	commit := "12345"

	c1ps1 := createGerritChange(TestHost, ChromeProject, 54321, 1)

	expiredTime := timestamppb.New(time.Now().AddDate(0, -31, 0))
	patches := []*bpb.GerritChange{c1ps1}

	build1 := createBuild(1, bpb.Status_SUCCESS, expiredTime, DefaultBucket, builder, patches)
	build2 := createBuild(2, bpb.Status_STARTED, nil, DefaultBucket, builder, patches)

	ctl := gomock.NewController(t)
	defer ctl.Finish()

	mbc := bgrpcpb.NewMockBuildsClient(ctl)
	c := NewBuildbucketClient(mbc)

	req := c.createSearchBuildRequest(builder, DefaultBucket, commit, patches)
	assert.Equal(t, ChromeProject, req.Predicate.Builder.Project)

	// response in reverse chronological order.
	resp := &bpb.SearchBuildsResponse{
		Builds: []*bpb.Build{
			build2,
			build1,
		},
	}
	mbc.EXPECT().SearchBuilds(ctx, req).Return(resp, nil)

	build, err := c.GetBuildWithPatches(ctx, builder, DefaultBucket, commit, patches)
	assert.NoError(t, err)
	assert.Equal(t, int64(2), build.GetId())
	assert.Equal(t, patches, build.Input.GerritChanges)
}

func TestGetBuildWithPatches_OldBuild_NoMatch(t *testing.T) {
	ctx := context.Background()

	builder := "builder"
	commit := "12345"

	c1ps1 := createGerritChange(TestHost, ChromeProject, 54321, 1)

	expiredTime := timestamppb.New(time.Now().AddDate(0, -31, 0))
	patches := []*bpb.GerritChange{c1ps1}

	build1 := createBuild(1, bpb.Status_SUCCESS, expiredTime, DefaultBucket, builder, patches)

	ctl := gomock.NewController(t)
	defer ctl.Finish()

	mbc := bgrpcpb.NewMockBuildsClient(ctl)
	c := NewBuildbucketClient(mbc)

	req := c.createSearchBuildRequest(builder, DefaultBucket, commit, patches)
	resp := &bpb.SearchBuildsResponse{
		Builds: []*bpb.Build{
			build1,
		},
	}
	mbc.EXPECT().SearchBuilds(ctx, req).Return(resp, nil)

	build, err := c.GetBuildWithPatches(ctx, builder, DefaultBucket, commit, patches)
	assert.NoError(t, err)
	assert.Nil(t, build)
}

func TestGetBuildWithPatches_FailedBuild_NoMatch(t *testing.T) {
	ctx := context.Background()

	builder := "builder"
	commit := "12345"

	c1ps1 := createGerritChange(TestHost, ChromeProject, 54321, 1)
	patches := []*bpb.GerritChange{c1ps1}

	ctl := gomock.NewController(t)
	defer ctl.Finish()

	mbc := bgrpcpb.NewMockBuildsClient(ctl)
	c := NewBuildbucketClient(mbc)

	req := c.createSearchBuildRequest(builder, DefaultBucket, commit, patches)
	// 1 day old
	ts := timestamppb.New(time.Now().AddDate(0, 1, 0))
	resp := &bpb.SearchBuildsResponse{
		Builds: []*bpb.Build{
			createBuild(1, bpb.Status_FAILURE, ts, DefaultBucket, builder, patches),
		},
	}
	mbc.EXPECT().SearchBuilds(ctx, req).Return(resp, nil)

	build, err := c.GetBuildWithPatches(ctx, builder, DefaultBucket, commit, patches)
	assert.NoError(t, err)
	assert.Nil(t, build)
}

func TestGetBuildWithPatches_NilPatches_NoMatch(t *testing.T) {
	ctx := context.Background()

	builder := "builder"
	commit := "12345"

	ctl := gomock.NewController(t)
	defer ctl.Finish()

	mbc := bgrpcpb.NewMockBuildsClient(ctl)
	c := NewBuildbucketClient(mbc)

	req := c.createSearchBuildRequest(builder, DefaultBucket, commit, nil)
	// 1 day old
	ts := timestamppb.New(time.Now().AddDate(0, 1, 0))
	resp := &bpb.SearchBuildsResponse{
		Builds: []*bpb.Build{
			createBuild(1, bpb.Status_FAILURE, ts, DefaultBucket, builder, nil),
		},
	}
	mbc.EXPECT().SearchBuilds(ctx, req).Return(resp, nil)

	build, err := c.GetBuildWithPatches(ctx, builder, DefaultBucket, commit, nil)
	assert.NoError(t, err)
	assert.Nil(t, build)
}

func TestGetBuildWithDeps_ValidInputs_MatchingBuild(t *testing.T) {
	ctx := context.Background()

	builder := "builder"
	commit := "12345"
	webrtc := "https://webrtc.googlesource.com/src"

	ctl := gomock.NewController(t)
	defer ctl.Finish()

	mbc := bgrpcpb.NewMockBuildsClient(ctl)
	c := NewBuildbucketClient(mbc)

	req := c.createSearchBuildRequest(builder, DefaultBucket, commit, nil)
	depsOverride := map[string]*spb.Value{
		// ignored
		"foo": {},
		// parsed
		DepsOverrideKey: {
			Kind: &spb.Value_StructValue{
				StructValue: &spb.Struct{
					Fields: map[string]*spb.Value{
						webrtc: {
							Kind: &spb.Value_StringValue{
								StringValue: "1",
							},
						},
					},
				},
			},
		},
	}
	resp := &bpb.SearchBuildsResponse{
		Builds: []*bpb.Build{
			{
				Id:      1,
				Status:  bpb.Status_SUCCESS,
				EndTime: timestamppb.New(time.Now().AddDate(0, 1, 0)),
				Builder: &bpb.BuilderID{
					Project: ChromeProject,
					Bucket:  DefaultBucket,
					Builder: builder,
				},
				Input: &bpb.Build_Input{
					Properties: &spb.Struct{
						Fields: depsOverride,
					},
				},
			},
		},
	}
	mbc.EXPECT().SearchBuilds(ctx, req).Return(resp, nil)

	deps := map[string]string{
		webrtc: "1",
	}
	assert.NotNil(t, c.findMatchingBuild(resp.GetBuilds(), deps, nil))

	build, err := c.GetBuildWithDeps(ctx, builder, DefaultBucket, commit, deps)
	assert.NoError(t, err)
	assert.Equal(t, int64(1), build.GetId())
	assert.Nil(t, build.Input.GerritChanges)
	assert.Equal(t, "1", build.Input.Properties.Fields[DepsOverrideKey].GetStructValue().AsMap()[webrtc].(string))
}

func TestGetBuildFromWaterfall_ExistingMapping_BuildFound(t *testing.T) {
	ctx := context.Background()

	builder := "Linux Builder Perf"
	mirror, _ := PinpointWaterfall[builder]
	commit := "12345"

	// must be CI bucket, aka WaterfallBucket
	ts := timestamppb.New(time.Now().AddDate(0, 1, 0))
	build1 := createBuild(1, bpb.Status_SUCCESS, ts, WaterfallBucket, mirror, nil)
	build2 := createBuild(2, bpb.Status_STARTED, nil, WaterfallBucket, mirror, nil)

	ctl := gomock.NewController(t)
	defer ctl.Finish()

	mbc := bgrpcpb.NewMockBuildsClient(ctl)
	c := NewBuildbucketClient(mbc)

	req := c.createSearchBuildRequest(mirror, WaterfallBucket, commit, nil)
	// response in reverse chronological order.
	resp := &bpb.SearchBuildsResponse{
		Builds: []*bpb.Build{
			build2,
			build1,
		},
	}
	mbc.EXPECT().SearchBuilds(ctx, req).Return(resp, nil)

	build, err := c.GetBuildFromWaterfall(ctx, builder, commit)
	assert.NoError(t, err)
	assert.Equal(t, int64(2), build.GetId())
}

func TestGetBuildFromWaterfall_UnsupportedBuilder_Error(t *testing.T) {
	ctx := context.Background()

	commit := "12345"

	ctl := gomock.NewController(t)
	defer ctl.Finish()

	mbc := bgrpcpb.NewMockBuildsClient(ctl)
	c := NewBuildbucketClient(mbc)

	b, err := c.GetBuildFromWaterfall(ctx, "builder", commit)
	assert.ErrorContains(t, err, "has no supported CI waterfall builder")
	assert.Nil(t, b)
}

func TestGetBuildStatus_ValidRequest_StatusStarted(t *testing.T) {
	ctx := context.Background()
	buildID := int64(12345)

	ctl := gomock.NewController(t)
	defer ctl.Finish()

	mbc := bgrpcpb.NewMockBuildsClient(ctl)
	c := NewBuildbucketClient(mbc)

	req := &bpb.GetBuildStatusRequest{
		Id: buildID,
	}
	resp := createBuild(buildID, bpb.Status_STARTED, nil, "try", "builder", nil)
	mbc.EXPECT().GetBuildStatus(ctx, req).Return(resp, nil)

	status, err := c.GetBuildStatus(ctx, buildID)
	assert.NoError(t, err)
	assert.Equal(t, bpb.Status_STARTED, status)
}

func TestGetBuildStatus_Error_BuildStatusUnspecified(t *testing.T) {
	ctx := context.Background()
	buildID := int64(12345)

	ctl := gomock.NewController(t)
	defer ctl.Finish()

	mbc := bgrpcpb.NewMockBuildsClient(ctl)
	c := NewBuildbucketClient(mbc)

	req := &bpb.GetBuildStatusRequest{
		Id: buildID,
	}
	resp := appstatus.BadRequest(errors.New("random error"))
	mbc.EXPECT().GetBuildStatus(ctx, req).Return(nil, resp)

	status, err := c.GetBuildStatus(ctx, buildID)
	assert.ErrorContains(t, err, "random error")
	assert.Equal(t, bpb.Status_STATUS_UNSPECIFIED, status)
}

func createCASResponse(buildID int64, status bpb.Status, target, hash string) *bpb.Build {
	return &bpb.Build{
		Id:     buildID,
		Status: status,
		Output: &bpb.Build_Output{
			Properties: &spb.Struct{
				Fields: map[string]*spb.Value{
					// ignored
					"foo": {},
					// parsed
					"swarm_hashes_refs/refs/head/main/without_patch": {
						Kind: &spb.Value_StructValue{
							StructValue: &spb.Struct{
								Fields: map[string]*spb.Value{
									target: {
										Kind: &spb.Value_StringValue{
											StringValue: hash,
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

func TestGetCASReference_ValidRequest_CASResponse(t *testing.T) {
	ctx := context.Background()

	buildID := int64(12345)
	target := "performance_test_suite"

	ctl := gomock.NewController(t)
	defer ctl.Finish()

	mbc := bgrpcpb.NewMockBuildsClient(ctl)
	c := NewBuildbucketClient(mbc)

	req := c.createCASReferenceRequest(buildID)
	assert.Equal(t, "output.properties", req.Mask.Fields.Paths[0])

	resp := createCASResponse(buildID, bpb.Status_SUCCESS, target, "somehash/123")
	mbc.EXPECT().GetBuild(ctx, req).Return(resp, nil)

	ref, err := c.GetCASReference(ctx, buildID, target)
	assert.NoError(t, err)
	assert.Equal(t, DefaultCASInstance, ref.CasInstance)
	assert.Equal(t, "somehash", ref.Digest.Hash)
	assert.Equal(t, int64(123), ref.Digest.SizeBytes)
}

func TestGetCASReference_BuildInProgress_Error(t *testing.T) {
	ctx := context.Background()

	buildID := int64(12345)
	target := "performance_test_suite"

	ctl := gomock.NewController(t)
	defer ctl.Finish()

	mbc := bgrpcpb.NewMockBuildsClient(ctl)
	c := NewBuildbucketClient(mbc)
	req := c.createCASReferenceRequest(buildID)
	resp := &bpb.Build{
		Id:     buildID,
		Status: bpb.Status_STARTED,
	}

	mbc.EXPECT().GetBuild(ctx, req).Return(resp, nil)

	ref, err := c.GetCASReference(ctx, buildID, target)
	assert.ErrorContains(t, err, "Cannot fetch CAS information from build 12345 with status STARTED")
	assert.Nil(t, ref)
}

func TestGetCASReference_MissingTargetInResponse_Error(t *testing.T) {
	ctx := context.Background()

	buildID := int64(12345)
	target := "performance_test_suite"

	ctl := gomock.NewController(t)
	defer ctl.Finish()

	mbc := bgrpcpb.NewMockBuildsClient(ctl)
	c := NewBuildbucketClient(mbc)

	req := c.createCASReferenceRequest(buildID)
	resp := createCASResponse(buildID, bpb.Status_SUCCESS, "other_test_suite", "somehash/123")

	mbc.EXPECT().GetBuild(ctx, req).Return(resp, nil)

	ref, err := c.GetCASReference(ctx, buildID, target)
	assert.ErrorContains(t, err, "The target performance_test_suite cannot be found in the output properties")
	assert.Nil(t, ref)
}

func TestGetCASReference_ModifiedCasHashFormat_Error(t *testing.T) {
	ctx := context.Background()

	buildID := int64(12345)
	target := "performance_test_suite"

	ctl := gomock.NewController(t)
	defer ctl.Finish()

	mbc := bgrpcpb.NewMockBuildsClient(ctl)
	c := NewBuildbucketClient(mbc)

	req := c.createCASReferenceRequest(buildID)
	wrongHash := "somehash/123/456"
	resp := createCASResponse(buildID, bpb.Status_SUCCESS, target, wrongHash)

	mbc.EXPECT().GetBuild(ctx, req).Return(resp, nil)

	ref, err := c.GetCASReference(ctx, buildID, target)
	assert.ErrorContains(t, err, fmt.Sprintf("CAS hash %s has been changed to an unparsable format", wrongHash))
	assert.Nil(t, ref)
}

func TestStartChromeBuild_BuildWithDeps_SuccessfullyScheduled(t *testing.T) {
	ctx := context.Background()
	builder := "Linux Builder Perf"
	commit := "12345"

	ctl := gomock.NewController(t)
	defer ctl.Finish()

	mbc := bgrpcpb.NewMockBuildsClient(ctl)
	c := NewBuildbucketClient(mbc)

	webrtc := "https://webrtc.googlesource.com/src"
	depsMap := map[string]string{
		webrtc: "1",
	}

	req := c.createChromeBuildRequest("1", "1", builder, commit, depsMap, nil)

	// Checking defaults
	assert.Equal(t, ChromeProject, req.Builder.Project)
	assert.Equal(t, DefaultBucket, req.Builder.Bucket)
	assert.Equal(t, ChromiumGitilesHost, req.GitilesCommit.Host)
	assert.Equal(t, ChromiumGitilesProject, req.GitilesCommit.Project)
	assert.Equal(t, ChromiumGitilesRefAtHead, req.GitilesCommit.Ref)

	resp := &bpb.Build{
		Id:     int64(12345),
		Status: bpb.Status_SCHEDULED,
	}
	mbc.EXPECT().ScheduleBuild(gomock.AssignableToTypeOf(ctx), req).Return(resp, nil)

	build, err := c.StartChromeBuild(ctx, "1", "1", builder, commit, depsMap, nil)
	assert.NoError(t, err)
	assert.Equal(t, bpb.Status_SCHEDULED, build.Status)
}
