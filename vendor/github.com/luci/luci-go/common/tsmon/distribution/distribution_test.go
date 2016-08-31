// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package distribution

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestNew(t *testing.T) {
	Convey("Passing nil uses the default bucketer", t, func() {
		d := New(nil)
		So(d.Bucketer(), ShouldEqual, DefaultBucketer)
	})
}

func TestAdd(t *testing.T) {
	Convey("Add", t, func() {
		d := New(FixedWidthBucketer(10, 2))
		So(d.Sum(), ShouldEqual, 0)
		So(d.Count(), ShouldEqual, 0)

		d.Add(1)
		d.Add(10)
		d.Add(20)
		d.Add(30)
		So(d.Sum(), ShouldEqual, 61)
		So(d.Count(), ShouldEqual, 4)
	})
}
