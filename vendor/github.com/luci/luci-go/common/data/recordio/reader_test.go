// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package recordio

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

// plainReader implements the io.Reader interface on top of a bytes.Buffer;
// however, it does not also implement io.ByteReader.
type plainReader struct {
	buf bytes.Buffer
	err error
}

func (r *plainReader) loadFrames(chunks ...[]byte) {
	for _, chunk := range chunks {
		_, err := WriteFrame(&r.buf, chunk)
		if err != nil {
			panic(err)
		}
	}
}

func (r *plainReader) Read(data []byte) (int, error) {
	if r.err != nil {
		return 0, r.err
	}
	return r.buf.Read(data)
}

type testByteReader struct {
	plainReader

	readByteErr error
	readBytes   int
	byteBuf     [1]byte
}

func (r *testByteReader) ReadByte() (b byte, err error) {
	if r.readByteErr != nil {
		return 0, r.readByteErr
	}

	b, err = r.buf.ReadByte()
	if err == nil {
		r.readBytes++
	}
	return
}

func btos(b ...[]byte) []string {
	s := make([]string, len(b))
	for i, v := range b {
		s[i] = string(v)
	}
	return s
}

// TestReader tests the default Reader implementation, "reader".
func TestReader(t *testing.T) {
	t.Parallel()

	Convey(`A frame reader with max size 1MB using a plain io.Reader`, t, func() {
		maxSize := int64(1024 * 1024)
		tr := plainReader{}
		r := NewReader(&tr, maxSize)

		Convey(`Will return io.EOF with an empty reader.`, func() {
			_, err := r.ReadFrameAll()
			So(err, ShouldEqual, io.EOF)
		})

		Convey(`Can successfully read a frame.`, func() {
			data := []byte{0x13, 0x37, 0xd0, 0x65}
			tr.loadFrames(data)

			f, err := r.ReadFrameAll()
			So(err, ShouldBeNil)
			So(f, ShouldResemble, data)
		})

		Convey(`Can successfully read two frames.`, func() {
			data := [][]byte{
				{0x13, 0x37, 0xd0, 0x65},
				{0xd0, 0x06, 0xea, 0x15, 0xf0, 0x0d},
			}
			tr.loadFrames(data...)

			c, fr, err := r.ReadFrame()
			So(err, ShouldBeNil)
			So(c, ShouldEqual, 4)

			d, err := ioutil.ReadAll(fr)
			So(err, ShouldBeNil)
			So(d, ShouldResemble, data[0])

			c, fr, err = r.ReadFrame()
			So(err, ShouldBeNil)
			So(c, ShouldEqual, 6)

			d, err = ioutil.ReadAll(fr)
			So(err, ShouldBeNil)
			So(d, ShouldResemble, data[1])
		})

		Convey(`When reading a frame, will return EOF if the frame is exceeded.`, func() {
			data := []byte{0x13, 0x37, 0xd0, 0x65}
			tr.loadFrames(data)

			count, fr, err := r.ReadFrame()
			So(err, ShouldBeNil)
			So(count, ShouldEqual, 4)

			buf := make([]byte, 5)
			c, err := fr.Read(make([]byte, 5))
			So(c, ShouldEqual, 4)
			So(err, ShouldBeNil)

			buf = buf[count:]
			_, err = fr.Read(buf)
			So(err, ShouldEqual, io.EOF)
		})

		Convey(`Will fail if the underlying frame exceeds the maximum size.`, func() {
			var sizeBuf [binary.MaxVarintLen64]byte
			tr.buf.Write(sizeBuf[:binary.PutUvarint(sizeBuf[:], uint64(maxSize+1))])

			_, err := r.ReadFrameAll()
			So(err, ShouldEqual, ErrFrameTooLarge)
		})

		Convey(`Will fail if the frame contains an invalid size header.`, func() {
			tr.buf.Write([]byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF})
			_, err := r.ReadFrameAll()
			So(err, ShouldNotBeNil)
		})

		Convey(`Can read conscutive frames, then io.EOF.`, func() {
			data := [][]byte{}
			for _, size := range []int{
				0,
				14,
				1024 * 1024,
				0,
				511,
			} {
				data = append(data, bytes.Repeat([]byte{0x5A}, size))
				tr.loadFrames(data[len(data)-1])
			}

			for _, expected := range data {
				f, err := r.ReadFrameAll()
				So(err, ShouldBeNil)

				if len(expected) == 0 {
					expected = nil
				}
				So(f, ShouldResemble, expected)
			}

			_, err := r.ReadFrameAll()
			So(err, ShouldEqual, io.EOF)
		})
	})

	Convey(`A frame reader with max size 1MB using an io.Reader+io.ByteReader`, t, func() {
		tr := testByteReader{}
		r := NewReader(&tr, 1024*1024)

		Convey(`Will return io.EOF with an empty reader.`, func() {
			_, err := r.ReadFrameAll()
			So(err, ShouldEqual, io.EOF)
		})

		Convey(`Will use io.ByteReader to read the frame header.`, func() {
			data := []byte{0x13, 0x37, 0xd0, 0x65}
			tr.loadFrames(data)

			f, err := r.ReadFrameAll()
			So(err, ShouldBeNil)
			So(f, ShouldResemble, data)
			So(tr.readBytes, ShouldEqual, 1)
		})

		Convey(`Will fail if the underlying io.Reader returns an error.`, func() {
			tr.loadFrames([]byte{})
			tr.err = errors.New("test: test-induced error")
			tr.readByteErr = tr.err
			_, err := r.ReadFrameAll()
			So(err, ShouldEqual, tr.err)
		})

		Convey(`Will fail if an error is returned while reading frame's data.`, func() {
			data := []byte{0x13, 0x37, 0xd0, 0x65}
			tr.loadFrames(data)

			// Have "ReadByte()" calls ignore the configured error. This will cause
			// the frame size to be read without incident, but the frame data to still
			// return an error.
			tr.err = errors.New("test: test-induced error")
			data, err := r.ReadFrameAll()
			So(err, ShouldEqual, tr.err)
		})
	})
}

func TestSplit(t *testing.T) {
	t.Parallel()

	Convey(`Testing Split`, t, func() {
		for _, v := range [][]string{
			{},
			{""},
			{"", "foo", ""},
			{"foo", "bar", "baz"},
		} {
			Convey(fmt.Sprintf(`Can Split stream: %#v`, v), func() {
				// Write frames to "buf".
				var buf bytes.Buffer
				for _, s := range v {
					_, err := WriteFrame(&buf, []byte(s))
					if err != nil {
						panic(err)
					}
				}

				// Confirm that Split works.
				sp, err := Split(buf.Bytes())
				So(err, ShouldBeNil)
				So(btos(sp...), ShouldResemble, v)
			})
		}

		Convey(`Will refuse to split a frame that is too large.`, func() {
			_, err := Split([]byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0x00})
			So(err, ShouldEqual, ErrFrameTooLarge)
		})

		Convey(`Will fail to split if there aren't enough bytes.`, func() {
			sp, err := Split([]byte{0x01, 0xAA, 0x02}) // 1-byte {0xAA}, 2-bytes ... EOF!
			So(sp, ShouldResemble, [][]byte{{0xAA}})
			So(err, ShouldEqual, ErrFrameTooLarge)
		})
	})
}
