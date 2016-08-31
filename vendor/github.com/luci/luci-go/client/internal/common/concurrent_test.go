// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package common

import (
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/maruel/ut"
)

func ExampleNewGoroutinePool() {
	pool := NewGoroutinePool(2, NewCanceler())
	for _, s := range []string{"knock!", "knock!"} {
		s := s // Create a new s for closure.
		pool.Schedule(func() { fmt.Print(s) }, nil)
	}
	if pool.Wait() == nil {
		fmt.Printf("\n")
	}
	pool.Cancel(errors.New("pool is no more"))
	pool.Schedule(func() {}, func() {
		fmt.Printf("canceled because %s\n", pool.CancelationReason())
	})
	err := pool.Wait()
	fmt.Printf("all jobs either executed or canceled (%s)\n", err)
	// Output:
	// knock!knock!
	// canceled because pool is no more
	// all jobs either executed or canceled (pool is no more)
}

type newPoolFunc func(p int) GoroutinePool

func TestGoroutinePool(t *testing.T) {
	testGoroutinePool(t, func(p int) GoroutinePool {
		return NewGoroutinePool(p, NewCanceler())
	})
}

func testGoroutinePool(t *testing.T, newPool newPoolFunc) {
	t.Parallel()

	const MAX = 10
	const J = 200
	pool := newPool(MAX)
	logs := make(chan int)
	for i := 1; i <= J; i++ {
		i := i
		pool.Schedule(func() {
			logs <- -i
			time.Sleep(time.Millisecond)
			logs <- i
		}, nil)
	}
	var fail error
	go func() {
		defer close(logs)
		fail = pool.Wait()
	}()
	currentJobs := map[int]bool{}
	doneJobs := map[int]bool{}
	for j := range logs {
		if j < 0 {
			t.Logf("Job %d started, %d others running", -j, len(currentJobs))
			ut.AssertEqual(t, true, len(currentJobs) < MAX)
			currentJobs[-j] = true
		} else {
			t.Logf("Job %d ended", j)
			delete(currentJobs, j)
			doneJobs[j] = true
		}
	}
	ut.AssertEqual(t, 0, len(currentJobs))
	ut.AssertEqual(t, J, len(doneJobs))
	ut.AssertEqual(t, nil, fail)
}

func TestGoroutinePoolCancel(t *testing.T) {
	testGoroutinePoolCancel(t, func(p int) GoroutinePool {
		return NewGoroutinePool(p, NewCanceler())
	})
}

func testGoroutinePoolCancel(t *testing.T, newPool newPoolFunc) {
	t.Parallel()

	cancelError := errors.New("cancelError")
	const MAX = 10
	const J = 11 * MAX
	pool := newPool(MAX)
	logs := make(chan int, 2*J) // Avoid job blocking when writing to log.
	for i := 1; i <= J; i++ {
		i := i
		pool.Schedule(func() {
			if i == 1 {
				pool.Cancel(cancelError)
			}
			logs <- i
		}, func() {
			// Push negative on canceled tasks.
			logs <- -i
		})
	}
	var wait1 error
	var wait2 error
	go func() {
		defer close(logs)
		wait1 = pool.Wait()
		// Schedule more jobs, all of which must be cancelled.
		for i := J + 1; i <= 2*J; i++ {
			i := i
			pool.Schedule(func() {
				logs <- i
			}, func() {
				logs <- -i
			})
		}
		wait2 = pool.Wait()
	}()
	// At most MAX-1 could have been executing concurrently with job[i=1]
	// after it had cancelled the pool and before it wrote to log.
	// No new job should have started after that. So, log must have at most MAX-1
	// jobs after 1.
	finishedPre := 0
	finishedAfter := 0
	canceled := 0
	for i := range logs {
		if i == 1 {
			finishedAfter++
			break
		}
		finishedPre++
	}
	for i := range logs {
		ut.AssertEqual(t, true, i <= J)
		if i > 0 {
			finishedAfter++
		} else {
			canceled++
		}
	}
	t.Logf("JOBS: Total %d Pre %d After %d Cancelled %d", 2*J, finishedPre, finishedAfter, canceled)
	ut.AssertEqual(t, 2*J, finishedPre+finishedAfter+canceled)
	ut.AssertEqualf(t, true, finishedAfter < MAX,
		"%d (>=%d MAX) jobs started after cancellation.", finishedAfter, MAX)
	ut.AssertEqualf(t, true, canceled >= J, "%d > %d", canceled, J)
	ut.AssertEqual(t, cancelError, wait1)
	ut.AssertEqual(t, cancelError, wait2)
}

func TestGoroutinePoolCancelFuncCalled(t *testing.T) {
	testGoroutinePoolCancelFuncCalled(t, func(p int) GoroutinePool {
		return NewGoroutinePool(p, NewCanceler())
	})
}

func testGoroutinePoolCancelFuncCalled(t *testing.T, newPool newPoolFunc) {
	t.Parallel()

	cancelError := errors.New("cancelError")
	// Simulate deterministically when the semaphore returs immediately
	// because of cancelation, as opposed to actually having a resource.
	pipe := make(chan string, 2)
	logs := make(chan string, 3)
	slow := func() {
		// Get 2 item to make sure GoroutinePool can't schedule new job.
		item := <-pipe
		logs <- item
	}
	pool := newPool(1)
	pool.Schedule(slow, slow)
	// This job would have to wait for slow to finish,
	// but slow is waiting for channel to have something.
	// This is sort of a deadlock.
	pool.Schedule(func() { pipe <- "job" }, func() { pipe <- "onCancel" })
	pool.Cancel(cancelError)
	// Canceling should result in onCancel of last job.
	// In case it's a bug, don't wait forever for slow, but unblock slow().
	pipe <- "unblock"
	ut.AssertEqual(t, cancelError, pool.Wait())
	close(pipe)
	if pipeItem, ok := <-pipe; ok {
		logs <- pipeItem
	}
	close(logs)
	for l := range logs {
		if l == "onCancel" {
			return
		}
	}
	t.Fatalf("onCancel wasn't called.")
}

// Purpose: re-use GoroutinePool tests for GoroutinePriorityPool.
type goroutinePriorityPoolforTest struct {
	GoroutinePriorityPool
}

func (c *goroutinePriorityPoolforTest) Schedule(job func(), onCanceled func()) {
	c.GoroutinePriorityPool.Schedule(0, job, onCanceled)
}

func TestGoroutinePriorityPool(t *testing.T) {
	testGoroutinePool(t, func(p int) GoroutinePool {
		return &goroutinePriorityPoolforTest{NewGoroutinePriorityPool(p, NewCanceler())}
	})
}

func TestGoroutinePriorityPoolCancel(t *testing.T) {
	testGoroutinePoolCancel(t, func(p int) GoroutinePool {
		return &goroutinePriorityPoolforTest{NewGoroutinePriorityPool(p, NewCanceler())}
	})
}

func TestGoroutinePriorityPoolCancelFuncCalled(t *testing.T) {
	testGoroutinePoolCancelFuncCalled(t, func(p int) GoroutinePool {
		return &goroutinePriorityPoolforTest{NewGoroutinePriorityPool(p, NewCanceler())}
	})
}

func TestGoroutinePriorityPoolWithPriority(t *testing.T) {
	t.Parallel()

	const maxPriorities = 15
	pool := NewGoroutinePriorityPool(1, NewCanceler())
	logs := make(chan int)
	wg := sync.WaitGroup{}
	for i := 0; i < maxPriorities; i++ {
		wg.Add(1)
		i := i
		go func() {
			pool.Schedule(int64(i), func() { logs <- i }, nil)
			wg.Done()
		}()
	}
	wg.Wait()
	var fail error
	go func() {
		defer close(logs)
		fail = pool.Wait()
		ut.ExpectEqual(t, nil, fail)
	}()
	doneJobs := make([]bool, maxPriorities)
	// First job can be any, the rest must be in order.
	prio := <-logs
	doneJobs[prio] = true
	for prio := range logs {
		ut.AssertEqual(t, false, doneJobs[prio])
		doneJobs[prio] = true
		// All higher priority jobs must be finished.
		for before := 0; before < prio; before++ {
			ut.AssertEqual(t, true, doneJobs[before])
		}
	}
	ut.AssertEqual(t, maxPriorities, len(doneJobs))
	for p, d := range doneJobs {
		ut.AssertEqualIndex(t, p, true, d)
	}
}

func assertClosed(t *testing.T, c *canceler) {
	// Both channels are unbuffered, so there should be at most one value.
	<-c.channel
	select {
	case _, ok := <-c.channel:
		ut.AssertEqual(t, false, ok)
	default:
		t.Fatalf("channel is not closed")
	}

	<-c.closeChannel
	select {
	case _, ok := <-c.closeChannel:
		ut.AssertEqual(t, false, ok)
	default:
		t.Fatalf("channel is not closed")
	}
}

func TestCancelable(t *testing.T) {
	t.Parallel()
	c := newCanceler()
	ut.AssertEqual(t, nil, c.CancelationReason())
	select {
	case <-c.Channel():
		t.FailNow()
	default:
	}

	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		select {
		case err, isCanceled := <-c.Channel():
			ut.ExpectEqualf(t, true, isCanceled, "Closed, but shouldn't be.")
			ut.ExpectEqual(t, ErrCanceled, err)
		}
	}()
	c.Cancel(nil)
	ut.AssertEqual(t, ErrCanceled, c.CancelationReason())
	t.Log("waiting for goroutine above to end.")
	wg.Wait()
	ut.AssertEqual(t, nil, c.Close())
	assertClosed(t, c)
}

func TestCancelableDoubleCancel(t *testing.T) {
	t.Parallel()
	errReason := errors.New("reason")
	errWhatever := errors.New("whatever")
	c := newCanceler()
	c.Cancel(errReason)
	ut.AssertEqual(t, errReason, c.CancelationReason())
	c.Cancel(errWhatever)
	ut.AssertEqual(t, errReason, c.CancelationReason())
	ut.AssertEqual(t, nil, c.Close())
}

func TestCancelableDoubleCloseAndCancel(t *testing.T) {
	t.Parallel()
	errReason := errors.New("reason")
	c := newCanceler()
	ut.AssertEqual(t, nil, c.Close())
	ut.AssertEqual(t, nil, c.Close())
	c.Cancel(errReason)
	ut.AssertEqual(t, errReason, c.CancelationReason())
}

func TestCancelableUnblockAfterClosed(t *testing.T) {
	t.Parallel()
	c := newCanceler()
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		select {
		case _, stillOpen := <-c.Channel():
			ut.ExpectEqual(t, false, stillOpen)
		}
	}()
	ut.AssertEqual(t, nil, c.Close())
	t.Log("waiting for goroutine above to end.")
	wg.Wait()
	assertClosed(t, c)
}
