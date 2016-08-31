// Copyright 2014 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package testclock

import (
	"container/heap"
	"sync"
	"time"

	"github.com/luci/luci-go/common/clock"
	"golang.org/x/net/context"
)

// TestClock is a Clock interface with additional methods to help instrument it.
type TestClock interface {
	clock.Clock

	// Set sets the test clock's time.
	Set(time.Time)

	// Add advances the test clock's time.
	Add(time.Duration)

	// SetTimerCallback is a goroutine-safe method to set an instance-wide
	// callback that is invoked when any timer begins.
	SetTimerCallback(TimerCallback)
}

// TimerCallback that can be invoked when a timer has been set. This is useful
// for sychronizing state when testing.
type TimerCallback func(time.Duration, clock.Timer)

// testClock is a test-oriented implementation of the 'Clock' interface.
//
// This implementation's Clock responses are configurable by modifying its
// member variables. Time-based events are explicitly triggered by sending on a
// Timer instance's channel.
type testClock struct {
	sync.Mutex

	now time.Time // The current clock time.

	timerCallback TimerCallback // Optional callback when a timer has been set.
	pendingTimers pendingTimerHeap
}

var _ TestClock = (*testClock)(nil)

// New returns a TestClock instance set at the specified time.
func New(now time.Time) TestClock {
	return &testClock{
		now: now,
	}
}

func (c *testClock) Now() time.Time {
	c.Lock()
	defer c.Unlock()

	return c.now
}

func (c *testClock) Sleep(ctx context.Context, d time.Duration) clock.TimerResult {
	return <-c.After(ctx, d)
}

func (c *testClock) NewTimer(ctx context.Context) clock.Timer {
	t := newTimer(ctx, c)
	return t
}

func (c *testClock) After(ctx context.Context, d time.Duration) <-chan clock.TimerResult {
	t := newTimer(ctx, c)
	t.Reset(d)
	return t.afterC
}

func (c *testClock) Set(t time.Time) {
	c.Lock()
	defer c.Unlock()

	c.setTimeLocked(t)
}

func (c *testClock) Add(d time.Duration) {
	c.Lock()
	defer c.Unlock()

	c.setTimeLocked(c.now.Add(d))
}

func (c *testClock) setTimeLocked(t time.Time) {
	if t.Before(c.now) {
		panic("Cannot go backwards in time. You're not Doc Brown.")
	}
	c.now = t

	// Unblock any blocking timers that are waiting on our lock.
	c.triggerTimersLocked()
}

func (c *testClock) triggerTimersLocked() {
	for len(c.pendingTimers) > 0 {
		e := c.pendingTimers[0]
		if c.now.Before(e.deadline) {
			break
		}

		heap.Pop(&c.pendingTimers)
		e.triggerC <- c.now
		close(e.triggerC)
	}
}

func (c *testClock) addPendingTimer(t *timer, d time.Duration, triggerC chan<- time.Time) {
	deadline := c.Now().Add(d)
	if callback := c.timerCallback; callback != nil {
		callback(d, t)
	}

	c.Lock()
	defer c.Unlock()

	heap.Push(&c.pendingTimers, &pendingTimer{
		timer:    t,
		deadline: deadline,
		triggerC: triggerC,
	})
	c.triggerTimersLocked()
}

func (c *testClock) clearPendingTimer(t *timer) {
	c.Lock()
	defer c.Unlock()

	for i := 0; i < len(c.pendingTimers); {
		if e := c.pendingTimers[0]; e.timer == t {
			heap.Remove(&c.pendingTimers, i)
			close(e.triggerC)
		} else {
			i++
		}
	}
}

func (c *testClock) SetTimerCallback(callback TimerCallback) {
	c.Lock()
	defer c.Unlock()

	c.timerCallback = callback
}

// pendingTimer is a single registered timer instance along with its trigger
// deadline.
type pendingTimer struct {
	*timer

	deadline time.Time
	triggerC chan<- time.Time
}

// pendingTimerHeap is a heap.Interface implementation for a slice of
// pendingTimer.
type pendingTimerHeap []*pendingTimer

func (h pendingTimerHeap) Less(i, j int) bool { return h[i].deadline.Before(h[j].deadline) }
func (h pendingTimerHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }
func (h pendingTimerHeap) Len() int           { return len(h) }

func (h *pendingTimerHeap) Push(x interface{}) { *h = append(*h, x.(*pendingTimer)) }
func (h *pendingTimerHeap) Pop() (v interface{}) {
	idx := len(*h) - 1
	v, *h = (*h)[idx], (*h)[:idx]
	return
}
