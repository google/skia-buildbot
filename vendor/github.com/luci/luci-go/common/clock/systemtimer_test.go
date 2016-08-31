// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package clock

import (
	"testing"
	"time"

	"golang.org/x/net/context"

	. "github.com/smartystreets/goconvey/convey"
)

func TestSystemTimer(t *testing.T) {
	t.Parallel()

	Convey(`A systemTimer instance`, t, func() {
		ctx, cancelFunc := context.WithCancel(context.Background())
		t := GetSystemClock().NewTimer(ctx)
		defer t.Stop()

		Convey(`Should start with a non-nil channel.`, func() {
			So(t.GetC(), ShouldNotBeNil)
		})

		Convey(`When stopped, should return inactive.`, func() {
			So(t.Stop(), ShouldBeFalse)
		})

		Convey(`Will return immediately if the Context is canceled before Reset.`, func() {
			cancelFunc()

			t.Reset(veryLongTime)
			So((<-t.GetC()).Err, ShouldEqual, context.Canceled)
		})

		Convey(`Will return if the Context is canceled after Reset.`, func() {
			t.Reset(veryLongTime)
			cancelFunc()

			So((<-t.GetC()).Err, ShouldEqual, context.Canceled)
		})

		Convey(`A timer will use the same channel when Reset.`, func() {
			timerC := t.GetC()

			// Reset our timer to something more reasonable. It should trigger the
			// timer channel, which should trigger our
			t.Reset(timeBase)
			So((<-timerC).Err, ShouldBeNil)
		})

		Convey(`A timer will not signal if stopped.`, func() {
			t.Reset(timeBase)
			t.Stop()

			// This isn't a perfect test, but it's a good boundary for flake.
			time.Sleep(3 * timeBase)

			triggered := false
			select {
			case <-t.GetC():
				triggered = true
			default:
				break
			}
			So(triggered, ShouldBeFalse)
		})

		Convey(`When reset`, func() {
			So(t.Reset(veryLongTime), ShouldBeFalse)

			Convey(`When reset again to a short duration, should return that it was active and trigger.`, func() {
				// Upper bound of supported platform resolution. Windows is 15ms, so
				// make sure we exceed that.
				So(t.Reset(timeBase), ShouldBeTrue)
				So((<-t.GetC()).IsZero(), ShouldBeFalse)

				// Again (reschedule).
				So(t.Reset(timeBase), ShouldBeFalse)
				So((<-t.GetC()).IsZero(), ShouldBeFalse)
			})

			Convey(`When stopped, should return active and have a non-nil C.`, func() {
				active := t.Stop()
				So(active, ShouldBeTrue)
				So(t.GetC(), ShouldNotBeNil)
			})

			Convey(`When stopped, should return active.`, func() {
				active := t.Stop()
				So(active, ShouldBeTrue)
			})

			Convey(`Should have a non-nil channel.`, func() {
				So(t.GetC(), ShouldNotBeNil)
			})
		})
	})
}
