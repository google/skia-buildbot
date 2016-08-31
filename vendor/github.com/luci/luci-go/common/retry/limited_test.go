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

func TestLimited(t *testing.T) {
	Convey(`A Limited Iterator, using an instrumented context`, t, func() {
		ctx, clock := testclock.UseTime(context.Background(), time.Date(2015, 1, 1, 0, 0, 0, 0, time.UTC))
		l := Limited{}

		Convey(`When empty, will return Stop immediately..`, func() {
			So(l.Next(ctx, nil), ShouldEqual, Stop)
		})

		Convey(`With 3 retries, will Stop after three retries.`, func() {
			l.Delay = time.Second
			l.Retries = 3

			So(l.Next(ctx, nil), ShouldEqual, time.Second)
			So(l.Next(ctx, nil), ShouldEqual, time.Second)
			So(l.Next(ctx, nil), ShouldEqual, time.Second)
			So(l.Next(ctx, nil), ShouldEqual, Stop)
		})

		Convey(`Will stop after MaxTotal.`, func() {
			l.Retries = 1000
			l.Delay = 3 * time.Second
			l.MaxTotal = 8 * time.Second

			So(l.Next(ctx, nil), ShouldEqual, 3*time.Second)
			clock.Add(8 * time.Second)
			So(l.Next(ctx, nil), ShouldEqual, Stop)
		})
	})
}
