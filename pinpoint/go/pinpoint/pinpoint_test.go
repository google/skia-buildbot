package pinpoint

import (
	"context"
	"fmt"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	. "go.chromium.org/luci/common/testing/assertions"
	"go.skia.org/infra/pinpoint/go/read_values"
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
			resp, err := pp.Schedule(ctx, req, "")
			So(resp, ShouldNotBeNil)
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
			resp, err := pp.Schedule(ctx, req, "")
			So(resp, ShouldBeNil)
			So(err, ShouldErrLike, fmt.Sprintf(missingRequiredParamTemplate, "start"))

			req = defaultRunRequest()
			req.EndCommit = ""
			resp, err = pp.Schedule(ctx, req, "")
			So(resp, ShouldBeNil)
			So(err, ShouldErrLike, fmt.Sprintf(missingRequiredParamTemplate, "end"))
		})
		Convey(`When bad device in request`, func() {
			req := defaultRunRequest()
			req.Device = "fake-device"
			resp, err := pp.Schedule(ctx, req, "")
			So(resp, ShouldBeNil)
			So(err, ShouldErrLike, fmt.Sprintf("Device %s not allowed", req.Device))
		})
		Convey(`When missing benchmark`, func() {
			req := defaultRunRequest()
			req.Benchmark = ""
			resp, err := pp.Schedule(ctx, req, "")
			So(resp, ShouldBeNil)
			So(err, ShouldErrLike, fmt.Sprintf(missingRequiredParamTemplate, "benchmark"))
		})
		Convey(`When missing story`, func() {
			req := defaultRunRequest()
			req.Story = ""
			resp, err := pp.Schedule(ctx, req, "")
			So(resp, ShouldBeNil)
			So(err, ShouldErrLike, fmt.Sprintf(missingRequiredParamTemplate, "story"))
		})
		Convey(`When missing chart`, func() {
			req := defaultRunRequest()
			req.Chart = ""
			resp, err := pp.Schedule(ctx, req, "")
			So(resp, ShouldBeNil)
			So(err, ShouldErrLike, fmt.Sprintf(missingRequiredParamTemplate, "chart"))
		})
	})
}
