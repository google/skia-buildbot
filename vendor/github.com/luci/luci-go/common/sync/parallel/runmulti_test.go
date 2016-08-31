// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package parallel

import (
	"sort"
	"testing"

	"golang.org/x/net/context"

	"github.com/luci/luci-go/common/errors"
	. "github.com/smartystreets/goconvey/convey"
)

func TestRunMulti(t *testing.T) {
	t.Parallel()

	Convey(`A RunMulti operation with two workers can be nested without deadlock.`, t, func() {
		const n = 2
		const inner = 128

		// This will hand out "n" tokens, then close "tokensOutC".
		tokenC := make(chan struct{})
		tokensOutC := make(chan struct{})
		go func() {
			defer close(tokensOutC)

			for i := 0; i < n; i++ {
				tokenC <- struct{}{}
			}
		}()

		err := RunMulti(context.Background(), n, func(mr MultiRunner) error {
			return mr.RunMulti(func(workC chan<- func() error) {
				// Dispatch n top-level dispatchers and block until they are both
				// executed. This will consume the total number of workers. In a normal
				// Runner, this would prevent the top-level dispatchers' dispatched
				// routines from running.
				for i := 0; i < n; i++ {
					i := i
					workC <- func() error {
						// Take one token.
						<-tokenC

						// Wait until all of the tokens have been taken.
						<-tokensOutC

						// Dispatch a bunch of sub-work.
						return mr.RunMulti(func(workC chan<- func() error) {
							for j := 0; j < inner; j++ {
								index := (i * inner) + j
								workC <- func() error { return numberError(index) }
							}
						})
					}
				}
			})
		})

		// Flatten our "n" top-level MultiErrors together.
		So(err, ShouldHaveSameTypeAs, (errors.MultiError)(nil))
		aggregateErr := make(errors.MultiError, 0, (n * inner))
		for _, ierr := range err.(errors.MultiError) {
			So(ierr, ShouldHaveSameTypeAs, (errors.MultiError)(nil))
			aggregateErr = append(aggregateErr, ierr.(errors.MultiError)...)
		}
		So(aggregateErr, ShouldHaveLength, (n * inner))

		// Make sure all of the error values that we expect are present.
		actual := make([]int, len(aggregateErr))
		expected := make([]int, len(aggregateErr))
		for i := 0; i < len(aggregateErr); i++ {
			actual[i] = int(aggregateErr[i].(numberError))
			expected[i] = i
		}
		sort.Ints(actual)
		So(actual, ShouldResemble, expected)
	})

	Convey(`A RunMulti operation will stop executing jobs if its Context is canceled.`, t, func() {
		const n = 128
		const cancelPoint = 16

		c, cancelFunc := context.WithCancel(context.Background())
		err := RunMulti(c, 1, func(mr MultiRunner) error {
			return mr.RunMulti(func(workC chan<- func() error) {
				for i := 0; i < n; i++ {
					i := i

					if i == cancelPoint {
						// This and all future work should not be dispatched. Our previous
						// work item *may* execute depending on whether it was dispatched
						// before or after the cancel was processed.
						cancelFunc()
					}

					workC <- func() error { return nil }
				}
			})
		})

		// We should have somewhere between (n-cancelPoint-1) and (n-cancelPoint)
		// context errors.
		So(err, ShouldHaveSameTypeAs, (errors.MultiError)(nil))
		So(len(err.(errors.MultiError)), ShouldBeBetweenOrEqual, n-cancelPoint-1, n-cancelPoint)
	})

	Convey(`A RunMulti operation with no worker limit will not be constrained.`, t, func() {
		const n = 128

		// This will hand out "n" tokens, then close "tokensOutC".
		tokenC := make(chan struct{})
		tokensOutC := make(chan struct{})
		go func() {
			defer close(tokensOutC)

			for i := 0; i < n; i++ {
				tokenC <- struct{}{}
			}
		}()

		err := RunMulti(context.Background(), 0, func(mr MultiRunner) error {
			// This will deadlock if all n workers can't run simultaneously.
			return mr.RunMulti(func(workC chan<- func() error) {
				for i := 0; i < n; i++ {
					workC <- func() error {
						<-tokenC
						<-tokensOutC
						return nil
					}
				}
			})
		})
		So(err, ShouldBeNil)
	})
}
