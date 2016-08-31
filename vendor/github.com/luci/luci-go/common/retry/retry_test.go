// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package retry

import (
	"errors"
	"testing"
	"time"

	"github.com/luci/luci-go/common/clock"
	"github.com/luci/luci-go/common/clock/testclock"
	. "github.com/smartystreets/goconvey/convey"
	"golang.org/x/net/context"
)

// testIterator is an Iterator implementation used for testing.
type testIterator struct {
	total int
	count int
}

func (i *testIterator) Next(_ context.Context, _ error) time.Duration {
	defer func() { i.count++ }()
	if i.count >= i.total {
		return Stop
	}
	return time.Second
}

func TestRetry(t *testing.T) {
	t.Parallel()

	// Generic test failure.
	failure := errors.New("retry: test error")

	Convey(`A testing function`, t, func() {
		ctx, c := testclock.UseTime(context.Background(), time.Date(2015, 1, 1, 0, 0, 0, 0, time.UTC))

		// Every time we sleep, update time by one second and count.
		sleeps := 0
		c.SetTimerCallback(func(time.Duration, clock.Timer) {
			c.Add(1 * time.Second)
			sleeps++
		})

		Convey(`A test Iterator with three retries`, func() {
			g := func() Iterator {
				return &testIterator{total: 3}
			}

			Convey(`Executes a successful function once.`, func() {
				var count, callbacks int
				err := Retry(ctx, g, func() error {
					count++
					return nil
				}, func(error, time.Duration) {
					callbacks++
				})
				So(err, ShouldBeNil)
				So(count, ShouldEqual, 1)
				So(callbacks, ShouldEqual, 0)
				So(sleeps, ShouldEqual, 0)
			})

			Convey(`Executes a failing function three times.`, func() {
				var count, callbacks int
				err := Retry(ctx, g, func() error {
					count++
					return failure
				}, func(error, time.Duration) {
					callbacks++
				})
				So(err, ShouldEqual, failure)
				So(count, ShouldEqual, 4)
				So(callbacks, ShouldEqual, 3)
				So(sleeps, ShouldEqual, 3)
			})

			Convey(`Executes a function that fails once, then succeeds once.`, func() {
				failure := errors.New("retry: test error")
				var count, callbacks int
				err := Retry(ctx, g, func() error {
					defer func() { count++ }()
					if count == 0 {
						return failure
					}
					return nil
				}, func(error, time.Duration) {
					callbacks++
				})
				So(err, ShouldEqual, nil)
				So(count, ShouldEqual, 2)
				So(callbacks, ShouldEqual, 1)
				So(sleeps, ShouldEqual, 1)
			})
		})

		Convey(`Does not retry if callback is not set.`, func() {
			So(Retry(ctx, nil, func() error {
				return failure
			}, nil), ShouldEqual, failure)
			So(sleeps, ShouldEqual, 0)
		})
	})
}
