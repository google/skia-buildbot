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

type otherMEType []error

func (o otherMEType) Error() string { return "FAIL" }

func TestMultiError(t *testing.T) {
	t.Parallel()

	Convey("MultiError works", t, func() {
		var me error = MultiError{fmt.Errorf("hello"), fmt.Errorf("bob")}

		So(me.Error(), ShouldEqual, `hello (and 1 other error)`)

		Convey("MultiErrorFromErrors with errors works", func() {
			mec := make(chan error, 4)
			mec <- nil
			mec <- fmt.Errorf("first error")
			mec <- nil
			mec <- fmt.Errorf("what")
			close(mec)

			err := MultiErrorFromErrors(mec)
			So(err.Error(), ShouldEqual, `first error (and 1 other error)`)
		})

		Convey("MultiErrorFromErrors with nil works", func() {
			So(MultiErrorFromErrors(nil), ShouldBeNil)

			c := make(chan error)
			close(c)
			So(MultiErrorFromErrors(c), ShouldBeNil)

			c = make(chan error, 2)
			c <- nil
			c <- nil
			close(c)
			So(MultiErrorFromErrors(c), ShouldBeNil)
		})
	})
}

func TestUpstreamErrors(t *testing.T) {
	t.Parallel()

	Convey("Test MultiError", t, func() {
		Convey("nil", func() {
			me := MultiError(nil)
			So(me.Error(), ShouldEqual, "(0 errors)")
			Convey("single", func() {
				So(SingleError(error(me)), ShouldBeNil)
			})
		})
		Convey("one", func() {
			me := MultiError{errors.New("sup")}
			So(me.Error(), ShouldEqual, "sup")
		})
		Convey("two", func() {
			me := MultiError{errors.New("sup"), errors.New("what")}
			So(me.Error(), ShouldEqual, "sup (and 1 other error)")
		})
		Convey("more", func() {
			me := MultiError{errors.New("sup"), errors.New("what"), errors.New("nerds")}
			So(me.Error(), ShouldEqual, "sup (and 2 other errors)")

			Convey("single", func() {
				So(SingleError(error(me)), ShouldResemble, errors.New("sup"))
			})
		})
	})

	Convey("SingleError passes through", t, func() {
		e := errors.New("unique")
		So(SingleError(e), ShouldEqual, e)
	})

	Convey("Test MultiError Conversion", t, func() {
		ome := otherMEType{errors.New("sup")}
		So(Fix(ome), ShouldHaveSameTypeAs, MultiError{})
	})

	Convey("Fix passes through", t, func() {
		e := errors.New("unique")
		So(Fix(e), ShouldEqual, e)
	})
}

func ExampleMultiError() {
	errCh := make(chan error, 10)
	errCh <- nil // nils are ignored
	errCh <- fmt.Errorf("what")
	close(errCh)

	err := MultiErrorFromErrors(errCh)
	fmt.Printf("got: %s len=%d\n", err, len(err.(MultiError)))

	errCh = make(chan error, 10)
	errCh <- nil // and if the channel only has nils
	close(errCh)

	err = MultiErrorFromErrors(errCh) // then it returns nil
	fmt.Printf("got: %v\n", err)

	// Output:
	// got: what len=1
	// got: <nil>
}
