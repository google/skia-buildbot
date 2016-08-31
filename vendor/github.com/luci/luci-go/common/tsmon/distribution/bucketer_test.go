// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package distribution

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestFixedWidthBucketer(t *testing.T) {
	Convey("Invalid values panic", t, func() {
		So(func() { FixedWidthBucketer(10, -1) }, ShouldPanic)
		So(func() { FixedWidthBucketer(-1, 1) }, ShouldPanic)
	})

	Convey("Zero size", t, func() {
		b := FixedWidthBucketer(10, 0)
		So(b.NumBuckets(), ShouldEqual, 2)
		So(b.Bucket(-100), ShouldEqual, 0)
		So(b.Bucket(-1), ShouldEqual, 0)
		So(b.Bucket(0), ShouldEqual, 1)
		So(b.Bucket(100), ShouldEqual, 1)
	})

	Convey("One size", t, func() {
		b := FixedWidthBucketer(10, 1)
		So(b.NumBuckets(), ShouldEqual, 3)
		So(b.Bucket(-100), ShouldEqual, 0)
		So(b.Bucket(-1), ShouldEqual, 0)
		So(b.Bucket(0), ShouldEqual, 1)
		So(b.Bucket(5), ShouldEqual, 1)
		So(b.Bucket(10), ShouldEqual, 2)
		So(b.Bucket(100), ShouldEqual, 2)
	})
}

func TestGeometricBucketer(t *testing.T) {
	Convey("Invalid values panic", t, func() {
		So(func() { GeometricBucketer(10, -1) }, ShouldPanic)
		So(func() { GeometricBucketer(-1, 10) }, ShouldPanic)
		So(func() { GeometricBucketer(0, 10) }, ShouldPanic)
		So(func() { GeometricBucketer(1, 10) }, ShouldPanic)
	})

	Convey("Zero size", t, func() {
		b := GeometricBucketer(10, 0)
		So(b.NumBuckets(), ShouldEqual, 2)
		So(b.Bucket(-100), ShouldEqual, 0)
		So(b.Bucket(-1), ShouldEqual, 0)
		So(b.Bucket(0), ShouldEqual, 1)
		So(b.Bucket(100), ShouldEqual, 1)
	})

	Convey("One size", t, func() {
		b := GeometricBucketer(4, 4)
		So(b.NumBuckets(), ShouldEqual, 6)
		So(b.Bucket(-100), ShouldEqual, 0)
		So(b.Bucket(-1), ShouldEqual, 0)
		So(b.Bucket(0), ShouldEqual, 1)
		So(b.Bucket(1), ShouldEqual, 2)
		So(b.Bucket(3), ShouldEqual, 2)
		So(b.Bucket(4), ShouldEqual, 3)
		So(b.Bucket(15), ShouldEqual, 3)
		So(b.Bucket(16), ShouldEqual, 4)
		So(b.Bucket(63), ShouldEqual, 4)
		So(b.Bucket(64), ShouldEqual, 5)
	})
}
