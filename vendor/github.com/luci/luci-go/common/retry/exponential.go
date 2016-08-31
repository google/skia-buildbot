// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package retry

import (
	"time"

	"golang.org/x/net/context"
)

// ExponentialBackoff is an Iterator implementation that implements exponential
// backoff retry.
type ExponentialBackoff struct {
	Limited

	Multiplier float64       // The exponential multiplier. If zero, a default of 2 will be used.
	MaxDelay   time.Duration // The maximum duration. If zero, no maximum will be enforced.
}

// Next implements Iterator.
func (b *ExponentialBackoff) Next(ctx context.Context, err error) time.Duration {
	// Get Limited base delay.
	delay := b.Limited.Next(ctx, err)
	if delay == Stop {
		return Stop
	}

	// Bound our delay.
	if b.MaxDelay != 0 && b.MaxDelay < delay {
		delay = b.MaxDelay
	} else {
		// Calculate the next delay exponentially.
		multiplier := b.Multiplier
		if multiplier < 1 {
			multiplier = 2
		}
		b.Delay = time.Duration(float64(b.Delay) * multiplier)
	}
	return delay
}
