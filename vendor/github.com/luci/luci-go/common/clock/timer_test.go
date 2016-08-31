// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package clock

import (
	"errors"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"golang.org/x/net/context"
)

func TestTimerResult(t *testing.T) {
	t.Parallel()

	Convey(`Testing TimerResult`, t, func() {
		Convey(`A TimerResult with no error is not incomplete.`, func() {
			So(TimerResult{}.Incomplete(), ShouldBeFalse)
		})

		Convey(`A TimerResult with context.Canceled or context.DeadlineExceeded is incomplete.`, func() {
			So(TimerResult{Err: context.Canceled}.Incomplete(), ShouldBeTrue)
			So(TimerResult{Err: context.DeadlineExceeded}.Incomplete(), ShouldBeTrue)
		})

		Convey(`A TimerResult with an unknown error will panic during Incomplete().`, func() {
			So(func() {
				TimerResult{Err: errors.New("test error")}.Incomplete()
			}, ShouldPanic)
		})
	})
}
