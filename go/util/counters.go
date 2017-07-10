package util

import (
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
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
	val  int64
	file string
	lock sync.RWMutex
	t    time.Duration
}

// NewPersistentAutoDecrementCounter returns a PersistentAutoDecrementCounter
// instance using the given file.
func NewPersistentAutoDecrementCounter(file string, t time.Duration) (*PersistentAutoDecrementCounter, error) {
	val := int64(0)
	contents, err := ioutil.ReadFile(file)
	if err == nil {
		val, err = strconv.ParseInt(string(contents), 10, 32)
		if err != nil {
			return nil, fmt.Errorf("Invalid or corrupted file for PersistentAutoDecrementCounter: %s", err)
		}
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("Unable to read file for PersistentAutoDecrementCounter: %s", err)
	}
	return &PersistentAutoDecrementCounter{
		val:  val,
		file: file,
		t:    t,
	}, nil
}

// add adds the given integer to the counter.
func (c *PersistentAutoDecrementCounter) add(v int64) error {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.val += v
	return ioutil.WriteFile(c.file, []byte(fmt.Sprintf("%d", c.val)), os.ModePerm)
}

// Inc increments the PersistentAutoDecrementCounter and schedules a decrement.
func (c *PersistentAutoDecrementCounter) Inc() error {
	if err := c.add(1); err != nil {
		return err
	}
	go func() {
		time.Sleep(c.t)
		if err := c.add(-1); err != nil {
			sklog.Errorf("Failed to decrement PersistentAutoDecrementCounter: %s", err)
		}
	}()
	return nil
}

// Get returns the current value of the PersistentAutoDecrementCounter.
func (c *PersistentAutoDecrementCounter) Get() int64 {
	c.lock.RLock()
	defer c.lock.RUnlock()
	return c.val
}
