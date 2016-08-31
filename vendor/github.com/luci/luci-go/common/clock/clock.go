// Copyright 2014 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

// Package clock is an interface to system time and timers which is easy to
// test.
package clock

import (
	"time"

	"golang.org/x/net/context"
)

// Clock is an interface to system time.
//
// The standard clock is SystemClock, which falls through to the system time
// library. Another clock, FakeClock, is available to simulate time facilities
// for testing.
type Clock interface {
	// Returns the current time (see time.Now).
	Now() time.Time

	// Sleeps the current goroutine (see time.Sleep).
	//
	// Sleep will return a TimerResult containing the time when it was awakened
	// and detailing its execution. If the sleep terminated prematurely from
	// cancellation, the TimerResult's Incomplete() method will return true.
	Sleep(context.Context, time.Duration) TimerResult

	// Creates a new Timer instance, bound to this Clock.
	//
	// If the supplied Context is canceled, the timer will expire immediately.
	NewTimer(c context.Context) Timer

	// Waits a duration, then sends the current time over the returned channel.
	//
	// If the supplied Context is canceled, the timer will expire immediately.
	After(context.Context, time.Duration) <-chan TimerResult
}
