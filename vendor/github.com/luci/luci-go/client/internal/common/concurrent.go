// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package common

import (
	"container/heap"
	"errors"
	"io"
	"os"
	"os/signal"
	"sync"
)

// ErrCanceled is the default reason (error) for cancelation of Cancelable.
var ErrCanceled = errors.New("canceled")

type semaphore chan bool

func newSemaphore(size int) semaphore {
	s := make(semaphore, size)
	for i := 0; i < cap(s); i++ {
		s <- true
	}
	return s
}

// wait waits for the Semaphore to have 1 item available.
//
// Returns ErrCanceled if canceled before 1 item was acquired.
// It may return nil even if the cancelable was canceled.
func (s semaphore) wait(canceler Canceler) error {
	// "select" randomly selects a channel when both channels are available. It
	// is still possible that the other channel is also available.
	select {
	case <-canceler.Channel():
		return ErrCanceled
	case <-s:
		return nil
	}
}

// signal adds 1 to the semaphore.
func (s semaphore) signal() {
	s <- true
}

// Cancelable implements asynchronous cancelation of execution.
//
// Typical implementation of Cancelable is a pipeline step doing work
// asynchronously in some goroutine. Calling Cancel would ask this goroutine
// to stop processing as soon as possible.
type Cancelable interface {
	// Cancel asks to cancel execution asynchronously.
	//
	// If reason is nil, default ErrCanceled is used.
	// Calling more than once is a no-op: the call is ingored.
	//
	// Cancel opportunistically asks to stop execution, but does not wait
	// for execution to actually stop. It is thus suitable to be called from
	// any other goroutine and will not cause a deadlock.
	Cancel(reason error)

	// CancelationReason returns the reason provided by the first Cancel() call,
	// or nil if not yet canceled.
	CancelationReason() error
}

// Canceler implements Cancelable, and assists in graceful interrupt and
// error handling in pipelines with channels.
type Canceler interface {
	Cancelable
	// Close releases all resources and makes the instance unusable.
	// Returns error to implement io.Closer , but it is always nil.
	// Calling more than once is a no-op: the call does nothing.
	io.Closer

	// Channel returns the channel producing reason values once Cancel() is
	// called. When Close() is called, this channel is closed.
	//
	// Use it in producers to ublock when Cancel() is called:
	//	func produce(out chan <- interface{}, item interface{}) error {
	//		select {
	//			case reason, ok := <- Canceler.Channel():
	//				if !ok {
	//					// Canceler is closed; for us it's just as Canceled.
	//					return ErrCanceled
	//				}
	//				return reason // Canceled with this reason.
	//			case out := <- item:
	//				return nil
	//		}
	//	}
	Channel() <-chan error
}

// CancelOnCtrlC makes a Canceler to be canceled on Ctrl-C (os.Interrupt).
//
// It is fine to call this function multiple times on multiple Canceler.
func CancelOnCtrlC(c Canceler) {
	interrupted := make(chan os.Signal, 1)
	signal.Notify(interrupted, os.Interrupt)
	go func() {
		defer signal.Stop(interrupted)
		select {
		case <-interrupted:
			c.Cancel(errors.New("Ctrl-C"))
		case <-c.Channel():
		}
	}()
}

// NewCanceler returns a new instance of canceler.
//
// Call Cancel() once no longer used to release resources.
func NewCanceler() Canceler {
	return newCanceler()
}

// GoroutinePool executes at a limited number of jobs concurrently, queueing
// others.
//
// GoroutinePool implements Canceler allowing canceling all not yet started
// jobs.
type GoroutinePool interface {
	Canceler

	// Wait blocks until all started jobs finish.
	//
	// Returns nil if all jobs have been executed, or reason for cancelation of
	// some jobs. This call is not itself cancelable, meaning that it'll block
	// until all worker goroutines are finished.
	Wait() error
	// Schedule adds a new job for execution as a separate goroutine. If the
	// GoroutinePool is canceled, onCanceled is called instead. It is fine to
	// pass nil as onCanceled.
	Schedule(job func(), onCanceled func())
}

// NewGoroutinePool creates a new GoroutinePool with at most maxConcurrentJobs.
func NewGoroutinePool(maxConcurrentJobs int, canceler Canceler) GoroutinePool {
	if canceler == nil {
		return nil
	}
	return &goroutinePool{
		Canceler: canceler,
		sema:     newSemaphore(maxConcurrentJobs),
	}
}

type goroutinePool struct {
	Canceler
	wg   sync.WaitGroup
	sema semaphore
}

func (c *goroutinePool) Wait() error {
	c.wg.Wait()
	return c.CancelationReason()
}

func (c *goroutinePool) Schedule(job func(), onCanceled func()) {
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		if c.sema.wait(c.Canceler) == nil {
			defer c.sema.signal()
			// Do not start a new job if canceled.
			if c.CancelationReason() == nil {
				job()
				return
			}
		}
		if onCanceled != nil {
			onCanceled()
		}
	}()
}

// GoroutinePriorityPool executes at a limited number of jobs concurrently,
// queueing others.
//
// GoroutinePriorityPool implements Canceler allowing canceling all not yet
// started jobs.
type GoroutinePriorityPool interface {
	Canceler

	// Wait blocks until all started jobs finish.
	//
	// Returns nil if all jobs have been executed, or reason for cancelation of
	// some jobs. This call is not itself cancelable, meaning that it'll block
	// until all worker goroutines are finished.
	Wait() error
	// Schedule adds a new job for execution as a separate goroutine.
	//
	// If the GoroutinePriorityPool is canceled, onCanceled is called instead. It
	// is fine to pass nil as onCanceled. Smaller values of priority imply earlier
	// execution.
	Schedule(priority int64, job func(), onCanceled func())
}

// NewGoroutinePriorityPool creates a new goroutine pool with at most
// maxConcurrentJobs and schedules jobs according to priority.
func NewGoroutinePriorityPool(maxConcurrentJobs int, canceler Canceler) GoroutinePriorityPool {
	if canceler == nil {
		return nil
	}
	pool := &goroutinePriorityPool{
		Canceler: canceler,
		sema:     newSemaphore(maxConcurrentJobs),
		tasks:    priorityQueue{},
	}
	return pool
}

type goroutinePriorityPool struct {
	Canceler
	sema  semaphore
	wg    sync.WaitGroup
	lock  sync.Mutex
	tasks priorityQueue
}

func (c *goroutinePriorityPool) Wait() error {
	c.wg.Wait()
	return c.CancelationReason()
}

func (c *goroutinePriorityPool) Schedule(priority int64, job func(), onCanceled func()) {
	c.wg.Add(1)
	c.lock.Lock()
	defer c.lock.Unlock()
	heap.Push(&c.tasks, &task{priority, job, onCanceled})
	go c.executeOne()
}

func (c *goroutinePriorityPool) executeOne() {
	defer c.wg.Done()
	if c.sema.wait(c.Canceler) == nil {
		defer c.sema.signal()
		task := c.popTask()
		// Do not start a new job if canceled.
		if c.CancelationReason() == nil {
			task.job()
		} else if task.onCanceled != nil {
			task.onCanceled()
		}
	} else if task := c.popTask(); task.onCanceled != nil {
		task.onCanceled()
	}
}

func (c *goroutinePriorityPool) popTask() *task {
	c.lock.Lock()
	defer c.lock.Unlock()
	return heap.Pop(&c.tasks).(*task)
}

type task struct {
	priority   int64
	job        func()
	onCanceled func()
}

// A priorityQueue implements heap.Interface and holds tasks.
type priorityQueue []*task

func (pq priorityQueue) Len() int { return len(pq) }

func (pq priorityQueue) Less(i, j int) bool {
	return pq[i].priority < pq[j].priority
}

func (pq priorityQueue) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
}

func (pq *priorityQueue) Push(x interface{}) {
	item := x.(*task)
	*pq = append(*pq, item)
}

func (pq *priorityQueue) Pop() interface{} {
	old := *pq
	n := len(old)
	item := old[n-1]
	// This will leak up to the largest extent the pool ever grew to, until the
	// pool itself is cleared up.
	*pq = old[0 : n-1]
	return item
}

// canceler implements Canceler interface.
type canceler struct {
	channel      chan error
	closeChannel chan bool

	lock   sync.Mutex
	reason error
	closed bool
}

func newCanceler() *canceler {
	return &canceler{
		channel:      make(chan error),
		closeChannel: make(chan bool),
	}
}

func (c *canceler) Cancel(reason error) {
	c.lock.Lock()
	defer c.lock.Unlock()
	if c.reason != nil {
		// Called more than once, ignore.
		return
	}
	if reason == nil {
		reason = ErrCanceled
	}
	c.reason = reason
	if c.closed {
		return
	}
	go func() {
		defer close(c.channel)
		for {
			select {
			case <-c.closeChannel:
				return
			case c.channel <- c.reason:
			}
		}
	}()
}

func (c *canceler) CancelationReason() error {
	c.lock.Lock()
	defer c.lock.Unlock()
	return c.reason
}

func (c *canceler) Channel() <-chan error {
	return c.channel
}

func (c *canceler) Close() error {
	c.lock.Lock()
	defer c.lock.Unlock()
	if c.closed {
		return nil
	}
	c.closed = true
	close(c.closeChannel)
	// If Cancel() was called before, then its goroutine will close c.channel.
	// Otherwise, we do.
	if c.reason == nil {
		close(c.channel)
	}
	return nil
}
