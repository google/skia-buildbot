// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package errors

import (
	"sync"
)

// LazyMultiError is a lazily-constructed MultiError.
//
// LazyMultiError is like MultiError, except that you know the ultimate size up
// front, and then you call Assign for each error encountered, and it's
// potential index. The underlying MultiError will only be allocated if one of
// the Assign'd errors is non-nil. Similarly, Get will retrieve either the
// allocated MultiError, or nil if no error was encountered.
// Build one with NewLazyMultiError.
type LazyMultiError interface {
	// Assign semantically assigns the error to the given index in the MultiError.
	// If the error is nil, no action is taken. Otherwise the MultiError is
	// allocated to its full size (if not already), and the error assigned into
	// it.
	//
	// Returns true iff err != nil (i.e. "was it assigned?"), so you can use this
	// like:
	//   if !lme.Assign(i, err) {
	//     // stuff requiring err == nil
	//   }
	Assign(int, error) bool

	// GetOne returns the error at the given index (which may be nil)
	GetOne(int) error

	// Get returns the MultiError, or nil, if no non-nil error was Assign'd.
	Get() error
}

type lazyMultiError struct {
	sync.Mutex

	size int
	me   MultiError
}

// NewLazyMultiError makes a new LazyMultiError of the provided size.
func NewLazyMultiError(size int) LazyMultiError {
	return &lazyMultiError{size: size}
}

func (e *lazyMultiError) Assign(i int, err error) bool {
	if err == nil {
		return false
	}
	e.Lock()
	defer e.Unlock()
	if e.me == nil {
		e.me = make(MultiError, e.size)
	}
	e.me[i] = err
	return true
}

func (e *lazyMultiError) GetOne(i int) error {
	e.Lock()
	defer e.Unlock()
	if e.me == nil {
		return nil
	}
	return e.me[i]
}

func (e *lazyMultiError) Get() error {
	e.Lock()
	defer e.Unlock()
	if e.me == nil {
		return nil
	}
	return e.me
}
