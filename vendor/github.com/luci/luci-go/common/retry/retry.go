// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package retry

import (
	"time"

	"github.com/luci/luci-go/common/clock"
	"github.com/luci/luci-go/common/logging"
	"golang.org/x/net/context"
)

// Stop is a sentinel value returned by a Iterator to indicate that no more
// attempts should be made.
const Stop time.Duration = -1

// Callback is a callback function that Retry will invoke every time an
// attempt fails prior to sleeping.
type Callback func(error, time.Duration)

// LogCallback builds a Callback which logs a Warning with the opname, error
// and delay.
func LogCallback(c context.Context, opname string) Callback {
	return func(err error, delay time.Duration) {
		logging.Fields{
			logging.ErrorKey: err,
			"opname":         opname,
			"delay":          delay,
		}.Warningf(c, "operation failed transiently")
	}
}

// Iterator describes a stateful implementation of retry logic.
type Iterator interface {
	// Returns the next retry delay, or Stop if no more retries should be made.
	Next(context.Context, error) time.Duration
}

// Factory is a function that produces an independent Iterator instance.
//
// Since each Iterator is mutated as it is iterated through, this is used to
// produce a fresh Iterator for a new round of retries. Unless the caller is
// fully aware of what they're doing, this should not return the an Iterator
// instance more than once.
type Factory func() Iterator

// Retry executes a function 'fn'. If the function returns an error, it will
// be re-executed according to a retry plan.
//
// If a Factory is supplied, it will be called to generate a single retry
// Iterator for this Retry round. If nil, Retry will execute the target function
// exactly once regardless of return value.
//
// If the supplied context is canceled, retry will stop executing. Retry will
// not execute the supplied function at all if the context is canceled when
// Retry is invoked.
//
// If 'callback' is not nil, it will be invoked if an error occurs (prior to
// sleeping).
func Retry(ctx context.Context, f Factory, fn func() error, callback Callback) (err error) {
	var it Iterator
	if f != nil {
		it = f()
	}

	for {
		// If we've been cancelled, don't try/retry.
		select {
		case <-ctx.Done():
			return ctx.Err()

		default:
			break
		}

		// Execute the function.
		err = fn()
		if err == nil || it == nil {
			return
		}

		delay := it.Next(ctx, err)
		if delay == Stop {
			return
		}

		// Notify our observer that we are retrying.
		if callback != nil {
			callback(err, delay)
		}

		if delay > 0 {
			if tr := <-clock.After(ctx, delay); tr.Incomplete() {
				// Context was canceled.
				return tr.Err
			}
		}
	}
}
