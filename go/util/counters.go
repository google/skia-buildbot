package util

/*
   This file contains implementations for various types of counters.
*/

import (
	"encoding/gob"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"go.skia.org/infra/go/sklog"
)

// These can be mocked for testing.
var timeAfterFunc = time.AfterFunc
var timeNowFunc = time.Now

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

// PersistentAutoDecrementCounter is an AutoDecrementCounter which uses a file
// to persist its value between program restarts.
type PersistentAutoDecrementCounter struct {
	times    []time.Time
	file     string
	mtx      sync.RWMutex
	duration time.Duration
}

// Helper function for reading the PersistentAutoDecrementCounter's timings from
// a backing file.
func read(file string) ([]time.Time, error) {
	times := []time.Time{}
	f, err := os.Open(file)
	if err == nil {
		defer Close(f)
		if err = gob.NewDecoder(f).Decode(&times); err != nil {
			return nil, fmt.Errorf("Invalid or corrupted file for PersistentAutoDecrementCounter: %s", err)
		}
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("Unable to read file for PersistentAutoDecrementCounter: %s", err)
	}
	return times, nil
}

// NewPersistentAutoDecrementCounter returns a PersistentAutoDecrementCounter
// instance using the given file.
func NewPersistentAutoDecrementCounter(file string, d time.Duration) (*PersistentAutoDecrementCounter, error) {
	times, err := read(file)
	if err != nil {
		return nil, err
	}
	c := &PersistentAutoDecrementCounter{
		times:    times,
		file:     file,
		duration: d,
	}
	// Write the file in case we didn't have one before.
	if err := c.write(); err != nil {
		return nil, err
	}
	// Start timers for any existing counts.
	now := timeNowFunc().UTC()
	for _, t := range c.times {
		timeAfterFunc(t.Sub(now), func() {
			c.decLogErr()
		})
	}
	return c, nil
}

// write the timings to the backing file. Assumes the caller holds a write lock.
func (c *PersistentAutoDecrementCounter) write() error {
	return WithWriteFile(c.file, func(w io.Writer) error {
		return gob.NewEncoder(w).Encode(c.times)
	})
}

// Inc increments the PersistentAutoDecrementCounter and schedules a decrement.
func (c *PersistentAutoDecrementCounter) Inc() error {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	c.times = append(c.times, timeNowFunc().UTC().Add(c.duration))
	timeAfterFunc(c.duration, func() {
		c.decLogErr()
	})
	return c.write()
}

// dec decrements the PersistentAutoDecrementCounter.
func (c *PersistentAutoDecrementCounter) dec() error {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	c.times = c.times[1:]
	return c.write()
}

// decLogErr decrements the PersistentAutoDecrementCounter and logs any error.
func (c *PersistentAutoDecrementCounter) decLogErr() {
	if err := c.dec(); err != nil {
		sklog.Errorf("Failed to decrement PersistentAutoDecrementCounter: %s", err)
	}
}

// Get returns the current value of the PersistentAutoDecrementCounter.
func (c *PersistentAutoDecrementCounter) Get() int64 {
	c.mtx.RLock()
	defer c.mtx.RUnlock()
	return int64(len(c.times))
}
