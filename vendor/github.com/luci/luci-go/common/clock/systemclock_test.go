// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package clock

import (
	"testing"
	"time"

	"golang.org/x/net/context"

	. "github.com/smartystreets/goconvey/convey"
)

// timeBase is the amount of time where short-response goroutine events "should
// happen". This isn't a great measure, since scheduling can take longer. Making
// this short will make the test run faster at the possible expense of increased
// raciness. Making this longer will increase test time, but will potentially
// reduce the change of race-related errors.
//
// This should be kept >60ms which is a fairly gratuitous RTC-based scheduler
// delay (1 hr / 2^16) that some older OSes may be subject to.
const timeBase = 60 * time.Millisecond

// veryLongTime is a time long enough that it won't feasably happen during the
// course of test execution.
const veryLongTime = 1000 * timeBase

// TestSystemClock tests the non-trivial system clock methods.
func TestSystemClock(t *testing.T) {
	t.Parallel()

	Convey(`A cancelable Context`, t, func() {
		c, cancelFunc := context.WithCancel(context.Background())
		sc := GetSystemClock()

		Convey(`Will perform a full sleep if the Context isn't canceled.`, func() {
			So(sc.Sleep(c, timeBase).Incomplete(), ShouldBeFalse)
		})

		Convey(`Will terminate the Sleep prematurely if the Context is canceled.`, func() {
			cancelFunc()
			So(sc.Sleep(c, veryLongTime).Incomplete(), ShouldBeTrue)
			So(sc.Sleep(c, veryLongTime).Err, ShouldEqual, context.Canceled)
		})
	})
}
