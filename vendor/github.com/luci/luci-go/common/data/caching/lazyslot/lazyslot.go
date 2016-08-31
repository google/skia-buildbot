// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

// Package lazyslot implements a caching scheme for globally shared objects that
// take significant time to refresh.
//
// The defining property of the implementation is that only one goroutine will
// block when refreshing such object, while all others will use a slightly stale
// cached copy.
package lazyslot

import (
	"sync"
	"time"

	"golang.org/x/net/context"

	"github.com/luci/luci-go/common/clock"
)

// Value is what's stored in a Slot. It is treated as immutable value.
type Value struct {
	// Value is whatever fetcher returned.
	Value interface{}
	// Expiration is time when this value expires and should be refetched.
	Expiration time.Time
}

// Fetcher knows how to load a new value.
//
// If it returns no errors, it MUST return non-nil Value.Value or Slot.Get will
// panic.
type Fetcher func(c context.Context, prev Value) (Value, error)

// Slot holds a cached Value and refreshes it when it expires.
//
// Only one goroutine will be busy refreshing, all others will see a slightly
// stale copy of the value during the refresh.
type Slot struct {
	Fetcher Fetcher       // used to actually load the value on demand
	Timeout time.Duration // how long to allow to fetch, 15 sec by default.

	lock              sync.RWMutex    // protects the guts below
	current           *Value          // currently known value or nil if not fetched
	currentFetcherCtx context.Context // non-nil if some goroutine is fetching now
}

// Get returns stored value if it is still fresh.
//
// It may return slightly stale copy if some other goroutine is fetching a new
// copy now. If there's no cached copy at all, blocks until it is retrieved.
//
// Returns an error only when Fetcher returns an error. Panics if fetcher
// doesn't produce a value, and doesn't return an error.
func (s *Slot) Get(c context.Context) (result Value, err error) {
	// state is populate in the anonymous function below.
	var state struct {
		C         context.Context
		Fetcher   Fetcher
		PrevValue Value
	}

	result, err, done := func() (result Value, err error, done bool) {
		now := clock.Now(c)

		// Fast path. Checks a cached value exists and it is still fresh or some
		// goroutine is already updating it (in that case we return a stale copy).
		s.lock.RLock()
		if s.current != nil && (now.Before(s.current.Expiration) || s.currentFetcherCtx != nil) {
			result = *s.current
			done = true
		}
		s.lock.RUnlock()
		if done {
			return
		}

		// Slow path. This lock protects the guts of the slot and makes sure only
		// one goroutine is doing an initial fetch.
		s.lock.Lock()
		defer s.lock.Unlock()

		// A cached value exists and it is still fresh? Return it right away.
		if s.current != nil && now.Before(s.current.Expiration) {
			result = *s.current
			done = true
			return
		}

		// Fetching the value for the first time ever? Do it under the lock because
		// there's nothing to return yet. All goroutines would have to wait for this
		// initial fetch to complete. They'll all block on s.lock.RLock() above.
		if s.current == nil {
			result, err = doFetch(s.makeFetcherCtx(c), s.Fetcher, Value{})
			if err == nil {
				s.current = &result
			}
			done = true
			return
		}

		// We have a cached copy but it has expired. Maybe some other goroutine is
		// fetching it already? Returns the cached stale copy if so.
		if s.currentFetcherCtx != nil {
			result = *s.current
			done = true
			return
		}

		// No one is fetching the value now, we should do it. Prepare a new context
		// that will be used to do the fetch once lock is released.
		s.currentFetcherCtx = s.makeFetcherCtx(c)

		// Copy lock-protected guts into local variables before releasing the lock.
		state.C = s.currentFetcherCtx
		state.Fetcher = s.Fetcher
		state.PrevValue = *s.current
		return
	}()
	if done {
		return
	}

	// Finish the fetch and update the cached value.
	return func() (result Value, err error) {
		defer func() {
			s.lock.Lock()
			defer s.lock.Unlock()
			s.currentFetcherCtx = nil
			// result.Value is not nil iff fetch succeeded and didn't panic.
			if result.Value != nil {
				s.current = &result
			}
		}()
		return doFetch(state.C, state.Fetcher, state.PrevValue)
	}()
}

// makeFetcherCtx prepares a context to use for fetch operation.
//
// Must be called under the lock.
func (s *Slot) makeFetcherCtx(c context.Context) context.Context {
	timeout := 15 * time.Second
	if s.Timeout != 0 {
		timeout = s.Timeout
	}
	fetcherCtx, _ := clock.WithTimeout(c, timeout)
	return fetcherCtx
}

// doFetch calls fetcher callback and validates return value.
func doFetch(ctx context.Context, cb Fetcher, prev Value) (result Value, err error) {
	result, err = cb(ctx, prev)
	switch {
	case err == nil && result.Value == nil:
		panic("lazyslot.Slot Fetcher returned nil value")
	case err != nil:
		result = Value{}
	}
	return
}
