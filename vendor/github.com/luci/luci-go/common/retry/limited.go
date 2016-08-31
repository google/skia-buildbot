// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package retry

import (
	"time"

	"github.com/luci/luci-go/common/clock"
	"golang.org/x/net/context"
)

// Limited is an Iterator implementation that is limited by a maximum number of
// retries or time.
type Limited struct {
	Delay   time.Duration // The next generated delay.
	Retries int           // The number of remaining retries.

	MaxTotal time.Duration // The maximum total elapsed time. If zero, no maximum will be enforced.

	startTime time.Time // The time when the generator initially started.
}

var _ Iterator = (*Limited)(nil)

// Next implements the Iterator interface.
func (i *Limited) Next(ctx context.Context, _ error) time.Duration {
	if i.Retries == 0 {
		return Stop
	}
	i.Retries--

	// If there is a maximum total time, enforce it.
	if i.MaxTotal > 0 {
		now := clock.Now(ctx)
		if i.startTime.IsZero() {
			i.startTime = now
		}

		var elapsed time.Duration
		if now.After(i.startTime) {
			elapsed = now.Sub(i.startTime)
		}

		// Remaining time is the difference between total allowed time and elapsed
		// time.
		remaining := i.MaxTotal - elapsed
		if remaining <= 0 {
			// No more time!
			i.Retries = 0
			return Stop
		}
	}

	return i.Delay
}
