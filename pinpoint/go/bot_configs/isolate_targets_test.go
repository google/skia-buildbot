package bot_configs

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	. "go.chromium.org/luci/common/testing/assertions"
)

func TestGetIsolateTarget(t *testing.T) {
	Convey(`OK`, t, func() {
		Convey(`With configuration defined bot`, func() {
			target, err := GetIsolateTarget("android-go-perf", "benchmark")
			So(target, ShouldEqual, "performance_test_suite_android_clank_monochrome")
			So(err, ShouldBeNil)
		})
		Convey(`With regex matching`, func() {
			target, err := GetIsolateTarget("android-go_webview-perf", "benchmark")
			So(target, ShouldEqual, "performance_webview_test_suite")
			So(err, ShouldBeNil)
		})
		Convey(`With configuration unlisted bot`, func() {
			target, err := GetIsolateTarget("linux-perf", "benchmark")
			So(target, ShouldEqual, "performance_test_suite")
			So(err, ShouldBeNil)
		})
		Convey(`With webrtc benchmark`, func() {
			target, err := GetIsolateTarget("linux-perf", "webrtc_perf_tests")
			So(target, ShouldEqual, "webrtc_perf_tests")
			So(err, ShouldBeNil)
		})
	})
	Convey(`Error with bot not listed in bot_configs`, t, func() {
		target, err := GetIsolateTarget("fake device", "benchmark")
		So(target, ShouldBeBlank)
		So(err, ShouldErrLike, "Cannot get isolate target of bot")
	})
}
