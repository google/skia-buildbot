// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package errors

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestFilter(t *testing.T) {
	t.Parallel()

	aerr := New("test error A")
	berr := New("test error B")

	Convey(`Filter works.`, t, func() {
		So(Filter(nil, nil), ShouldEqual, nil)
		So(Filter(nil, nil, aerr, berr), ShouldEqual, nil)
		So(Filter(aerr, nil), ShouldEqual, aerr)
		So(Filter(aerr, nil, aerr, berr), ShouldEqual, nil)
		So(Filter(aerr, berr), ShouldEqual, aerr)
		So(Filter(MultiError{aerr, berr}, berr), ShouldResemble, MultiError{aerr, nil})
		So(Filter(MultiError{aerr, aerr}, aerr), ShouldEqual, nil)
		So(Filter(MultiError{MultiError{aerr, aerr}, aerr}, aerr), ShouldEqual, nil)
	})

	Convey(`FilterFunc works.`, t, func() {
		So(FilterFunc(nil, func(error) bool { return false }), ShouldEqual, nil)
		So(FilterFunc(aerr, func(error) bool { return true }), ShouldEqual, nil)
		So(FilterFunc(aerr, func(error) bool { return false }), ShouldEqual, aerr)

		// Make sure MultiError gets evaluated before its contents do.
		So(FilterFunc(MultiError{aerr, berr}, func(e error) bool {
			if me, ok := e.(MultiError); ok {
				return len(me) == 2
			}
			return false
		}), ShouldEqual, nil)
	})
}
