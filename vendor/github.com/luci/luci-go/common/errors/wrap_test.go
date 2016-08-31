// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package errors

import (
	"fmt"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

type testWrapped struct {
	error
}

func (w *testWrapped) Error() string {
	if w.error == nil {
		return "wrapped: nil"
	}
	return fmt.Sprintf("wrapped: %v", w.error.Error())
}

func (w *testWrapped) InnerError() error {
	return w.error
}

func testWrap(err error) error {
	return &testWrapped{err}
}

func TestWrapped(t *testing.T) {
	t.Parallel()

	Convey(`Test Wrapped`, t, func() {
		Convey(`A nil error`, func() {
			var err error

			Convey(`Unwraps to nil.`, func() {
				So(Unwrap(err), ShouldBeNil)
			})

			Convey(`When wrapped, does not unwrap to nil.`, func() {
				So(Unwrap(testWrap(err)), ShouldNotBeNil)
			})
		})

		Convey(`A non-wrapped error.`, func() {
			err := New("test error")

			Convey(`Unwraps to itself.`, func() {
				So(Unwrap(err), ShouldEqual, err)
			})

			Convey(`When wrapped, unwraps to itself.`, func() {
				So(Unwrap(testWrap(err)), ShouldEqual, err)
			})

			Convey(`When double-wrapped, unwraps to itself.`, func() {
				So(Unwrap(testWrap(testWrap(err))), ShouldEqual, err)
			})
		})
	})
}
