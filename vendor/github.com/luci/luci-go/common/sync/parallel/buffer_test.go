// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package parallel

import (
	"sync/atomic"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestBuffer(t *testing.T) {
	t.Parallel()

	Convey(`A task Buffer`, t, func() {
		b := &Buffer{}
		defer func() {
			if b != nil {
				b.Close()
			}
		}()

		Convey(`With 10 maximum goroutines, will execute at most 10 simultaneous tasks.`, func() {
			const iters = 1000
			b.Maximum = 10

			called := int32(0)
			max := countMaxGoroutines(iters, 10, func(f func() error) {
				b.WorkC() <- WorkItem{F: func() error {
					atomic.AddInt32(&called, 1)
					return f()
				}}
			})

			So(max, ShouldEqual, 10)
			So(called, ShouldEqual, iters)
		})

		Convey(`Will buffer tasks indefinitely.`, func() {
			const iters = 1000
			b.Maximum = 10

			errC := make([]<-chan error, iters)
			unblockC := make(chan struct{})
			for i := range errC {
				i := i

				errC[i] = b.RunOne(func() error {
					<-unblockC
					return numberError(i)
				})
			}

			// All of the tasks are currently dispatched and blocking on unblockC.
			// Unlock them all and read the resulting errors.
			close(unblockC)

			errs := make([]bool, iters)
			for _, c := range errC {
				errs[int((<-c).(numberError))] = true
			}

			tc := 0
			for _, v := range errs {
				if v {
					tc++
				}
			}
			So(tc, ShouldEqual, iters)
		})

		Convey(`Can buffer tasks faster than they are reaped via Run.`, func() {
			const iters = 1000
			b.Maximum = 10

			errC := b.Run(func(taskC chan<- func() error) {
				for i := 0; i < iters; i++ {
					i := i
					taskC <- func() error {
						return numberError(i)
					}
				}
			})

			// No errors have been reaped yet, so only b.Maximum tasks should have been
			// dispatched, and they will be blocked on sending to errC. Unblock all of
			// the tasks and confirm that error collection works.
			seen := make(map[int]struct{}, iters)
			for err := range errC {
				seen[int(err.(numberError))] = struct{}{}
			}
			So(len(seen), ShouldEqual, iters)
		})

		Convey(`Has proper task dispatch order`, func() {
			const iters = 10

			// We use a Maximum of 1 to control task execution order. There should be
			// no data races.
			b.Maximum = 1

			account := func(errC <-chan error) []int {
				var order []int
				for err := range errC {
					if err != nil {
						order = append(order, int(err.(numberError)))
					}
				}
				return order
			}
			gen := func(taskC chan<- func() error) {
				for i := 0; i < iters; i++ {
					i := i
					taskC <- func() error {
						return numberError(i)
					}
				}
			}

			// Start with two buffered tasks to fill our work channels so our buffer
			// empty order is deterministic.
			startC0 := b.RunOne(func() error { return nil })
			startC1 := b.RunOne(func() error { return nil })
			release := func() {
				<-startC0
				<-startC1
			}

			Convey(`Is FIFO by default.`, func() {
				So(account(b.runThen(gen, release)), ShouldResemble, []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9})
			})

			Convey(`Will be LIFO if LIFO is set.`, func() {
				b.SetFIFO(false)
				So(account(b.runThen(gen, release)), ShouldResemble, []int{9, 8, 7, 6, 5, 4, 3, 2, 1, 0})
			})
		})

		Convey(`Will finish tasks if closed while some are pending.`, func() {
			const iters = 1000
			b.Maximum = 10

			// Buffer iters tests. Each will block pending a signal. Since
			// iters > b.Maximum, this ensures that some tasks remain in our Buffer
			// which, in turn, ensures that when we Close the Buffer, it will not be
			// empty.
			workCount := int32(0)
			unblockC := make(chan struct{})
			finishedC := make(chan struct{})
			for i := 0; i < iters; i++ {
				b.WorkC() <- WorkItem{F: func() error {
					<-unblockC
					atomic.AddInt32(&workCount, 1)
					return nil
				}, After: func() {
					if finishedC != nil {
						finishedC <- struct{}{}
					}
				}}
			}

			// First stage: fully execute one of the tasks.
			unblockC <- struct{}{}
			<-finishedC

			// Second stage: Close our Buffer, then unblock the remainder. We
			// synchronize this by waiting for Buffer's internal workC to close. This
			// is okay, since we're not sending more work through it.
			go func() {
				<-b.workC

				finishedC = nil
				close(unblockC)
			}()

			b.Close()
			b = nil // So our test doesn't double-close.

			So(workCount, ShouldEqual, iters)
		})
	})
}
