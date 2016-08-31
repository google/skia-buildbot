// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package recordio

import (
	"bytes"
	"encoding/binary"
	"errors"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

type testWriter struct {
	buf  bytes.Buffer
	errQ []error
}

func (w *testWriter) Write(data []byte) (int, error) {
	if len(w.errQ) > 0 {
		err := w.errQ[0]
		w.errQ = w.errQ[1:]

		if err != nil {
			return 0, err
		}
	}
	return w.buf.Write(data)
}

func TestWriteFrame(t *testing.T) {
	t.Parallel()

	Convey(`Using a buffered test writer`, t, func() {
		tw := &testWriter{}

		Convey(`WriteFrame will successfully encode a zero-byte frame.`, func() {
			count, err := WriteFrame(tw, []byte{})
			So(count, ShouldEqual, 1)
			So(err, ShouldEqual, nil)
			So(tw.buf.Bytes(), ShouldResemble, []byte{0x00})
		})

		Convey(`WriteFrame will successfully encode a 129-byte frame.`, func() {
			data := bytes.Repeat([]byte{0x5A}, 129)

			count, err := WriteFrame(tw, data)
			So(count, ShouldEqual, 131)
			So(err, ShouldEqual, nil)
			So(tw.buf.Bytes(), ShouldResemble, append([]byte{0x81, 0x01}, data...))
		})

		Convey(`Will return an error when failing to write the size header.`, func() {
			failErr := errors.New("test: test-induced size error")
			tw.errQ = []error{
				failErr,
			}

			count, err := WriteFrame(tw, []byte{0xd0, 0x65})
			So(count, ShouldEqual, 0)
			So(err, ShouldEqual, failErr)
		})

		Convey(`Will return an error when failing to write the frame data.`, func() {
			failErr := errors.New("test: test-induced size error")
			tw.errQ = []error{
				nil,
				failErr,
			}

			count, err := WriteFrame(tw, []byte{0xd0, 0x65})
			So(count, ShouldEqual, 1)
			So(err, ShouldEqual, failErr)
		})
	})
}

func TestWriter(t *testing.T) {
	t.Parallel()

	Convey(`A Writer, configured to write to a buffer`, t, func() {
		tw := &testWriter{}
		w := NewWriter(tw)

		Convey(`Can write consecutive frames in 3-byte chunks.`, func() {
			expected := []byte{}
			var sizeBuf [binary.MaxVarintLen64]byte
			for _, size := range []int{
				1,
				0,
				1025,
				129,
				11,
			} {
				b := bytes.Repeat([]byte{0x55}, size)
				expected = append(expected, sizeBuf[:binary.PutUvarint(sizeBuf[:], uint64(size))]...)
				expected = append(expected, b...)

				for len(b) > 0 {
					count := 3
					if count > len(b) {
						count = len(b)
					}

					c, err := w.Write(b[:count])
					So(err, ShouldBeNil)
					So(c, ShouldEqual, count)

					b = b[count:]
				}
				So(w.Flush(), ShouldBeNil)
			}

			So(tw.buf.Bytes(), ShouldResemble, expected)
		})

		Convey(`Will write empty frames if Flush()ed.`, func() {
			So(w.Flush(), ShouldBeNil)
			So(w.Flush(), ShouldBeNil)
			So(w.Flush(), ShouldBeNil)
			So(tw.buf.Bytes(), ShouldResemble, []byte{0x00, 0x00, 0x00})
		})

		Convey(`Will fail to Flush() if the Write fails.`, func() {
			failErr := errors.New("test: test-induced size error")
			tw.errQ = []error{
				failErr,
			}
			So(w.Flush(), ShouldEqual, failErr)
		})
	})
}
