// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package iotools

import (
	"bytes"
	"errors"
	"io"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

// testWriter is an io.Writer and io.ByteWriter implementation that always
// writes the full amount and returns the configured error.
type testWriter struct {
	buf             bytes.Buffer
	writeByteCalled bool
	err             error
}

func (w *testWriter) Write(buf []byte) (int, error) {
	amt, _ := w.buf.Write(buf)
	return amt, w.err
}

func (w *testWriter) WriteByte(b byte) error {
	w.writeByteCalled = true

	if err := w.err; err != nil {
		return err
	}
	return w.buf.WriteByte(b)
}

type notAByteWriter struct {
	inner io.Writer
}

func (w *notAByteWriter) Write(buf []byte) (int, error) {
	return w.inner.Write(buf)
}

func TestCountingWriter(t *testing.T) {
	t.Parallel()

	Convey(`A CountingWriter backed by a test writer`, t, func() {
		tw := testWriter{}
		cw := CountingWriter{Writer: &tw}

		Convey(`When writing 10 bytes of data, registers a count of 10.`, func() {
			data := []byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}

			amount, err := cw.Write(data)
			So(err, ShouldBeNil)
			So(amount, ShouldEqual, 10)

			So(tw.buf.Bytes(), ShouldResemble, data)
			So(cw.Count, ShouldEqual, 10)
		})

		Convey(`When using 32 sequential WriteByte, uses underlying WriteByte and registers a count of 32.`, func() {
			written := bytes.Buffer{}

			for i := 0; i < 32; i++ {
				So(cw.WriteByte(byte(i)), ShouldBeNil)
				So(cw.Count, ShouldEqual, i+1)

				// Record for bulk comparison.
				written.WriteByte(byte(i))
			}

			So(tw.buf.Bytes(), ShouldResemble, written.Bytes())
			So(cw.Count, ShouldEqual, 32)
			So(tw.writeByteCalled, ShouldBeTrue)
		})

		Convey(`When an error is returned in Write, the error is propagated.`, func() {
			tw.err = errors.New("test error")
			data := []byte{0, 1, 2, 3}

			amount, err := cw.Write(data)
			So(amount, ShouldEqual, len(data))
			So(err, ShouldEqual, tw.err)
			So(tw.buf.Bytes(), ShouldResemble, data)
			So(cw.Count, ShouldEqual, len(data))
		})

		Convey(`When an error is returned in WriteByte, the error is propagated.`, func() {
			tw.err = errors.New("test error")

			err := cw.WriteByte(0x55)
			So(err, ShouldEqual, tw.err)
			So(tw.buf.Bytes(), ShouldHaveLength, 0)
			So(cw.Count, ShouldEqual, 0)
			So(tw.writeByteCalled, ShouldBeTrue)
		})

		Convey(`When WriteByte is disabled`, func() {
			cw.Writer = &notAByteWriter{&tw}

			Convey(`WriteByte calls the underlying Write and propagates test error.`, func() {
				tw.err = errors.New("test error")

				err := cw.WriteByte(0x55)
				So(err, ShouldEqual, tw.err)
				So(tw.buf.Bytes(), ShouldResemble, []byte{0x55})
				So(cw.Count, ShouldEqual, 1)
				So(tw.writeByteCalled, ShouldBeFalse)
			})
		})
	})
}
