// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package parallel

// SemaphoreToken is a semaphore token.
type SemaphoreToken struct{}

// Semaphore is a sync.Locker that implements a n-semaphore.
//
// Lock the semaphore acquires a semaphore token, possibly blocking until one
// is available.
//
// Unlock releases an owned token, returning it to the semaphore.
//
// For semaphore s, len(s) is the current number of acquired resources, and
// cap(s) is the total resource size of the semaphore.
type Semaphore chan SemaphoreToken

// Lock acquires a semaphore resource, blocking until one is available.
func (s Semaphore) Lock() {
	if cap(s) > 0 {
		s <- SemaphoreToken{}
	}
}

// Unlock releases a single semaphore resource.
func (s Semaphore) Unlock() {
	if cap(s) > 0 {
		<-s
	}
}

// TakeAll blocks until it holds all available semaphore resources. When it
// returns, the caller owns all of the resources in the semaphore.
func (s Semaphore) TakeAll() {
	for i := 0; i < cap(s); i++ {
		s.Lock()
	}
}
