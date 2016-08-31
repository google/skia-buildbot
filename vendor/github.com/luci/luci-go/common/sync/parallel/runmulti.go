// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package parallel

import (
	"github.com/luci/luci-go/common/errors"

	"golang.org/x/net/context"
)

// RunMulti initiates a nested RunMulti operation. It invokes an entry function,
// passing it a MultiRunner instance bound to the supplied constraints. Any
// nested parallel operations scheduled through that MultiRunner will not
// starve each other regardless of execution order.
//
// This is useful when sharing the same outer Runner constraints with multiple
// tiers of parallel operations. A naive approach would be to re-use a Runner's
// WorkC() or Run() functions, but this can result in deadlock if the outer
// functions consume all available resources running their inner payloads,
// forcing their inner payloads to block forever.
//
// The supplied Context will be monitored for cancellation. If the Context is
// canceled, new work dispatch will be inhibited. Any methods added to the
// work channel will not be executed, and RunMulti will treat them as if they
// ran and immediately returned the Context's Err() value.
func RunMulti(c context.Context, workers int, fn func(MultiRunner) error) error {
	// Create a Runner to manage our goroutines. We will not set its Maximum,
	// since we will be metering that internally using our own semaphore.
	r := Runner{
		Sustained: workers,
	}
	defer r.Close()

	nrc := nestedRunnerContext{
		ctx:   c,
		workC: r.WorkC(),
	}
	if workers > 0 {
		// Create a semaphore with (workers-1) tokens. We subtract one because the
		// runner has an implicit token on virtue of running the work.
		nrc.sem = make(Semaphore, workers-1)
	}
	return fn(&nrc)
}

// MultiRunner can execute nested RunMulti against the same outer Runner.
type MultiRunner interface {
	// RunMulti runs the supplied generator, returning an errors.MultiError with
	// the task results.
	//
	// Since it blocks on result, RunMulti is safe to chain with other RunMulti
	// operations without risk of deadlock, as the caller's blocking counts as one
	// of the run tokens.
	//
	// Note that there is no association between the MultiError's error order and
	// the generated task order.
	RunMulti(func(chan<- func() error)) error
}

type nestedRunnerContext struct {
	ctx   context.Context
	workC chan<- WorkItem
	sem   Semaphore
}

func (nrc *nestedRunnerContext) RunMulti(gen func(chan<- func() error)) error {
	var (
		result    errors.MultiError
		doneC     = make(chan error)
		realWorkC = make(chan func() error)
	)
	defer close(doneC)

	// Call our task generator.
	go func() {
		defer close(realWorkC)
		gen(realWorkC)
	}()

	var (
		outstanding = 0
		contextErr  error

		// We will toggle these based what we want to block on.
		activeWorkC     = realWorkC
		activeSem       Semaphore
		newWorkC        = activeWorkC
		pendingWorkItem func() error
	)

	// Main dispatch control loop. Our goal is to have at least one task executing
	// at any given time. If we want to execute more, we must acquire a token from
	// the main semaphore.
	for activeWorkC != nil || outstanding > 0 {
		// Track whether we have a semaphore token. If we aren't using a semaphore,
		// we have an implicit token (unthrottled).
		hasToken := (nrc.sem == nil)

		select {
		case workItem, ok := <-newWorkC:
			// Incoming task.
			switch {
			case !ok:
				// Clear activeWorkC, instructing our select loop to stop accepting new
				// requests.
				activeWorkC = nil

			case contextErr != nil:
				// Ignore this request and pretend that it returned the Context error.
				result = append(result, contextErr)

			default:
				// Enqueue this request for dispatch.
				pendingWorkItem = workItem
			}

		case err := <-doneC:
			// A dispatched task has finished.
			if err != nil {
				result = append(result, err)
			}

			// Return one of our semaphore tokens if we have more than one outstanding
			// task.
			if outstanding > 1 {
				hasToken = true
			}
			outstanding--

		case <-nrc.ctx.Done():
			// Record our Context error. Future jobs will immediately fail with this
			// error.
			contextErr = nrc.ctx.Err()

		case activeSem <- SemaphoreToken{}:
			// We have a pending task, and we just acquired a semaphore token.
			hasToken = true
		}

		// If we have a pending task, maybe dispatch it.
		if pendingWorkItem != nil {
			// If we have no outstanding tasks, use "our" semaphore token to dispatch
			// this one immediately.
			//
			// If we have a token, use it immediately for this task.
			if outstanding == 0 || hasToken {
				nrc.workC <- WorkItem{
					F:    pendingWorkItem,
					ErrC: doneC,
				}

				outstanding++
				pendingWorkItem = nil
				hasToken = false
			}
		}

		// If we still have a token at this point, release it.
		if hasToken {
			nrc.sem.Unlock()
		}

		// Toggle blocking criteria.
		if pendingWorkItem == nil {
			// We have no currently-pending task, so pull a new one.
			newWorkC = activeWorkC

			// Don't try and acquire a semaphore token anymore, since we have nothing
			// to dispatch with it at the moment.
			activeSem = nil
		} else {
			// We have a pending task, but didn't dispatch, so we are blocking on
			// token acquisition.
			activeSem = nrc.sem

			// We only handle one pending task, so don't acquire any new tasks until
			// this one has been dispatched.
			newWorkC = nil
		}
	}

	// Choose our error response.
	if len(result) > 0 {
		return result
	}
	return nil
}
