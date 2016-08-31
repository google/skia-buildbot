// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package parallel

import (
	"errors"
	"fmt"
	"sync/atomic"
	"testing"

	. "github.com/luci/luci-go/common/testing/assertions"
	. "github.com/smartystreets/goconvey/convey"
)

type numberError int

func (e numberError) Error() string {
	return fmt.Sprintf("#%d", e)
}

func TestRunner(t *testing.T) {
	t.Parallel()

	Convey("When using a Runner directly", t, func() {
		r := &Runner{}
		defer func() {
			if r != nil {
				r.Close()
			}
		}()

		Convey(`Can schedule individual tasks.`, func() {
			const iters = 100

			ac := int32(0)
			resultC := make(chan int)

			// Dispatch iters tasks.
			for i := 0; i < iters; i++ {
				i := i

				errC := r.RunOne(func() error {
					atomic.AddInt32(&ac, int32(i))
					return numberError(i)
				})

				// Reap errC.
				go func() {
					resultC <- int((<-errC).(numberError))
				}()
			}

			// Reap the results and compare.
			result := 0
			for i := 0; i < iters; i++ {
				result += <-resultC
			}
			So(result, ShouldEqual, atomic.LoadInt32(&ac))
		})

		Convey(`Can use WorkC directly in a bidirectional select loop.`, func() {
			// Generate a function that writes a value to "outC".
			outC := make(chan int)
			valueWriter := func(v int) func() error {
				return func() error {
					outC <- v
					return nil
				}
			}

			// We will repeatedly dispatch tasks and reap their results in the same
			// select loop.
			//
			// remaining is the total number of tasks to dispatch. dispatch controls
			// the number of simultaneous tasks that we're willing to send at a time.
			// count is the number of tasks that have completed.
			remaining := 1000
			count := 0
			dispatch := 10
			wc := r.WorkC()

			for count < 1000 {
				select {
				case wc <- WorkItem{F: valueWriter(1)}:
					dispatch--
					remaining--
					break

				case v := <-outC:
					count += v
					dispatch++
				}

				if dispatch == 0 || remaining == 0 {
					// Stop writing work items until we reap some results.
					wc = nil
				} else {
					// We have results, start writing work items again.
					wc = r.WorkC()
				}
			}

			So(count, ShouldEqual, 1000)
		})

		Convey(`A WorkItem's After method can recover from a panic.`, func() {
			testErr := errors.New("test error")

			var err error
			r.WorkC() <- WorkItem{
				F: func() error {
					panic(testErr)
				},
				After: func() {
					if r := recover(); r != nil {
						err = r.(error)
					}
				},
			}

			r.Close()
			r = nil // Do we don't close it in a defer.

			So(err, ShouldEqual, testErr)
		})

		Convey("Ignore consumes the errors and blocks", func() {
			count := new(int32)
			Ignore(r.Run(func(ch chan<- func() error) {
				for i := 0; i < 100; i++ {
					ch <- func() error {
						atomic.AddInt32(count, 1)
						return fmt.Errorf("whaaattt")
					}
				}
			}))
			So(*count, ShouldEqual, 100)
		})

		Convey("Must panics on the first error", func() {
			r.Maximum = 1
			count := new(int32)

			// Reap errC at the end, since Must will panic without consuming its
			// contents.
			var errC <-chan error
			defer func() {
				if errC != nil {
					for range errC {
					}
				}
			}()

			So(func() {
				errC = r.Run(func(ch chan<- func() error) {
					for i := 0; i < 100; i++ {
						i := i
						ch <- func() error {
							atomic.AddInt32(count, 1)
							return fmt.Errorf("whaaattt: %d", i)
						}
					}
				})
				Must(errC)
			}, ShouldPanicLike, "whaaattt: 0")
			// Either:
			//   * the panic happened and we load count before ch is unblocked
			//   * the panic happened then ch(1) pushes and runs, then we load count
			// So count will either be 1 or 2, but never more or less.
			So(atomic.LoadInt32(count), ShouldBeBetweenOrEqual, 1, 2)
		})
	})
}
