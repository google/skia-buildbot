package counters

/*
   This file contains implementations for various types of counters.
*/

import (
	"context"
	"encoding/gob"
	"fmt"
	"sync"
	"time"

	"cloud.google.com/go/storage"
	"go.skia.org/infra/go/gcs"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
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
// in GCS to persist its value between program restarts.
type PersistentAutoDecrementCounter struct {
	gcs      gcs.GCSClient
	times    []time.Time
	file     string
	mtx      sync.RWMutex
	duration time.Duration
}

// Helper function for reading the PersistentAutoDecrementCounter's timings from
// a backing file in GCS.
func read(ctx context.Context, gcsClient gcs.GCSClient, path string) ([]time.Time, error) {
	times := []time.Time{}
	r, err := gcsClient.FileReader(ctx, path)
	if err == nil {
		defer util.Close(r)
		if err = gob.NewDecoder(r).Decode(&times); err != nil {
			return nil, fmt.Errorf("Invalid or corrupted file for PersistentAutoDecrementCounter: %s", err)
		}
	} else if err != storage.ErrObjectNotExist {
		return nil, fmt.Errorf("Unable to read file for PersistentAutoDecrementCounter: %s", err)
	}
	return times, nil
}

// NewPersistentAutoDecrementCounter returns a PersistentAutoDecrementCounter
// instance using the given file.
func NewPersistentAutoDecrementCounter(ctx context.Context, gcsClient gcs.GCSClient, path string, d time.Duration) (*PersistentAutoDecrementCounter, error) {
	times, err := read(ctx, gcsClient, path)
	if err != nil {
		return nil, err
	}
	c := &PersistentAutoDecrementCounter{
		gcs:      gcsClient,
		times:    times,
		file:     path,
		duration: d,
	}
	// Write the file in case we didn't have one before.
	if err := c.write(ctx); err != nil {
		return nil, err
	}
	// Start timers for any existing counts.
	now := timeNowFunc().UTC()
	for _, t := range c.times {
		t := t
		timeAfterFunc(t.Sub(now), func() {
			c.decLogErr(ctx, t)
		})
	}
	return c, nil
}

// write the timings to the backing file. Assumes the caller holds a write lock.
func (c *PersistentAutoDecrementCounter) write(ctx context.Context) error {
	w := c.gcs.FileWriter(ctx, c.file, gcs.FILE_WRITE_OPTS_TEXT)
	defer util.Close(w)
	return gob.NewEncoder(w).Encode(c.times)
}

// Inc increments the PersistentAutoDecrementCounter and schedules a decrement.
func (c *PersistentAutoDecrementCounter) Inc(ctx context.Context) error {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	decTime := timeNowFunc().UTC().Add(c.duration)
	c.times = append(c.times, decTime)
	timeAfterFunc(c.duration, func() {
		c.decLogErr(ctx, decTime)
	})
	return c.write(ctx)
}

// dec decrements the PersistentAutoDecrementCounter.
func (c *PersistentAutoDecrementCounter) dec(ctx context.Context, t time.Time) error {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	for i, x := range c.times {
		if x == t {
			c.times = append(c.times[:i], c.times[i+1:]...)
			return c.write(ctx)
		}
	}
	sklog.Debugf("PersistentAutoDecrementCounter: Nothing to delete; did we get reset?")
	return nil
}

// decLogErr decrements the PersistentAutoDecrementCounter and logs any error.
func (c *PersistentAutoDecrementCounter) decLogErr(ctx context.Context, t time.Time) {
	if err := c.dec(ctx, t); err != nil {
		sklog.Errorf("Failed to persist PersistentAutoDecrementCounter: %s", err)
	}
}

// Get returns the current value of the PersistentAutoDecrementCounter.
func (c *PersistentAutoDecrementCounter) Get() int64 {
	c.mtx.RLock()
	defer c.mtx.RUnlock()
	return int64(len(c.times))
}

// Reset resets the value of the PersistentAutoDecrementCounter to zero.
func (c *PersistentAutoDecrementCounter) Reset(ctx context.Context) error {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	sklog.Debugf("PersistentAutoDecrementCounter: reset.")
	c.times = []time.Time{}
	return c.write(ctx)
}

// GetDecrementTimes returns a slice of time.Time which indicate *roughly* when
// the counter will be decremented. This is informational only, and the caller
// should not rely on the times to be perfectly accurate.
func (c *PersistentAutoDecrementCounter) GetDecrementTimes() []time.Time {
	c.mtx.RLock()
	defer c.mtx.RUnlock()
	rv := make([]time.Time, len(c.times))
	for i, t := range c.times {
		rv[i] = t
	}
	return rv
}
