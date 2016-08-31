// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package testclock

import (
	"time"

	"github.com/luci/luci-go/common/clock"
	"golang.org/x/net/context"
)

// timer is an implementation of clock.TestTimer that uses a channel
// to signal the timer to fire.
//
// The channel is buffered so it can be used without requiring a separate
// signalling goroutine.
type timer struct {
	ctx   context.Context
	clock *testClock

	// tags is the set of tags in the Context when this timer was created.
	tags []string

	// afterC will have the TimerResult of the timer's expiration written to it
	// when this timer triggers or is canceled.
	afterC chan clock.TimerResult

	// monitorFinishedC is used by our timer monitor to signal that the timer has
	// stopped.
	monitorFinishedC chan bool
}

var _ clock.Timer = (*timer)(nil)

// NewTimer returns a new, instantiated timer.
func newTimer(ctx context.Context, clk *testClock) *timer {
	return &timer{
		ctx:    ctx,
		clock:  clk,
		tags:   clock.Tags(ctx),
		afterC: make(chan clock.TimerResult, 1),
	}
}

func (t *timer) GetC() <-chan clock.TimerResult {
	return t.afterC
}

func (t *timer) Reset(d time.Duration) (active bool) {
	if d < 0 {
		d = 0
	}

	// Stop our current polling goroutine, if it's running.
	active = t.Stop()

	// Add ourself to our Clock's scheduler.
	triggerC := make(chan time.Time, 1)
	t.clock.addPendingTimer(t, d, triggerC)

	// Start a timer monitor goroutine.
	t.monitorFinishedC = make(chan bool, 1)
	go t.monitor(triggerC, t.monitorFinishedC)
	return
}

func (t *timer) Stop() (stopped bool) {
	// If the timer is not running, we're done.
	if t.monitorFinishedC == nil {
		return false
	}

	// Shutdown and wait for our monitoring goroutine.
	t.clock.clearPendingTimer(t)
	stopped = <-t.monitorFinishedC

	// Clear state.
	t.monitorFinishedC = nil
	return
}

// GetTags returns the tags associated with the specified timer. If the timer
// has no tags, an empty slice (nil) will be returned.
func GetTags(t clock.Timer) []string {
	if tt, ok := t.(*timer); ok {
		return clock.Tags(tt.ctx)
	}
	return nil
}

// HasTags tests if a given timer has the same tags.
func HasTags(t clock.Timer, first string, tags ...string) bool {
	timerTags := GetTags(t)
	if len(timerTags) == 0 {
		return false
	}

	if first != timerTags[0] {
		return false
	}

	if len(timerTags) > 1 {
		// Compare the remainder.
		timerTags = timerTags[1:]
		if len(timerTags) != len(tags) {
			return false
		}

		for i, tag := range timerTags {
			if tags[i] != tag {
				return false
			}
		}
	}
	return true
}

// monitor is run in a goroutine, monitoring the timer Context for
// cancellation and forcing a premature timer completion if the monitor
// was canceled.
func (t *timer) monitor(triggerC chan time.Time, finishedC chan bool) {
	monitorRunning := false
	defer func() {
		finishedC <- monitorRunning
		close(finishedC)
	}()

	select {
	case now, ok := <-triggerC:
		if !ok {
			// Our monitor goroutine has been reaped, probably because we were
			// stopped.
			monitorRunning = true
			break
		}

		// The Clock has signalled that this timer has expired.
		t.afterC <- clock.TimerResult{
			Time: now,
			Err:  nil,
		}

	case <-t.ctx.Done():
		// Our Context has been canceled.
		t.clock.clearPendingTimer(t)
		t.afterC <- clock.TimerResult{
			Time: t.clock.Now(),
			Err:  t.ctx.Err(),
		}
	}
}
