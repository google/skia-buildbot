// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package promise

import (
	"errors"
	"sync"

	"golang.org/x/net/context"
)

var (
	// ErrNoData is an error returned when a request completes without available data.
	ErrNoData = errors.New("promise: No Data")
)

// Generator is the Promise's generator type.
type Generator func(context.Context) (interface{}, error)

// Promise is a promise structure with goroutine-safe methods that is
// responsible for owning a single piece of data. Promises have multple readers
// and a single writer.
//
// Readers will retrieve the Promise's data via Get(). If the data has not been
// populated, the reader will block pending the data. Once the data has been
// delivered, all readers will unblock and receive a reference to the Promise's
// data.
type Promise struct {
	signalC chan struct{} // Channel whose closing signals that the data is available.

	// onGet, if not nil, is invoked when Get is called.
	onGet func(context.Context)

	data interface{} // The Promise's data.
	err  error       // The error status.
}

// New instantiates a new, empty Promise instance. The Promise's value will be
// the value returned by the supplied generator function.
//
// The generator will be invoked immediately in its own goroutine.
func New(c context.Context, gen Generator) *Promise {
	p := Promise{
		signalC: make(chan struct{}),
	}

	// Execute our generator function in a separate goroutine.
	go p.runGen(c, gen)
	return &p
}

// NewDeferred instantiates a new, empty Promise instance. The Promise's value
// will be the value returned by the supplied generator function.
//
// Unlike New, the generator function will not be immediately executed. Instead,
// it will be run when the first call to Get is made, and will use one of the
// Get callers' goroutines.
// goroutine a Get caller.
func NewDeferred(gen Generator) *Promise {
	var startOnce sync.Once

	p := Promise{
		signalC: make(chan struct{}),
	}
	p.onGet = func(c context.Context) {
		startOnce.Do(func() { p.runGen(c, gen) })
	}
	return &p
}

func (p *Promise) runGen(c context.Context, gen Generator) {
	defer close(p.signalC)
	p.data, p.err = gen(c)
}

// Get returns the promise's value. If the value isn't set, Get will block until
// the value is available, following the Context's timeout parameters.
//
// If the value is available, it will be returned with its error status. If the
// context times out or is cancelled, the appropriate context error will be
// returned.
func (p *Promise) Get(c context.Context) (interface{}, error) {
	// If we have an onGet function, run it (deferred case).
	if p.onGet != nil {
		p.onGet(c)
	}

	// Block until at least one of these conditions is satisfied. If both are,
	// "select" will choose one pseudo-randomly.
	select {
	case <-p.signalC:
		return p.data, p.err

	case <-c.Done():
		// Make sure we don't actually have data.
		select {
		case <-p.signalC:
			return p.data, p.err

		default:
			return nil, c.Err()
		}
	}
}

// Peek returns the promise's current value. If the value isn't set, Peek will
// return immediately with ErrNoData.
func (p *Promise) Peek() (interface{}, error) {
	select {
	case <-p.signalC:
		return p.data, p.err

	default:
		return nil, ErrNoData
	}
}
