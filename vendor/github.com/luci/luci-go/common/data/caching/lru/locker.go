// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

// Package lru provides least-recently-used (LRU) cache.
package lru

import (
	"sync"
)

// Locker is a read/write locker interface.
//
// Its RLock and RUnlock methods follow the same conventions as sync.RWMutex.
type Locker interface {
	sync.Locker

	// RLock locks the Locker for reading.
	RLock()
	// RLock unlocks the Locker for reading.
	RUnlock()
}

// nopLocker is a Locker implementation that performs no locking.
type nopLocker struct{}

func (nopLocker) Lock()    {}
func (nopLocker) Unlock()  {}
func (nopLocker) RLock()   {}
func (nopLocker) RUnlock() {}
