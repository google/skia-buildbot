// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package errors

import (
	"errors"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestTransientError(t *testing.T) {
	t.Parallel()

	Convey(`A nil error`, t, func() {
		err := error(nil)

		Convey(`Is not transient.`, func() {
			So(IsTransient(err), ShouldBeFalse)
		})

		Convey(`Returns nil when wrapped with a transient error.`, func() {
			So(WrapTransient(err), ShouldBeNil)
		})
	})

	Convey(`An error`, t, func() {
		err := errors.New("test error")

		Convey(`Is not transient.`, func() {
			So(IsTransient(err), ShouldBeFalse)
		})
		Convey(`When wrapped with a transient error`, func() {
			terr := WrapTransient(err)
			Convey(`Has the same error string.`, func() {
				So(terr.Error(), ShouldEqual, "test error")
			})
			Convey(`Is transient.`, func() {
				So(IsTransient(terr), ShouldBeTrue)
			})
		})

		Convey(`A MultiError with a transient sub-error is transient.`, func() {
			So(IsTransient(MultiError{nil, WrapTransient(err), err}), ShouldBeTrue)
		})
	})
}
