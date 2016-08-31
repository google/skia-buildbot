// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package testclock

import (
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/luci/luci-go/common/clock"
	"golang.org/x/net/context"

	. "github.com/smartystreets/goconvey/convey"
)

// trashTimer is a useless implementation of clock.Timer specifically designed
// to exist and not be a test timer type.
type trashTimer struct {
	clock.Timer
}

func TestTestTimer(t *testing.T) {
	t.Parallel()

	Convey(`A testing clock instance`, t, func() {
		ctx, cancelFunc := context.WithCancel(context.Background())
		defer cancelFunc()

		now := TestTimeLocal
		clk := New(now)

		Convey(`A timer instance`, func() {
			t := clk.NewTimer(ctx)

			Convey(`Should have a non-nil C.`, func() {
				So(t.GetC(), ShouldNotBeNil)
			})

			Convey(`When activated`, func() {
				So(t.Reset(1*time.Second), ShouldBeFalse)

				Convey(`When reset, should return active.`, func() {
					So(t.Reset(1*time.Hour), ShouldBeTrue)
					So(t.GetC(), ShouldNotBeNil)
				})

				Convey(`When stopped, should return active.`, func() {
					So(t.Stop(), ShouldBeTrue)
					So(t.GetC(), ShouldNotBeNil)

					Convey(`And when stopped again, should return inactive.`, func() {
						So(t.Stop(), ShouldBeFalse)
						So(t.GetC(), ShouldNotBeNil)
					})
				})

				Convey(`When stopped after expiring, will return false.`, func() {
					clk.Add(1 * time.Second)
					So(t.Stop(), ShouldBeFalse)

					var signalled bool
					select {
					case <-t.GetC():
						signalled = true
					default:
						break
					}
					So(signalled, ShouldBeTrue)
				})
			})

			Convey(`Should successfully signal.`, func() {
				So(t.Reset(1*time.Second), ShouldBeFalse)
				clk.Add(1 * time.Second)

				So(<-t.GetC(), ShouldResemble, clock.TimerResult{Time: now.Add(1 * time.Second)})
			})

			Convey(`Should signal immediately if the timer is in the past.`, func() {
				So(t.Reset(-1*time.Second), ShouldBeFalse)
				So(<-t.GetC(), ShouldResemble, clock.TimerResult{Time: now})
			})

			Convey(`Will trigger immediately if the Context is canceled when reset.`, func() {
				cancelFunc()

				// Works for the first timer?
				So(t.Reset(time.Hour), ShouldBeFalse)
				So((<-t.GetC()).Err, ShouldEqual, context.Canceled)

				// Works for the second timer?
				So(t.Reset(time.Hour), ShouldBeFalse)
				So((<-t.GetC()).Err, ShouldEqual, context.Canceled)
			})

			Convey(`Will trigger when the Context is canceled.`, func() {
				clk.SetTimerCallback(func(time.Duration, clock.Timer) {
					cancelFunc()
				})

				// Works for the first timer?
				So(t.Reset(time.Hour), ShouldBeFalse)
				So((<-t.GetC()).Err, ShouldEqual, context.Canceled)

				// Works for the second timer?
				So(t.Reset(time.Hour), ShouldBeFalse)
				So((<-t.GetC()).Err, ShouldEqual, context.Canceled)
			})

			Convey(`Will use the same channel when reset.`, func() {
				timerC := t.GetC()
				t.Reset(time.Second)
				clk.Add(time.Second)
				So(<-timerC, ShouldResemble, clock.TimerResult{Time: now.Add(1 * time.Second)})
			})

			Convey(`Will not signal the timer channel if stopped.`, func() {
				t.Reset(time.Second)
				So(t.Stop(), ShouldBeTrue)
				clk.Add(time.Second)

				triggered := false
				select {
				case <-t.GetC():
					triggered = true
				default:
					break
				}
				So(triggered, ShouldBeFalse)
			})

			Convey(`Will not trigger on previous time thresholds if reset.`, func() {
				t.Reset(time.Second)
				t.Reset(2 * time.Second)
				clk.Add(time.Second)
				clk.Add(time.Second)
				So((<-t.GetC()).Time, ShouldResemble, clk.Now())
			})

			Convey(`Can set and retrieve timer tags.`, func() {
				var tagMu sync.Mutex
				tagMap := map[string]struct{}{}

				// On the last timer callback, advance time past the timer threshold.
				timers := make([]clock.Timer, 10)
				count := 0
				clk.SetTimerCallback(func(_ time.Duration, t clock.Timer) {
					tagMu.Lock()
					defer tagMu.Unlock()

					tagMap[strings.Join(GetTags(t), "")] = struct{}{}
					count++
					if count == len(timers) {
						clk.Add(time.Second)
					}
				})

				for i := range timers {
					t := clk.NewTimer(clock.Tag(ctx, fmt.Sprintf("%d", i)))
					t.Reset(time.Second)
					timers[i] = t
				}

				// Wait until all timers have expired.
				for _, t := range timers {
					<-t.GetC()
				}

				// Did we see all of their tags?
				for i := range timers {
					_, ok := tagMap[fmt.Sprintf("%d", i)]
					So(ok, ShouldBeTrue)
				}
			})

			Convey(`A non-test timer has a nil tag.`, func() {
				t := trashTimer{}
				So(GetTags(t), ShouldBeNil)
			})
		})

		Convey(`Multiple goroutines using timers...`, func() {
			// Mark when timers are started, so we can ensure that our signalling
			// happens after the timers have been instantiated.
			timerStartedC := make(chan bool)
			clk.SetTimerCallback(func(_ time.Duration, _ clock.Timer) {
				timerStartedC <- true
			})

			resultC := make(chan clock.TimerResult)
			for i := time.Duration(0); i < 5; i++ {
				go func(d time.Duration) {
					timer := clk.NewTimer(ctx)
					timer.Reset(d)
					resultC <- <-timer.GetC()
				}(i * time.Second)
				<-timerStartedC
			}

			moreResults := func() bool {
				select {
				case <-resultC:
					return true
				default:
					return false
				}
			}

			// Advance the clock to +2s. Three timers should signal.
			clk.Set(now.Add(2 * time.Second))
			<-resultC
			<-resultC
			<-resultC
			So(moreResults(), ShouldBeFalse)

			// Advance clock to +3s. One timer should signal.
			clk.Set(now.Add(3 * time.Second))
			<-resultC
			So(moreResults(), ShouldBeFalse)

			// Advance clock to +10s. One final timer should signal.
			clk.Set(now.Add(10 * time.Second))
			<-resultC
			So(moreResults(), ShouldBeFalse)
		})
	})
}

func TestTimerTags(t *testing.T) {
	t.Parallel()

	Convey(`A context with tags {"A", "B"}`, t, func() {
		c := clock.Tag(clock.Tag(context.Background(), "A"), "B")

		Convey(`Has tags, {"A", "B"}`, func() {
			So(clock.Tags(c), ShouldResemble, []string{"A", "B"})
		})

		Convey(`Will be retained by a testclock.Timer.`, func() {
			tc := New(TestTimeUTC)
			t := tc.NewTimer(c)

			So(GetTags(t), ShouldResemble, []string{"A", "B"})

			Convey(`The timer tests positive for tags {"A", "B"}`, func() {
				So(HasTags(t, "A", "B"), ShouldBeTrue)
			})

			Convey(`The timer tests negative for tags {"A"}, {"B"}, and {"A", "C"}`, func() {
				So(HasTags(t, "A"), ShouldBeFalse)
				So(HasTags(t, "B"), ShouldBeFalse)
				So(HasTags(t, "A", "C"), ShouldBeFalse)
			})
		})

		Convey(`A non-test timer tests negative for {"A"} and {"A", "B"}.`, func() {
			t := &trashTimer{}

			So(HasTags(t, "A"), ShouldBeFalse)
			So(HasTags(t, "A", "B"), ShouldBeFalse)
		})
	})
}
