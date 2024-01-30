package backends

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/golang/mock/gomock"

	"go.chromium.org/luci/grpc/appstatus"
	"go.skia.org/infra/go/buildbucket"

	. "github.com/smartystreets/goconvey/convey"
	bpb "go.chromium.org/luci/buildbucket/proto"
	. "go.chromium.org/luci/common/testing/assertions"
	spb "google.golang.org/protobuf/types/known/structpb"
	timestamppb "google.golang.org/protobuf/types/known/timestamppb"
)

const (
	TestHost = "chromium-review.googlesource.com"
)

func TestBuildbucketClientConfigs(t *testing.T) {
	Convey(`OK`, t, func() {
		Convey(`Defaults`, func() {
			c := DefaultClientConfig()

			So(c.Host, ShouldEqual, buildbucket.DEFAULT_HOST)
			So(c.Retries, ShouldEqual, DefaultRetries)
			So(c.PerRPCTimeout, ShouldEqual, DefaultPerRPCTimeout)
		})
	})
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

func TestCancelBuild(t *testing.T) {
	ctx := context.Background()

	buildID := int64(12345)
	summary := "no longer needed"

	Convey(`OK`, t, func() {
		ctl := gomock.NewController(t)
		defer ctl.Finish()

		mbc := bpb.NewMockBuildsClient(ctl)
		c := NewBuildbucketClient(mbc)

		req := &bpb.CancelBuildRequest{
			Id:              buildID,
			SummaryMarkdown: summary,
		}

		mbc.EXPECT().CancelBuild(ctx, req).Return(nil, nil)

		err := c.CancelBuild(ctx, buildID, summary)
		So(err, ShouldBeNil)
	})

	Convey(`Err`, t, func() {
		ctl := gomock.NewController(t)
		defer ctl.Finish()

		mbc := bpb.NewMockBuildsClient(ctl)
		c := NewBuildbucketClient(mbc)

		req := &bpb.CancelBuildRequest{
			Id:              buildID,
			SummaryMarkdown: summary,
		}

		resp := appstatus.BadRequest(errors.New("random error"))
		mbc.EXPECT().CancelBuild(ctx, req).Return(nil, resp)

		err := c.CancelBuild(ctx, buildID, summary)
		So(err, ShouldErrLike, "Failed to cancel build 12345")
	})
}

func TestGetBuildWithPatches(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	builder := "builder"
	commit := "12345"

	c1ps1 := createGerritChange(TestHost, ChromeProject, 54321, 1)

	expiredTime := timestamppb.New(time.Now().AddDate(0, -31, 0))
	patches := []*bpb.GerritChange{c1ps1}

	build1 := createBuild(1, bpb.Status_SUCCESS, expiredTime, DefaultBucket, builder, patches)
	build2 := createBuild(2, bpb.Status_STARTED, nil, DefaultBucket, builder, patches)

	Convey(`OK`, t, func() {
		Convey(`Match found`, func() {
			ctl := gomock.NewController(t)
			defer ctl.Finish()

			mbc := bpb.NewMockBuildsClient(ctl)
			c := NewBuildbucketClient(mbc)

			req := c.createSearchBuildRequest(builder, DefaultBucket, commit, patches)
			So(req.Predicate.Builder.Project, ShouldEqual, ChromeProject)

			// response in reverse chronological order.
			resp := &bpb.SearchBuildsResponse{
				Builds: []*bpb.Build{
					build2,
					build1,
				},
			}
			mbc.EXPECT().SearchBuilds(ctx, req).Return(resp, nil)

			build, err := c.GetBuildWithPatches(ctx, builder, DefaultBucket, commit, patches)
			So(err, ShouldBeNil)
			So(build.GetId(), ShouldEqual, 2)
			So(build.Input.GerritChanges, ShouldResembleProto, patches)
		})
	})

	Convey(`No Build Returned`, t, func() {
		Convey(`Old Build`, func() {
			ctl := gomock.NewController(t)
			defer ctl.Finish()

			mbc := bpb.NewMockBuildsClient(ctl)
			c := NewBuildbucketClient(mbc)

			req := c.createSearchBuildRequest(builder, DefaultBucket, commit, patches)
			resp := &bpb.SearchBuildsResponse{
				Builds: []*bpb.Build{
					build1,
				},
			}
			mbc.EXPECT().SearchBuilds(ctx, req).Return(resp, nil)

			build, err := c.GetBuildWithPatches(ctx, builder, DefaultBucket, commit, patches)
			So(err, ShouldBeNil)
			So(build, ShouldBeNil)
		})

		Convey(`Recent Failed Build`, func() {
			ctl := gomock.NewController(t)
			defer ctl.Finish()

			mbc := bpb.NewMockBuildsClient(ctl)
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
			So(err, ShouldBeNil)
			So(build, ShouldBeNil)
		})

		Convey(`Nil patches`, func() {
			ctl := gomock.NewController(t)
			defer ctl.Finish()

			mbc := bpb.NewMockBuildsClient(ctl)
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
			So(err, ShouldBeNil)
			So(build, ShouldBeNil)
		})
	})
}

func TestGetBuildWithDeps(t *testing.T) {
	ctx := context.Background()

	builder := "builder"
	commit := "12345"

	webrtc := "https://webrtc.googlesource.com/src"

	Convey(`Match using Deps`, t, func() {
		ctl := gomock.NewController(t)
		defer ctl.Finish()

		mbc := bpb.NewMockBuildsClient(ctl)
		c := NewBuildbucketClient(mbc)

		req := c.createSearchBuildRequest(builder, DefaultBucket, commit, nil)
		So(req.Predicate.Builder.Project, ShouldEqual, ChromeProject)

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

		deps := map[string]interface{}{
			webrtc: "1",
		}
		So(c.findMatchingBuild(resp.GetBuilds(), deps, nil), ShouldNotBeNil)

		build, err := c.GetBuildWithDeps(ctx, builder, DefaultBucket, commit, deps)
		So(err, ShouldBeNil)
		So(build.GetId(), ShouldEqual, 1)
		So(build.Input.GerritChanges, ShouldBeNil)
		So(build.Input.Properties.Fields[DepsOverrideKey].GetStructValue().AsMap()[webrtc].(string), ShouldEqual, "1")
	})
}

func TestGetBuildFromWaterfall(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	builder := "Linux Builder Perf"
	mirror, _ := PinpointWaterfall[builder]
	commit := "12345"

	// must be CI bucket, aka WaterfallBucket
	ts := timestamppb.New(time.Now().AddDate(0, 1, 0))
	build1 := createBuild(1, bpb.Status_SUCCESS, ts, WaterfallBucket, mirror, nil)
	build2 := createBuild(2, bpb.Status_STARTED, nil, WaterfallBucket, mirror, nil)

	Convey(`OK`, t, func() {
		Convey(`E2E`, func() {
			ctl := gomock.NewController(t)
			defer ctl.Finish()

			mbc := bpb.NewMockBuildsClient(ctl)
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
			So(err, ShouldBeNil)
			So(build.GetId(), ShouldEqual, 2)
		})
	})

	Convey(`Err`, t, func() {
		Convey(`Unsupported Builder`, func() {
			ctl := gomock.NewController(t)
			defer ctl.Finish()

			mbc := bpb.NewMockBuildsClient(ctl)
			c := NewBuildbucketClient(mbc)

			b, err := c.GetBuildFromWaterfall(ctx, "builder", commit)
			So(err, ShouldErrLike, "has no supported CI waterfall builder")
			So(b, ShouldBeNil)
		})
	})
}

func TestGetBuildStatus(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	buildID := int64(12345)

	Convey(`OK`, t, func() {
		ctl := gomock.NewController(t)
		defer ctl.Finish()

		mbc := bpb.NewMockBuildsClient(ctl)
		c := NewBuildbucketClient(mbc)

		req := &bpb.GetBuildStatusRequest{
			Id: buildID,
		}
		resp := createBuild(buildID, bpb.Status_STARTED, nil, "try", "builder", nil)
		mbc.EXPECT().GetBuildStatus(ctx, req).Return(resp, nil)

		status, err := c.GetBuildStatus(ctx, buildID)
		So(err, ShouldBeNil)
		So(status, ShouldEqual, bpb.Status_STARTED)
	})

	Convey(`Err`, t, func() {
		ctl := gomock.NewController(t)
		defer ctl.Finish()

		mbc := bpb.NewMockBuildsClient(ctl)
		c := NewBuildbucketClient(mbc)

		req := &bpb.GetBuildStatusRequest{
			Id: buildID,
		}
		resp := appstatus.BadRequest(errors.New("random error"))
		mbc.EXPECT().GetBuildStatus(ctx, req).Return(nil, resp)

		status, err := c.GetBuildStatus(ctx, buildID)
		So(err, ShouldErrLike, "random error")
		So(status, ShouldEqual, bpb.Status_STATUS_UNSPECIFIED)
	})
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

func TestGetCASReference(t *testing.T) {
	ctx := context.Background()

	buildID := int64(12345)
	target := "performance_test_suite"

	Convey(`OK`, t, func() {
		ctl := gomock.NewController(t)
		defer ctl.Finish()

		mbc := bpb.NewMockBuildsClient(ctl)
		c := NewBuildbucketClient(mbc)

		req := c.createCASReferenceRequest(buildID)
		So(req.Mask.Fields.Paths[0], ShouldEqual, "output.properties")

		resp := createCASResponse(buildID, bpb.Status_SUCCESS, target, "somehash/123")

		mbc.EXPECT().GetBuild(ctx, req).Return(resp, nil)

		ref, err := c.GetCASReference(ctx, buildID, target)
		So(err, ShouldBeNil)
		So(ref.CasInstance, ShouldEqual, DefaultCASInstance)
		So(ref.Digest.Hash, ShouldEqual, "somehash")
		So(ref.Digest.SizeBytes, ShouldEqual, 123)
	})

	Convey(`Err`, t, func() {
		Convey(`Non Successful Build`, func() {
			ctl := gomock.NewController(t)
			defer ctl.Finish()

			mbc := bpb.NewMockBuildsClient(ctl)
			c := NewBuildbucketClient(mbc)
			req := c.createCASReferenceRequest(buildID)
			resp := &bpb.Build{
				Id:     buildID,
				Status: bpb.Status_STARTED,
			}

			mbc.EXPECT().GetBuild(ctx, req).Return(resp, nil)

			ref, err := c.GetCASReference(ctx, buildID, target)
			So(ref, ShouldBeNil)
			So(err, ShouldErrLike, "Cannot fetch CAS information from build 12345 with status STARTED")
		})

		Convey(`Missing Target`, func() {
			ctl := gomock.NewController(t)
			defer ctl.Finish()

			mbc := bpb.NewMockBuildsClient(ctl)
			c := NewBuildbucketClient(mbc)

			req := c.createCASReferenceRequest(buildID)
			resp := createCASResponse(buildID, bpb.Status_SUCCESS, "other_test_suite", "somehash/123")

			mbc.EXPECT().GetBuild(ctx, req).Return(resp, nil)

			ref, err := c.GetCASReference(ctx, buildID, target)
			So(ref, ShouldBeNil)
			So(err, ShouldErrLike, "The target performance_test_suite cannot be found in the output properties")
		})

		Convey(`Wrong CAS Hash Format`, func() {
			ctl := gomock.NewController(t)
			defer ctl.Finish()

			mbc := bpb.NewMockBuildsClient(ctl)
			c := NewBuildbucketClient(mbc)

			req := c.createCASReferenceRequest(buildID)
			wrongHash := "somehash/123/456"
			resp := createCASResponse(buildID, bpb.Status_SUCCESS, target, wrongHash)

			mbc.EXPECT().GetBuild(ctx, req).Return(resp, nil)

			ref, err := c.GetCASReference(ctx, buildID, target)
			So(ref, ShouldBeNil)
			So(err, ShouldErrLike, fmt.Sprintf("CAS hash %s has been changed to an unparsable format", wrongHash))
		})
	})
}

func TestStartChromeBuild(t *testing.T) {
	ctx := context.Background()
	builder := "Linux Builder Perf"
	commit := "12345"

	Convey(`Schedule Chrome Build w/ DEPS`, t, func() {
		ctl := gomock.NewController(t)
		defer ctl.Finish()

		mbc := bpb.NewMockBuildsClient(ctl)
		c := NewBuildbucketClient(mbc)

		webrtc := "https://webrtc.googlesource.com/src"
		depsMap := map[string]interface{}{
			webrtc: "1",
		}

		req := c.createChromeBuildRequest("1", "1", builder, commit, depsMap, nil)

		// Checking defaults
		So(req.Builder.Project, ShouldEqual, ChromeProject)
		So(req.Builder.Bucket, ShouldEqual, DefaultBucket)
		So(req.GitilesCommit.Host, ShouldEqual, ChromiumGitilesHost)
		So(req.GitilesCommit.Project, ShouldEqual, ChromiumGitilesProject)
		So(req.GitilesCommit.Ref, ShouldEqual, ChromiumGitilesRefAtHead)

		resp := &bpb.Build{
			Id:     int64(12345),
			Status: bpb.Status_SCHEDULED,
		}
		mbc.EXPECT().ScheduleBuild(gomock.AssignableToTypeOf(ctx), req).Return(resp, nil)

		build, err := c.StartChromeBuild(ctx, "1", "1", builder, commit, depsMap, nil)
		So(err, ShouldBeNil)
		So(build.Status, ShouldEqual, bpb.Status_SCHEDULED)
	})

}
