// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package errors

import (
	"errors"
	"fmt"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestWalk(t *testing.T) {
	t.Parallel()

	Convey(`Testing the Walk function`, t, func() {
		count := 0
		keepWalking := true
		walkFn := func(err error) bool {
			count++
			return keepWalking
		}

		Convey(`Will not walk at all for a nil error.`, func() {
			Walk(nil, walkFn)
			So(count, ShouldEqual, 0)
		})

		Convey(`Will fully traverse a wrapped MultiError.`, func() {
			Walk(MultiError{nil, testWrap(New("sup")), nil}, walkFn)
			So(count, ShouldEqual, 3)
		})

		Convey(`Will unwrap a Wrapped error.`, func() {
			Walk(testWrap(New("sup")), walkFn)
			So(count, ShouldEqual, 2)
		})

		Convey(`Will short-circuit if the walk funciton returns false.`, func() {
			keepWalking = false
			Walk(testWrap(New("sup")), walkFn)
			So(count, ShouldEqual, 1)
		})
	})
}

func TestAny(t *testing.T) {
	t.Parallel()

	Convey(`Testing the Any function`, t, func() {
		testErr := errors.New("test error")
		filter := func(err error) bool { return err == testErr }

		for _, err := range []error{
			nil,
			Reason("error test: foo").Err(),
			errors.New("other error"),
		} {
			Convey(fmt.Sprintf(`Registers false for %T %v`, err, err), func() {
				So(Any(err, filter), ShouldBeFalse)
			})
		}

		for _, err := range []error{
			testErr,
			MultiError{errors.New("other error"), MultiError{testErr, nil}},
			Annotate(testErr).Reason("error test").Err(),
			WrapTransient(MultiError{WrapTransient(testErr), nil, errors.New("other error")}),
		} {
			Convey(fmt.Sprintf(`Registers true for %T %v`, err, err), func() {
				So(Any(err, filter), ShouldBeTrue)
			})
		}
	})
}

func TestContains(t *testing.T) {
	t.Parallel()

	Convey(`Testing the Contains function for a sentinel error`, t, func() {
		sentinel := errors.New("test error")

		for _, err := range []error{
			nil,
			errors.New("foo"),
			errors.New("test error"),
		} {
			Convey(fmt.Sprintf(`Registers false for %T %v`, err, err), func() {
				So(Contains(err, sentinel), ShouldBeFalse)
			})
		}

		for _, err := range []error{
			sentinel,
			MultiError{errors.New("other error"), MultiError{sentinel, nil}},
			WrapTransient(MultiError{WrapTransient(sentinel), nil, errors.New("other error")}),
		} {
			Convey(fmt.Sprintf(`Registers true for %T %v`, err, err), func() {
				So(Contains(err, sentinel), ShouldBeTrue)
			})
		}
	})
}
