// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package clock

import (
	"sync"
	"time"

	"golang.org/x/net/context"
)

// clockContext is a context.Context implementation that uses Clock library
// constructs.
type clockContext struct {
	sync.Mutex
	context.Context

	deadline time.Time
	err      error // The error to return, in place of the embedded Context's.
}

func (c *clockContext) Deadline() (time.Time, bool) {
	return c.deadline, true
}

func (c *clockContext) Err() error {
	c.Lock()
	defer c.Unlock()

	// Prefer our error over our parent's, if set.
	if err := c.err; err != nil {
		return err
	}
	return c.Context.Err()
}

func (c *clockContext) setError(err error) {
	c.Lock()
	defer c.Unlock()

	c.err = err
}

var _ context.Context = (*clockContext)(nil)

// WithDeadline is a clock library implementation of context.WithDeadline that
// uses the clock library's time features instead of the Go time library.
//
// For more information, see context.WithDeadline.
func WithDeadline(parent context.Context, deadline time.Time) (context.Context, context.CancelFunc) {
	if cur, ok := parent.Deadline(); ok && cur.Before(deadline) {
		// The current deadline is already sooner than the new one.
		return context.WithCancel(parent)
	}

	parent, cancelFunc := context.WithCancel(parent)
	c := &clockContext{
		Context:  parent,
		deadline: deadline,
	}

	d := deadline.Sub(Now(c))
	if d <= 0 {
		// Deadline has already passed.
		c.setError(context.DeadlineExceeded)
		cancelFunc()
		return c, cancelFunc
	}

	// Invoke our cancelFunc after the specified time.
	go func() {
		if ar := <-After(c, d); !ar.Incomplete() {
			// Timer expired naturally.
			c.setError(context.DeadlineExceeded)
			cancelFunc()
		}
	}()
	return c, cancelFunc
}

// WithTimeout is a clock library implementation of context.WithTimeout that
// uses the clock library's time features instead of the Go time library.
//
// For more information, see context.WithTimeout.
func WithTimeout(parent context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	return WithDeadline(parent, Now(parent).Add(timeout))
}
