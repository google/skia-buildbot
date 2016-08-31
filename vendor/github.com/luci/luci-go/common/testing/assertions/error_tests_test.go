// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package assertions

import (
	"errors"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

type customError struct{}

func (customError) Error() string { return "customError noob" }

func TestShouldErrLike(t *testing.T) {
	t.Parallel()

	ce := customError{}
	e := errors.New("e is for error")

	Convey("Test ShouldErrLike", t, func() {
		Convey("too many params", func() {
			So(ShouldErrLike(nil, nil, nil), ShouldContainSubstring, "requires 0 or 1")
		})
		Convey("no expectation", func() {
			So(ShouldErrLike(nil), ShouldEqual, "")
			So(ShouldErrLike(e), ShouldContainSubstring, "Expected: nil")
			So(ShouldErrLike(ce), ShouldContainSubstring, "Expected: nil")
		})
		Convey("nil expectation", func() {
			So(ShouldErrLike(nil, nil), ShouldEqual, "")
			So(ShouldErrLike(e, nil), ShouldContainSubstring, "Expected: nil")
			So(ShouldErrLike(ce, nil), ShouldContainSubstring, "Expected: nil")
		})
		Convey("nil actual", func() {
			So(ShouldErrLike(nil, "wut"), ShouldContainSubstring, "Expected '<nil>' to NOT be nil")
		})
		Convey("not an err", func() {
			So(ShouldErrLike(100, "wut"), ShouldContainSubstring, "Expected: 'error interface support'")
		})
		Convey("string actual", func() {
			So(ShouldErrLike(e, "is for error"), ShouldEqual, "")
			So(ShouldErrLike(ce, "customError"), ShouldEqual, "")
		})
		Convey("error actual", func() {
			So(ShouldErrLike(e, e), ShouldEqual, "")
			So(ShouldErrLike(ce, ce), ShouldEqual, "")
		})
		Convey("bad expected type", func() {
			So(ShouldErrLike(e, 20), ShouldContainSubstring, "unknown argument type int")
		})
	})
}
