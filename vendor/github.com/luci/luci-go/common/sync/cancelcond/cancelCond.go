// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

// Package cancelcond implements a wrapper around sync.Cond that response to
// context.Context cancellation.
package cancelcond

import (
	"sync"

	"golang.org/x/net/context"
)

// Cond is a wrapper around a sync.Cond that overloads its Wait method to accept
// a Context. This Context can be cancelled to prematurely terminate the Wait().
type Cond struct {
	*sync.Cond
}

// New creates a new Context-cancellable Cond.
func New(l sync.Locker) *Cond {
	return &Cond{
		Cond: sync.NewCond(l),
	}
}

// Wait wraps sync.Cond's Wait() method. It blocks, waiting for the underlying
// Conn to be signalled. If the Context is cancelled prematurely, Wait() will
// signal the underlying Cond and unblock it.
//
// Wait must be called while holding the Cond's lock. It yields the lock while
// it is blocking and reclaims it prior to returning.
func (cc *Cond) Wait(c context.Context) (err error) {
	// If we're already cancelled, return immediately.
	select {
	case <-c.Done():
		return c.Err()
	default:
		break
	}

	// Monitor our Context. If cancelled, it will broadcast a wakeup signal to our
	// Cond.
	//
	// Use "stopC" to make sure that we reap the goroutine before actually
	// returning. This will prevent us from leaking goroutines if the timeout is
	// never hit.
	stopC := make(chan struct{})
	finishedC := make(chan error)
	go func() {
		defer close(finishedC)

		select {
		case <-c.Done():
			err = c.Err()
			cc.Broadcast()
			break

		case <-stopC:
			break
		}
	}()
	defer func() {
		close(stopC)
		<-finishedC
	}()

	cc.Cond.Wait()
	return
}
