// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package clock

import (
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"
	"golang.org/x/net/context"
)

// manualClock is a partial Clock implementation that allows us to release
// blocking calls.
type manualClock struct {
	Clock

	now             time.Time
	timeoutCallback func(time.Duration) bool
	testFinishedC   chan struct{}
}

func (mc *manualClock) Now() time.Time {
	return mc.now
}

func (mc *manualClock) After(ctx context.Context, d time.Duration) <-chan TimerResult {
	resultC := make(chan TimerResult)
	go func() {
		ar := TimerResult{}
		defer func() {
			resultC <- ar
		}()

		// If we are instructed to immediately timeout, do so.
		if cb := mc.timeoutCallback; cb != nil && cb(d) {
			return
		}

		select {
		case <-ctx.Done():
			ar.Err = ctx.Err()
		case <-mc.testFinishedC:
			break
		}
	}()

	return resultC
}

func wait(c context.Context) error {
	<-c.Done()
	return c.Err()
}

func TestClockContext(t *testing.T) {
	t.Parallel()

	Convey(`A manual testing clock`, t, func() {
		mc := manualClock{
			now:           time.Date(2016, 1, 1, 0, 0, 0, 0, time.Local),
			testFinishedC: make(chan struct{}),
		}
		defer close(mc.testFinishedC)

		Convey(`A context with a deadline wrapping a cancellable parent`, func() {
			Convey(`Successfully reports its deadline.`, func() {
				cctx, _ := context.WithCancel(Set(context.Background(), &mc))
				ctx, _ := WithTimeout(cctx, 10*time.Millisecond)

				deadline, ok := ctx.Deadline()
				So(ok, ShouldBeTrue)
				So(deadline.After(mc.now), ShouldBeTrue)
			})

			Convey(`Will successfully time out.`, func() {
				mc.timeoutCallback = func(time.Duration) bool {
					return true
				}

				cctx, _ := context.WithCancel(Set(context.Background(), &mc))
				ctx, _ := WithTimeout(cctx, 10*time.Millisecond)
				So(wait(ctx), ShouldEqual, context.DeadlineExceeded)
			})

			Convey(`Will successfully cancel with its cancel func.`, func() {
				cctx, _ := context.WithCancel(Set(context.Background(), &mc))
				ctx, cf := WithTimeout(cctx, 10*time.Millisecond)

				go func() {
					cf()
				}()
				So(wait(ctx), ShouldEqual, context.Canceled)
			})

			Convey(`Will successfully cancel if the parent is canceled.`, func() {
				cctx, pcf := context.WithCancel(Set(context.Background(), &mc))
				ctx, _ := WithTimeout(cctx, 10*time.Millisecond)

				go func() {
					pcf()
				}()
				So(wait(ctx), ShouldEqual, context.Canceled)
			})
		})

		Convey(`A context with a deadline wrapping a parent with a shorter deadline`, func() {
			cctx, _ := context.WithTimeout(context.Background(), 10*time.Millisecond)
			ctx, cf := WithTimeout(cctx, 1*time.Hour)

			Convey(`Will successfully time out.`, func() {
				mc.timeoutCallback = func(d time.Duration) bool {
					return d == 10*time.Millisecond
				}

				So(wait(ctx), ShouldEqual, context.DeadlineExceeded)
			})

			Convey(`Will successfully cancel with its cancel func.`, func() {
				go func() {
					cf()
				}()
				So(wait(ctx), ShouldEqual, context.Canceled)
			})
		})

		Convey(`A context with a deadline in the past`, func() {
			ctx, _ := WithDeadline(context.Background(), mc.now.Add(-time.Second))

			Convey(`Will time out immediately.`, func() {
				So(wait(ctx), ShouldEqual, context.DeadlineExceeded)
			})
		})
	})
}
