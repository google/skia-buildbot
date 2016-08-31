// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package parallel

import (
	"sync"
	"sync/atomic"
)

// WorkItem is a single item of work that a Runner will execute. The supplied
// function, F, will be executed by a Runner goroutine and the result will
// be written to ErrC.
//
// An optional callback method, After, may be supplied to operate in response
// to work completion.
type WorkItem struct {
	// F is the work function to execute. This must be non-nil.
	F func() error
	// ErrC is the channel that will receive F's result. If nil or F panics, no
	// error will be sent.
	ErrC chan<- error

	// After, if not nil, is a callback method that will be invoked after the
	// result of F has been passed to ErrC.
	//
	// After is called by the same worker goroutine as F, so it will similarly
	// consume one worker during its execution.
	//
	// If F panics, After will still be called, and can be used to recover from
	// the panic.
	After func()
}

func (wi *WorkItem) execute() {
	if wi.After != nil {
		defer wi.After()
	}

	err := wi.F()
	if wi.ErrC != nil {
		wi.ErrC <- err
	}
}

// Runner manages parallel function dispatch.
//
// The zero value of a Runner accepts an unbounded number of tasks and maintains
// no sustained goroutines.
//
// Once started, a Runner must not be copied.
//
// Once a task has been dispatched to Runner, it will continue accepting tasks
// and consuming resources (namely, its dispatch goroutine) until its Close
// method is called.
type Runner struct {
	// Sustained is the number of sustained goroutines to use in this Runner.
	// Sustained goroutines are spawned on demand, but continue running to
	// dispatch future work until the Runner is closed.
	//
	// If Sustained is <= 0, no sustained goroutines will be executed.
	//
	// This value will be ignored after the first task has been dispatched.
	Sustained int

	// Maximum is the maximum number of goroutines to spawn at any given time.
	//
	// If Maximum is <= 0, no maximum will be enforced.
	//
	// This value will be ignored after the first task has been dispatched.
	Maximum int

	// initOnce is used to ensure that the Runner is internally initialized
	// exactly once.
	initOnce sync.Once
	// workC is the Runner's work item channel.
	workC chan WorkItem
	// dispatchFinishedC is closed when our dispatch loop has completed. This will
	// happen after workC has closed and all outstanding dispatched work has
	// finished.
	dispatchFinishedC chan struct{}
}

// init initializes the starting state of the Runner. It must be called at the
// beginning of all exported methods.
func (r *Runner) init() {
	r.initOnce.Do(func() {
		r.workC = make(chan WorkItem)
		r.dispatchFinishedC = make(chan struct{})

		go r.dispatchLoop(r.Sustained, r.Maximum)
	})
}

// disaptchLoop is run in a goroutine. It reads tasks from workC and executes
// them.
func (r *Runner) dispatchLoop(sustained int, maximum int) {
	defer close(r.dispatchFinishedC)

	// If a Maximum is set, use Semaphore to enforce it.
	if maximum > 0 {
		spawnC := make(Semaphore, maximum)
		r.dispatchLoopBody(sustained, spawnC.Lock, spawnC.Unlock)
		spawnC.TakeAll()
	} else {
		// Unbounded number of goroutines. Use a WaitGroup to track them, and block
		// until all of the task goroutines have completed.
		var wg sync.WaitGroup
		r.dispatchLoopBody(sustained, func() { wg.Add(1) }, wg.Done)
		wg.Wait()
	}
}

func (r *Runner) dispatchLoopBody(sustained int, before, after func()) {
	numSustained := 0
	for {
		before()
		work, ok := <-r.workC
		if !ok {
			after()
			return
		}

		// Spawn a work goroutine.
		isSustained := numSustained < sustained
		if isSustained {
			numSustained++
		}

		go func() {
			defer after()

			// Execute the work that the outer loop pulled
			work.execute()

			// Sustained execution loop.
			if isSustained {
				for work := range r.workC {
					work.execute()
				}
			}
		}()
	}
}

// Close will instruct the Runner to not accept any more jobs and block until
// all current work is finished.
//
// Close may only be called once; additional calls will panic.
//
// The Runner's dispatch methods will panic if new work is dispatched after
// Close has been called.
func (r *Runner) Close() {
	r.init()

	close(r.workC)
	<-r.dispatchFinishedC
}

// Run executes a generator function, dispatching each generated task to the
// Runner. Run returns immediately with an error channel that can be used to
// reap the results of those tasks.
//
// The returned error channel must be consumed, or it can block additional
// functions from being run from gen. A common consumption function is
// errors.MultiErrorFromErrors, which will buffer all non-nil errors into an
// errors.MultiError. Other functions to consider are Must and Ignore (in this
// package).
//
// Note that there is no association between error channel's error order and
// the generated task order. However, the channel will return exactly one error
// result for each generated task.
//
// If the Runner has been closed, this will panic with a reference to the closed
// dispatch channel.
func (r *Runner) Run(gen func(chan<- func() error)) <-chan error {
	return r.runThen(gen, nil)
}

// runThen is a thin wrapper around Run that enables an after call function to
// be invoked when the generator has finished.
func (r *Runner) runThen(gen func(chan<- func() error), then func()) <-chan error {
	r.init()

	return runImpl(gen, r.workC, then)
}

// RunOne executes a single task in the Runner, returning with a channel that
// can be used to reap the result of that task.
//
// The returned error channel must be consumed, or it can block additional
// functions from being run from gen. A common consumption function is
// errors.MultiErrorFromErrors, which will buffer all non-nil errors into an
// errors.MultiError. Other functions to consider are Must and Ignore (in this
// package).
//
// If the Runner has been closed, this will panic with a reference to the closed
// dispatch channel.
func (r *Runner) RunOne(f func() error) <-chan error {
	r.init()

	errC := make(chan error)
	r.workC <- WorkItem{
		F:     f,
		ErrC:  errC,
		After: func() { close(errC) },
	}
	return errC
}

// WorkC returns a channel which WorkItem can be directly written to.
func (r *Runner) WorkC() chan<- WorkItem {
	r.init()
	return r.workC
}

// runImpl sets up a localized system where a generator generates tasks and
// dispatches them to the supplied work channel.
//
// After all tasks have been written to the work channel, then is called.
func runImpl(gen func(chan<- func() error), workC chan<- WorkItem, then func()) <-chan error {
	errC := make(chan error)
	taskC := make(chan func() error)

	// Execute our generator method.
	go func() {
		defer close(taskC)
		gen(taskC)
	}()

	// Read tasks from taskC and dispatch actual work.
	go func() {
		if then != nil {
			defer then()
		}

		// Use a counter to track the number of active jobs.
		//
		// Add one implicit job for the outer task loop. This will ensure that if
		// we will never hit 0 until all of our tasks have dispatched.
		count := int32(1)
		finish := func() {
			if atomic.AddInt32(&count, -1) == 0 {
				close(errC)
			}
		}
		defer finish()

		// Dispatch the tasks in the task channel.
		for task := range taskC {
			atomic.AddInt32(&count, 1)
			workC <- WorkItem{
				F:     task,
				ErrC:  errC,
				After: finish,
			}
		}
	}()

	return errC
}
