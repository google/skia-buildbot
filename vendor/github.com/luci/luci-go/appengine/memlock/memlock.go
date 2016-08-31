// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

// Package memlock allows multiple appengine handlers to coordinate best-effort
// mutual execution via memcache. "best-effort" here means "best-effort"...
// memcache is not reliable. However, colliding on memcache is a lot cheaper
// than, for example, colliding with datastore transactions.
package memlock

import (
	"bytes"
	"errors"
	"time"

	"github.com/luci/gae/service/memcache"
	"github.com/luci/luci-go/common/clock"
	"github.com/luci/luci-go/common/logging"
	"golang.org/x/net/context"
)

// ErrFailedToLock is returned from TryWithLock when it fails to obtain a lock
// prior to invoking the user-supplied function.
var ErrFailedToLock = errors.New("memlock: failed to obtain lock")

// ErrEmptyClientID is returned from TryWithLock when you specify an empty
// clientID.
var ErrEmptyClientID = errors.New("memlock: empty clientID")

// memlockKeyPrefix is the memcache Key prefix for all user-supplied keys.
const memlockKeyPrefix = "memlock:"

type checkOp string

// var so we can override it in the tests
var delay = time.Second

type testStopCBKeyType int

var testStopCBKey testStopCBKeyType

const (
	release checkOp = "release"
	refresh         = "refresh"
)

// memcacheLockTime is the expiration time of the memcache entry. If the lock
// is correctly released, then it will be released before this time. It's a
// var so we can override it in the tests.
var memcacheLockTime = 16 * time.Second

// TryWithLock attempts to obtains the lock once, and then invokes f if
// successful. The context provided to f will be canceled (e.g. ctx.Done() will
// be closed) if memlock detects that we've lost the lock.
//
// TryWithLock function returns ErrFailedToLock if it fails to obtain the lock,
// otherwise returns the error that f returns.
//
// `key` is the memcache key to use (i.e. the name of the lock). Clients locking
// the same data must use the same key. clientID is the unique identifier for
// this client (lock-holder). If it's empty then TryWithLock() will return
// ErrEmptyClientID.
//
// Note that the lock provided by TryWithLock is a best-effort lock... some
// other form of locking or synchronization should be used inside of f (such as
// Datastore transactions) to ensure that f is, in fact, operating exclusively.
// The purpose of TryWithLock is to have a cheap filter to prevent unnecessary
// contention on heavier synchronization primitives like transactions.
func TryWithLock(ctx context.Context, key, clientID string, f func(context.Context) error) error {
	if len(clientID) == 0 {
		return ErrEmptyClientID
	}

	log := logging.Get(
		logging.SetFields(ctx, logging.Fields{
			"key":      key,
			"clientID": clientID,
		}))
	mc := memcache.Get(ctx)

	key = memlockKeyPrefix + key
	cid := []byte(clientID)

	// checkAnd gets the current value from memcache, and then attempts to do the
	// checkOp (which can either be `refresh` or `release`). These pieces of
	// functionality are necessarially intertwined, because CAS only works with
	// the exact-same *Item which was returned from a Get.
	//
	// refresh will attempt to CAS the item with the same content to reset it's
	// timeout.
	//
	// release will attempt to CAS the item to remove it's contents (clientID).
	// another lock observing an empty clientID will know that the lock is
	// obtainable.
	checkAnd := func(op checkOp) bool {
		itm, err := mc.Get(key)
		if err != nil {
			log.Warningf("error getting: %s", err)
			return false
		}

		if len(itm.Value()) > 0 && !bytes.Equal(itm.Value(), cid) {
			log.Infof("lock owned by %q", string(itm.Value()))
			return false
		}

		if op == refresh {
			itm.SetValue(cid).SetExpiration(memcacheLockTime)
		} else {
			if len(itm.Value()) == 0 {
				// it's already unlocked, no need to CAS
				log.Infof("lock already released")
				return true
			}
			itm.SetValue([]byte{}).SetExpiration(delay)
		}

		if err := mc.CompareAndSwap(itm); err != nil {
			log.Warningf("failed to %s lock: %q", op, err)
			return false
		}

		return true
	}

	// Now the actual logic begins. First we 'Add' the item, which will set it if
	// it's not present in the memcache, otherwise leaves it alone.

	err := mc.Add(mc.NewItem(key).SetValue(cid).SetExpiration(memcacheLockTime))
	if err != nil {
		if err != memcache.ErrNotStored {
			log.Warningf("error adding: %s", err)
		}
		if !checkAnd(refresh) {
			return ErrFailedToLock
		}
	}

	// At this point we nominally have the lock (at least for memcacheLockTime).
	finished := make(chan struct{})
	subCtx, cancelFunc := context.WithCancel(ctx)
	defer func() {
		cancelFunc()
		<-finished
	}()

	testStopCB, _ := ctx.Value(testStopCBKey).(func())

	// This goroutine checks to see if we still possess the lock, and refreshes it
	// if we do.
	go func() {
		defer func() {
			cancelFunc()
			close(finished)
		}()

		for {
			if tr := <-clock.After(subCtx, delay); tr.Incomplete() {
				log.Infof("context done: %s", tr.Err)
				break
			}
			if !checkAnd(refresh) {
				log.Warningf("lost lock: %s", err)
				return
			}
		}

		if testStopCB != nil {
			testStopCB()
		}
		checkAnd(release)
	}()

	return f(subCtx)
}
