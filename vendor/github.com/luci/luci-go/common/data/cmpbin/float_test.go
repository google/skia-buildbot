// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package cmpbin

import (
	"bytes"
	"io"
	"math/rand"
	"sort"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestFloats(t *testing.T) {
	t.Parallel()

	Convey("floats", t, func() {
		b := &bytes.Buffer{}

		Convey("good", func() {
			f1 := float64(1.234)
			n, err := WriteFloat64(b, f1)
			So(err, ShouldBeNil)
			So(n, ShouldEqual, 8)
			f, n, err := ReadFloat64(b)
			So(err, ShouldBeNil)
			So(n, ShouldEqual, 8)
			So(f, ShouldEqual, f1)
		})

		Convey("bad", func() {
			_, n, err := ReadFloat64(b)
			So(err, ShouldEqual, io.EOF)
			So(n, ShouldEqual, 0)
		})
	})
}

func TestFloatSortability(t *testing.T) {
	t.Parallel()

	Convey("floats maintain sort order", t, func() {
		vec := make(sort.Float64Slice, randomTestSize)
		r := rand.New(rand.NewSource(*seed))
		for i := range vec {
			vec[i] = r.Float64()
		}

		bin := make(sort.StringSlice, len(vec))
		b := &bytes.Buffer{}
		for i := range bin {
			b.Reset()
			n, err := WriteFloat64(b, vec[i])
			So(err, ShouldBeNil)
			So(n, ShouldEqual, 8)
			bin[i] = b.String()
		}

		vec.Sort()
		bin.Sort()

		for i := range vec {
			r, _, err := ReadFloat64(bytes.NewBufferString(bin[i]))
			So(err, ShouldBeNil)
			So(vec[i], ShouldEqual, r)
		}
	})
}
