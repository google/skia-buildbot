// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package parallel

import (
	"container/list"
	"sync"
	"sync/atomic"
)

// A Buffer embeds a Runner, overriding its RunOne method to buffer tasks
// indefinitely without blocking.
type Buffer struct {
	Runner

	// lifo, if non-zero-indicates a LIFO task dispatch or, if zero, a FIFO task
	// dispatch. For more informatio, see SetFIFO.
	lifo int32

	// initOnce ensures that the Buffer is initialized at most once.
	initOnce sync.Once
	// workC receives enqueued tasks for processing.
	workC chan WorkItem
	// tasksFinishedC is used to signal Close when our list has finished
	// dispatching tasks.
	tasksFinishedC chan struct{}
}

func (b *Buffer) init() {
	b.initOnce.Do(func() {
		b.workC = make(chan WorkItem)
		b.tasksFinishedC = make(chan struct{})

		go b.process()
	})
}

// process enqueues tasks into the Buffer and dispatches them to the underlying
// Runner when available.
func (b *Buffer) process() {
	defer close(b.tasksFinishedC)

	// outC is the channel that we send work to. We toggle it between nil and our
	// Runner's WorkC depending on whether we have work.
	//
	// cur is the current work to send. It is only valid if hasWork is true.
	var outC chan<- WorkItem
	var cur WorkItem

	// This is our work buffer. If we have unsent work, any additional work will
	// be written to this buffer.
	var buf list.List

	// Our main processing loop.
	inC := b.workC
	for {
		select {
		case work, ok := <-inC:
			if !ok {
				// Our work channel has been closed. We aren't accepting any new tasks.
				if outC == nil && buf.Len() == 0 {
					// We have no buffered work; exit immediately.
					return
				}

				// Mark that we're closed. When all of our work drains, we will exit.
				inC = nil
				break
			}

			// If we have no immediate work, send "work" directly; otherwise, buffer
			// work for future sending.
			if outC == nil {
				cur = work
				outC = b.Runner.WorkC()
			} else {
				buf.PushBack(&work)
			}

		case outC <- cur:
			// "cur" has been sent. Dequeue the next work item, or set outC to nil if
			// there are no more items.
			switch {
			case buf.Len() > 0:
				var e *list.Element
				if b.isFIFO() {
					e = buf.Front()
				} else {
					e = buf.Back()
				}

				cur = *(buf.Remove(e).(*WorkItem))
			case inC == nil:
				//  There's no more immediate work, no buffered work, and we're closed,
				//  so we're finished.
				return
			default:
				// No more work to send.
				outC = nil
			}
		}
	}
}

// Run implements the same semantics as Runner's Run. However, if the
// dispatch pipeline is full, Run will buffer the work and return immediately
// rather than block.
func (b *Buffer) Run(gen func(chan<- func() error)) <-chan error {
	return b.runThen(gen, nil)
}

// Run implements the same semantics as Runner's Run. However, if the
// dispatch pipeline is full, Run will buffer the work and return immediately
// rather than block.
func (b *Buffer) runThen(gen func(chan<- func() error), then func()) <-chan error {
	b.init()
	return runImpl(gen, b.workC, then)
}

// RunOne implements the same semantics as Runner's RunOne. However, if the
// dispatch pipeline is full, RunOne will buffer the work and return immediately
// rather than block.
func (b *Buffer) RunOne(f func() error) <-chan error {
	b.init()

	errC := make(chan error)
	b.workC <- WorkItem{
		F:     f,
		ErrC:  errC,
		After: func() { close(errC) },
	}
	return errC
}

// WorkC implements the same semantics as Runner's WorkC. However, this channel
// will not block pending work dispatch. Any tasks written to this channel that
// would block are instead buffered pending dispatch availability.
func (b *Buffer) WorkC() chan<- WorkItem {
	b.init()

	return b.workC
}

// Close flushes the remaining tasks in the Buffer and Closes the underlying
// Runner.
//
// Adding new tasks to the Buffer after Close has been invoked will cause a
// panic.
func (b *Buffer) Close() {
	b.init()

	close(b.workC)
	<-b.tasksFinishedC
	b.Runner.Close()
}

// SetFIFO sets the Buffer's task dispatch order to FIFO (true) or LIFO (false).
// This determines the order in which buffered tasks will be dispatched. In
// FIFO (first in, first out) mode, the first tasks to be buffered will be
// dispatchd first. In LIFO (last in, last out) mode, the last tasks to be
// buffered will be dispatched first.
func (b *Buffer) SetFIFO(fifo bool) {
	if fifo {
		atomic.StoreInt32(&b.lifo, 0)
	} else {
		atomic.StoreInt32(&b.lifo, 1)
	}
}

func (b *Buffer) isFIFO() bool {
	return atomic.LoadInt32(&b.lifo) == 0
}
