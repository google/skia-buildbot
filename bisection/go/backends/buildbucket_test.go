package backends

import (
	"context"
	"testing"
	"time"

	"github.com/golang/mock/gomock"

	"go.skia.org/infra/bisection/go/build_chrome"
	"go.skia.org/infra/go/buildbucket"

	. "github.com/smartystreets/goconvey/convey"
	bpb "go.chromium.org/luci/buildbucket/proto"
	. "go.chromium.org/luci/common/testing/assertions"
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
		Convey(`E2E`, func() {
			ctl := gomock.NewController(t)
			defer ctl.Finish()

			mbc := bpb.NewMockBuildsClient(ctl)
			c := NewBuildbucketClient(mbc)

			req := c.createSearchBuildRequest(builder, DefaultBucket, commit, patches)
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
	})
}

func TestGetBuildFromWaterfall(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	builder := "Linux Builder Perf"
	mirror, _ := build_chrome.PinpointWaterfall[builder]
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
