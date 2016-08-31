// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package clock

import (
	"time"

	"golang.org/x/net/context"
)

// Unique value for clock key.
var clockKey = "clock.Clock"

// Factory is a generator function that produces a Clock instnace.
type Factory func(context.Context) Clock

// SetFactory creates a new Context using the supplied Clock factory.
func SetFactory(ctx context.Context, f Factory) context.Context {
	return context.WithValue(ctx, &clockKey, f)
}

// Set creates a new Context using the supplied Clock.
func Set(ctx context.Context, c Clock) context.Context {
	return SetFactory(ctx, func(context.Context) Clock { return c })
}

// Get returns the Clock set in the supplied Context, defaulting to
// SystemClock() if none is set.
func Get(ctx context.Context) (clock Clock) {
	if v := ctx.Value(&clockKey); v != nil {
		if f, ok := v.(Factory); ok {
			clock = f(ctx)
		}
	}
	if clock == nil {
		clock = GetSystemClock()
	}
	return
}

// Now calls Clock.Now on the Clock instance stored in the supplied Context.
func Now(ctx context.Context) time.Time {
	return Get(ctx).Now()
}

// Sleep calls Clock.Sleep on the Clock instance stored in the supplied Context.
func Sleep(ctx context.Context, d time.Duration) TimerResult {
	return Get(ctx).Sleep(ctx, d)
}

// NewTimer calls Clock.NewTimer on the Clock instance stored in the supplied
// Context.
func NewTimer(ctx context.Context) Timer {
	return Get(ctx).NewTimer(ctx)
}

// After calls Clock.After on the Clock instance stored in the supplied Context.
func After(ctx context.Context, d time.Duration) <-chan TimerResult {
	return Get(ctx).After(ctx, d)
}

// Since is an equivalent of time.Since.
func Since(ctx context.Context, t time.Time) time.Duration {
	return Now(ctx).Sub(t)
}
