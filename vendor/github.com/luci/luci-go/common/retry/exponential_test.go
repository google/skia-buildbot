// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package retry

import (
	"testing"
	"time"

	"github.com/luci/luci-go/common/clock/testclock"
	. "github.com/smartystreets/goconvey/convey"
	"golang.org/x/net/context"
)

func TestExponentialBackoff(t *testing.T) {
	Convey(`An ExponentialBackoff Iterator, using an instrumented context`, t, func() {
		ctx, _ := testclock.UseTime(context.Background(), time.Date(2015, 1, 1, 0, 0, 0, 0, time.UTC))
		l := ExponentialBackoff{}

		Convey(`When empty, will Stop immediatley.`, func() {
			So(l.Next(ctx, nil), ShouldEqual, Stop)
		})

		Convey(`Will delay exponentially.`, func() {
			l.Retries = 4
			l.Delay = time.Second
			So(l.Next(ctx, nil), ShouldEqual, 1*time.Second)
			So(l.Next(ctx, nil), ShouldEqual, 2*time.Second)
			So(l.Next(ctx, nil), ShouldEqual, 4*time.Second)
			So(l.Next(ctx, nil), ShouldEqual, 8*time.Second)
			So(l.Next(ctx, nil), ShouldEqual, Stop)
		})

		Convey(`Will bound exponential delay when MaxDelay is set.`, func() {
			l.Retries = 4
			l.Delay = time.Second
			l.MaxDelay = 4 * time.Second
			So(l.Next(ctx, nil), ShouldEqual, 1*time.Second)
			So(l.Next(ctx, nil), ShouldEqual, 2*time.Second)
			So(l.Next(ctx, nil), ShouldEqual, 4*time.Second)
			So(l.Next(ctx, nil), ShouldEqual, 4*time.Second)
			So(l.Next(ctx, nil), ShouldEqual, Stop)
		})
	})
}
