// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package iotools

import (
	"io"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestByteSliceReader(t *testing.T) {
	Convey(`A ByteSliceReader for {0x60, 0x0d, 0xd0, 0x65}`, t, func() {
		data := []byte{0x60, 0x0d, 0xd0, 0x65}
		bsd := ByteSliceReader(data)

		Convey(`Can read byte-by-byte.`, func() {
			b, err := bsd.ReadByte()
			So(b, ShouldEqual, 0x60)
			So(err, ShouldBeNil)

			b, err = bsd.ReadByte()
			So(b, ShouldEqual, 0x0d)
			So(err, ShouldBeNil)

			b, err = bsd.ReadByte()
			So(b, ShouldEqual, 0xd0)
			So(err, ShouldBeNil)

			b, err = bsd.ReadByte()
			So(b, ShouldEqual, 0x65)
			So(err, ShouldBeNil)

			b, err = bsd.ReadByte()
			So(err, ShouldEqual, io.EOF)
		})

		Convey(`Can read the full array.`, func() {
			buf := make([]byte, 4)
			count, err := bsd.Read(buf)
			So(count, ShouldEqual, 4)
			So(err, ShouldBeNil)
			So(buf, ShouldResemble, data)
		})

		Convey(`When read into an oversized buf, returns io.EOF and setting the slice to nil.`, func() {
			buf := make([]byte, 16)
			count, err := bsd.Read(buf)
			So(count, ShouldEqual, 4)
			So(err, ShouldEqual, io.EOF)
			So(buf[:count], ShouldResemble, data)
			So([]byte(bsd), ShouldBeNil)
		})

		Convey(`When read in two 3-byte parts, the latter returns io.EOF and sets the slice to nil.`, func() {
			buf := make([]byte, 3)

			count, err := bsd.Read(buf)
			So(count, ShouldEqual, 3)
			So(err, ShouldBeNil)
			So(buf, ShouldResemble, data[:3])
			So([]byte(bsd), ShouldResemble, data[3:])

			count, err = bsd.Read(buf)
			So(count, ShouldEqual, 1)
			So(err, ShouldEqual, io.EOF)
			So(buf[:count], ShouldResemble, data[3:])
			So([]byte(bsd), ShouldBeNil)
		})
	})

	Convey(`A ByteSliceReader for nil`, t, func() {
		bsd := ByteSliceReader(nil)

		Convey(`A byte read will return io.EOF.`, func() {
			_, err := bsd.ReadByte()
			So(err, ShouldEqual, io.EOF)
		})
	})

	Convey(`A ByteSliceReader for an empty byte array`, t, func() {
		bsd := ByteSliceReader([]byte{})

		Convey(`A byte read will return io.EOF and set the array to nil.`, func() {
			_, err := bsd.ReadByte()
			So(err, ShouldEqual, io.EOF)
			So([]byte(bsd), ShouldBeNil)
		})
	})
}
