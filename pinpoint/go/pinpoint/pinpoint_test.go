package pinpoint

import (
	"context"
	"fmt"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/stretchr/testify/mock"
	. "go.chromium.org/luci/common/testing/assertions"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/pinpoint/go/build_chrome/mocks"
	"go.skia.org/infra/pinpoint/go/midpoint"
	"go.skia.org/infra/pinpoint/go/read_values"

	bpb "go.chromium.org/luci/buildbucket/proto"
)

func defaultRunRequest() PinpointRunRequest {
	return PinpointRunRequest{
		Device:            "linux-perf",
		Benchmark:         "benchmark",
		Story:             "story",
		Chart:             "chart",
		StartCommit:       "start_commit",
		EndCommit:         "end_commit",
		AggregationMethod: read_values.Sum.AggDataMethod(),
	}
}

func TestValidateRunRequest(t *testing.T) {
	Convey(`OK`, t, func() {
		ctx := context.Background()
		pp, err := New(ctx)
		So(err, ShouldBeNil)
		Convey(`Given valid inputs`, func() {
			req := defaultRunRequest()
			// Run() would result in an infinite loop
			// as shouldContinue would never return false
			err := pp.validateRunRequest(req)
			So(err, ShouldBeNil)
		})
	})

	Convey(`Fails input validation`, t, func() {
		ctx := context.Background()
		pp, err := New(ctx)
		So(err, ShouldBeNil)
		Convey(`When no base or exp hash`, func() {
			req := defaultRunRequest()
			req.StartCommit = ""
			resp, err := pp.Run(ctx, req, "")
			So(resp, ShouldBeNil)
			So(err, ShouldErrLike, fmt.Sprintf(missingRequiredParamTemplate, "start"))

			req = defaultRunRequest()
			req.EndCommit = ""
			resp, err = pp.Run(ctx, req, "")
			So(resp, ShouldBeNil)
			So(err, ShouldErrLike, fmt.Sprintf(missingRequiredParamTemplate, "end"))
		})
		Convey(`When bad device in request`, func() {
			req := defaultRunRequest()
			req.Device = "fake-device"
			resp, err := pp.Run(ctx, req, "")
			So(resp, ShouldBeNil)
			So(err, ShouldErrLike, fmt.Sprintf("Device %s not allowed", req.Device))
		})
		Convey(`When missing benchmark`, func() {
			req := defaultRunRequest()
			req.Benchmark = ""
			resp, err := pp.Run(ctx, req, "")
			So(resp, ShouldBeNil)
			So(err, ShouldErrLike, fmt.Sprintf(missingRequiredParamTemplate, "benchmark"))
		})
		Convey(`When missing story`, func() {
			req := defaultRunRequest()
			req.Story = ""
			resp, err := pp.Run(ctx, req, "")
			So(resp, ShouldBeNil)
			So(err, ShouldErrLike, fmt.Sprintf(missingRequiredParamTemplate, "story"))
		})
		Convey(`When missing chart`, func() {
			req := defaultRunRequest()
			req.Chart = ""
			resp, err := pp.Run(ctx, req, "")
			So(resp, ShouldBeNil)
			So(err, ShouldErrLike, fmt.Sprintf(missingRequiredParamTemplate, "chart"))
		})
	})
}

func TestShouldContinue(t *testing.T) {
	Convey(`Return True`, t, func() {
		Convey(`When no builds started`, func() {
			cdl := commitDataList{
				{
					commit: &midpoint.Commit{
						GitHash:       "start commit",
						RepositoryUrl: chromiumSrcGit,
					},
				},
				{
					commit: &midpoint.Commit{
						GitHash:       "end commit",
						RepositoryUrl: chromiumSrcGit,
					},
				},
			}
			cont := cdl.shouldContinue()
			So(cont, ShouldEqual, true)
		})
		Convey(`When build status is not finished`, func() {
			cdl := commitDataList{
				{
					build: &buildMetadata{
						buildStatus: bpb.Status_SUCCESS,
					},
				},
				{
					build: &buildMetadata{
						buildStatus: bpb.Status_STARTED,
					},
				},
			}
			cont := cdl.shouldContinue()
			So(cont, ShouldEqual, true)
		})
	})

	Convey(`Return False when builds are completed`, t, func() {
		cdl := commitDataList{
			{
				build: &buildMetadata{
					buildStatus: bpb.Status_CANCELED,
				},
			},
			{
				build: &buildMetadata{
					buildStatus: bpb.Status_FAILURE,
				},
			},
		}
		cont := cdl.shouldContinue()
		So(cont, ShouldEqual, false)
	})
}

func TestPollBuild(t *testing.T) {
	Convey(`OK`, t, func() {
		ctx := context.Background()
		mbc := mocks.NewBuildChromeClient(t)
		Convey(`When one build finishes`, func() {
			cdl := commitDataList{
				{
					build: &buildMetadata{
						buildID: 1,
					},
				},
				{
					build: &buildMetadata{
						buildID: 2,
					},
				},
			}
			mbc.On("GetStatus", ctx, mock.Anything).Return(bpb.Status_STARTED, nil).Once()
			mbc.On("GetStatus", ctx, mock.Anything).Return(bpb.Status_FAILURE, nil).Once()
			i, err := cdl.pollBuild(ctx, mbc)
			So(*i, ShouldEqual, 1)
			So(err, ShouldBeNil)
			So(cdl[0].build.buildStatus, ShouldEqual, bpb.Status_STARTED)
			So(cdl[1].build.buildStatus, ShouldEqual, bpb.Status_FAILURE)
		})
		Convey(`When no builds are finished`, func() {
			cdl := commitDataList{
				{
					build: &buildMetadata{
						buildID:     1,
						buildStatus: bpb.Status_SCHEDULED,
					},
				},
				{
					build: &buildMetadata{
						buildID:     2,
						buildStatus: bpb.Status_STARTED,
					},
				},
			}
			mbc.On("GetStatus", ctx, mock.Anything).Return(bpb.Status_SCHEDULED, nil).Once()
			mbc.On("GetStatus", ctx, mock.Anything).Return(bpb.Status_STARTED, nil).Once()
			i, err := cdl.pollBuild(ctx, mbc)
			So(i, ShouldBeNil)
			So(err, ShouldBeNil)
		})
		Convey(`When builds are already finished`, func() {
			cdl := commitDataList{
				{
					build: &buildMetadata{
						buildID:     1,
						buildStatus: bpb.Status_FAILURE,
					},
				},
				{
					build: &buildMetadata{
						buildID:     2,
						buildStatus: bpb.Status_CANCELED,
					},
				},
			}
			mbc.On("GetStatus", ctx, mock.Anything).Return(bpb.Status_FAILURE, nil).Once()
			mbc.On("GetStatus", ctx, mock.Anything).Return(bpb.Status_CANCELED, nil).Once()
			i, err := cdl.pollBuild(ctx, mbc)
			So(i, ShouldBeNil)
			So(err, ShouldBeNil)
		})

	})
	Convey(`Error`, t, func() {
		Convey(`When no valid builds to poll`, func() {
			ctx := context.Background()
			mbc := mocks.NewBuildChromeClient(t)
			cdl := commitDataList{
				{
					build: &buildMetadata{
						buildID: 1,
					},
				},
				{},
			}
			mbc.On("GetStatus", ctx, mock.Anything).Return(bpb.Status_STARTED, nil).Once()
			i, err := cdl.pollBuild(ctx, mbc)
			So(i, ShouldBeNil)
			So(err, ShouldErrLike, "Cannot poll build of non-existent build")
		})
		Convey(`When client fails GetStatus`, func() {
			ctx := context.Background()
			mbc := mocks.NewBuildChromeClient(t)
			cdl := commitDataList{
				{
					build: &buildMetadata{
						buildID: 1,
					},
				},
				{
					build: &buildMetadata{
						buildID: 2,
					},
				},
			}
			mbc.On("GetStatus", ctx, mock.Anything).Return(bpb.Status_STATUS_UNSPECIFIED, skerr.Fmt("some error")).Once()
			i, err := cdl.pollBuild(ctx, mbc)
			So(i, ShouldBeNil)
			So(err, ShouldErrLike, "Could not get build status")
		})
	})
}
