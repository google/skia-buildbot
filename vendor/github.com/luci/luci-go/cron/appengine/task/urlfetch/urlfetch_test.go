// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package urlfetch

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"golang.org/x/net/context"

	"github.com/luci/gae/service/urlfetch"
	"github.com/luci/luci-go/common/clock"
	"github.com/luci/luci-go/common/clock/testclock"

	"github.com/luci/luci-go/cron/appengine/messages"
	"github.com/luci/luci-go/cron/appengine/task/utils/tasktest"

	. "github.com/luci/luci-go/common/testing/assertions"
	. "github.com/smartystreets/goconvey/convey"
)

func TestValidateProtoMessage(t *testing.T) {
	tm := TaskManager{}

	Convey("ValidateProtoMessage passes good msg", t, func() {
		So(tm.ValidateProtoMessage(&messages.UrlFetchTask{
			Url: strPtr("https://blah.com"),
		}), ShouldBeNil)
	})

	Convey("ValidateProtoMessage wrong type", t, func() {
		So(tm.ValidateProtoMessage(&messages.NoopTask{}), ShouldErrLike, "wrong type")
	})

	Convey("ValidateProtoMessage empty", t, func() {
		So(tm.ValidateProtoMessage(tm.ProtoMessageType()), ShouldErrLike, "field 'url' is required")
	})

	Convey("ValidateProtoMessage bad method", t, func() {
		So(tm.ValidateProtoMessage(&messages.UrlFetchTask{
			Method: strPtr("BLAH"),
		}), ShouldErrLike, "unsupported HTTP method")
	})

	Convey("ValidateProtoMessage no URL", t, func() {
		So(tm.ValidateProtoMessage(&messages.UrlFetchTask{}), ShouldErrLike, "field 'url' is required")
	})

	Convey("ValidateProtoMessage bad URL", t, func() {
		So(tm.ValidateProtoMessage(&messages.UrlFetchTask{
			Url: strPtr("%%%%"),
		}), ShouldErrLike, "invalid URL")
	})

	Convey("ValidateProtoMessage non-absolute URL", t, func() {
		So(tm.ValidateProtoMessage(&messages.UrlFetchTask{
			Url: strPtr("/abc"),
		}), ShouldErrLike, "not an absolute url")
	})

	Convey("ValidateProtoMessage small timeout", t, func() {
		So(tm.ValidateProtoMessage(&messages.UrlFetchTask{
			Url:        strPtr("https://blah.com"),
			TimeoutSec: intPtr(0),
		}), ShouldErrLike, "minimum allowed 'timeout_sec' is 1 sec")
	})

	Convey("ValidateProtoMessage large timeout", t, func() {
		So(tm.ValidateProtoMessage(&messages.UrlFetchTask{
			Url:        strPtr("https://blah.com"),
			TimeoutSec: intPtr(10000),
		}), ShouldErrLike, "maximum allowed 'timeout_sec' is 480 sec")
	})
}

func TestLaunchTask(t *testing.T) {
	tm := TaskManager{}

	Convey("LaunchTask works", t, func(c C) {
		ts, ctx := newTestContext(time.Unix(0, 1))
		defer ts.Close()
		ctl := &tasktest.TestController{
			TaskMessage: &messages.UrlFetchTask{
				Url: strPtr(ts.URL),
			},
			SaveCallback: func() error { return nil },
		}
		So(tm.LaunchTask(ctx, ctl), ShouldBeNil)
		So(ctl.Log[:2], ShouldResemble, []string{
			"GET " + ts.URL,
			"Finished with overall status SUCCEEDED in 0",
		})
	})
}

func strPtr(s string) *string {
	return &s
}

func intPtr(i int) *int32 {
	j := int32(i)
	return &j
}

func newTestContext(now time.Time) (*httptest.Server, context.Context) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Hello, Client!"))
	}))
	c := context.Background()
	c = clock.Set(c, testclock.New(now))
	c = urlfetch.Set(c, http.DefaultTransport)
	return ts, c
}
