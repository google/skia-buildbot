package util

import (
	"sync"
	"time"
)

/*
	This file contains implementations for various types of counters.
*/

// AtomicCounter implements a counter which can be incremented and decremented atomically.
type AtomicCounter struct {
	val  int
	lock sync.RWMutex
}

// Inc increments the AtomicCounter.
func (c *AtomicCounter) Inc() {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.val += 1
}

// Dec decrements the AtomicCounter.
func (c *AtomicCounter) Dec() {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.val -= 1
}

// Get returns the current value of the AtomicCounter.
func (c *AtomicCounter) Get() int {
	c.lock.RLock()
	defer c.lock.RUnlock()
	return c.val
}

// AutoDecrementCounter is an AtomicCounter in which every increment has a
// corresponding decrement which takes place after a specified time.
type AutoDecrementCounter struct {
	c AtomicCounter
	t time.Duration
}

// NewAutoDecrementCounter returns an AutoDecrementCounter with the given timeout.
func NewAutoDecrementCounter(t time.Duration) *AutoDecrementCounter {
	return &AutoDecrementCounter{
		t: t,
	}
}

// Inc increments the AutoDecrementCounter and schedules a decrement.
func (c *AutoDecrementCounter) Inc() {
	c.c.Inc()
	go func() {
		time.Sleep(c.t)
		c.c.Dec()
	}()
}

// Get returns the current value of the AutoDecrementCounter.
func (c *AutoDecrementCounter) Get() int {
	return c.c.Get()
}
