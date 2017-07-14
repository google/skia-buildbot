package util

import (
	"encoding/gob"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"sync"
	"time"

	"go.skia.org/infra/go/sklog"
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

// PersistentAutoDecrementCounter is an AutoDecrementCounter which uses a file
// to persist its value between program restarts.
type PersistentAutoDecrementCounter struct {
	times    []time.Time
	file     string
	mtx      sync.RWMutex
	duration time.Duration
}

// NewPersistentAutoDecrementCounter returns a PersistentAutoDecrementCounter
// instance using the given file.
func NewPersistentAutoDecrementCounter(file string, t time.Duration) (*PersistentAutoDecrementCounter, error) {
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
	c := &PersistentAutoDecrementCounter{
		times:    times,
		file:     file,
		duration: t,
	}
	// Write the file in case we didn't have one before.
	if err := c.write(); err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	for _, t := range c.times {
		go c.decAfter(t.Sub(now))
	}
	return c, nil
}

// writeTemp writes the timings to a temporary file and returns its name.
func (c *PersistentAutoDecrementCounter) writeTemp() (string, error) {
	f, err := ioutil.TempFile(path.Dir(c.file), path.Base(c.file))
	if err != nil {
		return "", err
	}
	if err := gob.NewEncoder(f).Encode(c.times); err != nil {
		Close(f)
		Remove(f.Name())
		return "", err
	}
	if err := f.Close(); err != nil {
		Remove(f.Name())
		return "", err
	}
	return f.Name(), nil
}

// write the timings to the backing file. Assumes the caller holds a write lock.
func (c *PersistentAutoDecrementCounter) write() error {
	tmpFile, err := c.writeTemp()
	if err != nil {
		return err
	}
	return os.Rename(tmpFile, c.file)
}

// Inc increments the PersistentAutoDecrementCounter and schedules a decrement.
func (c *PersistentAutoDecrementCounter) Inc() error {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	c.times = append(c.times, time.Now().UTC().Add(c.duration))
	go c.decAfter(c.duration)
	return c.write()
}

// dec decrements the PersistentAutoDecrementCounter.
func (c *PersistentAutoDecrementCounter) dec() error {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	if time.Now().UTC().After(c.times[0]) {
		c.times = c.times[1:]
	}
	return c.write()
}

// decAfter waits until the specified duration has passed and decrements the
// counter.
func (c *PersistentAutoDecrementCounter) decAfter(d time.Duration) {
	time.Sleep(d)
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
