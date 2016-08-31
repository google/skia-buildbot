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

func TestBytes(t *testing.T) {
	t.Parallel()

	Convey("bytes", t, func() {
		b := &bytes.Buffer{}
		t := []byte("this is a test")

		Convey("good", func() {
			_, err := WriteBytes(b, t)
			So(err, ShouldBeNil)
			exn := b.Len()
			r, n, err := ReadBytes(b)
			So(err, ShouldBeNil)
			So(n, ShouldEqual, exn)
			So(r, ShouldResemble, t)
		})

		Convey("bad (truncated buffer)", func() {
			_, err := WriteBytes(b, t)
			So(err, ShouldBeNil)
			bs := b.Bytes()[:b.Len()-4]
			_, n, err := ReadBytes(bytes.NewBuffer(bs))
			So(err, ShouldEqual, io.EOF)
			So(n, ShouldEqual, len(bs))
		})

		Convey("bad (bad varint)", func() {
			_, n, err := ReadBytes(bytes.NewBuffer([]byte{b10001111}))
			So(err, ShouldEqual, io.EOF)
			So(n, ShouldEqual, 1)
		})

		Convey("bad (huge data)", func() {
			n, err := WriteBytes(b, make([]byte, 2*1024*1024+1))
			So(err, ShouldBeNil)
			So(n, ShouldEqual, ReadByteLimit+2)
			_, n, err = ReadBytes(b)
			So(err.Error(), ShouldContainSubstring, "too big!")
			So(n, ShouldEqual, ReadByteLimit)
		})

		Convey("bad (write errors)", func() {
			_, err := WriteBytes(&fakeWriter{1}, t)
			So(err.Error(), ShouldContainSubstring, "nope")

			// transition boundary
			_, err = WriteBytes(&fakeWriter{7}, t)
			So(err.Error(), ShouldContainSubstring, "nope")
		})
	})
}

func TestStrings(t *testing.T) {
	t.Parallel()

	Convey("strings", t, func() {
		b := &bytes.Buffer{}
		t := "this is a test"

		Convey("empty", func() {
			n, err := WriteString(b, "")
			So(err, ShouldBeNil)
			So(n, ShouldEqual, 1)
			So(b.Bytes(), ShouldResemble, []byte{0})
			r, n, err := ReadString(b)
			So(err, ShouldBeNil)
			So(n, ShouldEqual, 1)
			So(r, ShouldEqual, "")
		})

		Convey("nulls", func() {
			t := "\x00\x00\x00\x00\x00"
			n, err := WriteString(b, t)
			So(n, ShouldEqual, 6)
			So(err, ShouldBeNil)
			exn := b.Len()
			r, n, err := ReadString(b)
			So(err, ShouldBeNil)
			So(n, ShouldEqual, exn)
			So(r, ShouldEqual, t)
		})

		Convey("bad (truncated buffer)", func() {
			n, err := WriteString(b, t)
			So(n, ShouldEqual, (len(t)*8)/7+1)
			So(err, ShouldBeNil)
			bs := b.Bytes()[:b.Len()-1]
			_, n, err = ReadString(bytes.NewBuffer(bs))
			So(err, ShouldEqual, io.EOF)
			So(n, ShouldEqual, len(bs))
		})

		Convey("single", func() {
			n, err := WriteString(b, "1")
			So(err, ShouldBeNil)
			So(n, ShouldEqual, 2)
			So(b.Bytes(), ShouldResemble, []byte{b00110001, b10000000})
			r, n, err := ReadString(b)
			So(err, ShouldBeNil)
			So(n, ShouldEqual, 2)
			So(r, ShouldEqual, "1")
		})

		Convey("good", func() {
			n, err := WriteString(b, t)
			So(err, ShouldBeNil)
			So(n, ShouldEqual, (len(t)*8)/7+1)
			So(b.Bytes()[:8], ShouldResemble, []byte{
				b01110101, b00110101, b00011011, b00101111, b00110011, b00000011,
				b10100101, b11100111,
			})
			exn := b.Len()
			r, n, err := ReadString(b)
			So(err, ShouldBeNil)
			So(len(r), ShouldEqual, len(t))
			So(n, ShouldEqual, exn)
			So(r, ShouldEqual, t)
		})

	})
}

// TODO(riannucci): make it [][]string instead

func TestStringSortability(t *testing.T) {
	t.Parallel()

	Convey("strings maintain sort order", t, func() {
		special := []string{
			"",
			"\x00",
			"\x00\x00",
			"\x00\x00\x00",
			"\x00\x00\x00\x00",
			"\x00\x00\x00\x00\x00",
			"\x00\x00\x00\x00\x01",
			"\x00\x00\x00\x00\xFF",
			"1234567",
			"12345678",
			"123456789",
			string(make([]byte, 7*8)),
		}
		orig := make(sort.StringSlice, randomTestSize, randomTestSize+len(special))

		r := rand.New(rand.NewSource(*seed))
		for i := range orig {
			buf := make([]byte, r.Intn(100))
			for j := range buf {
				buf[j] = byte(r.Uint32()) // watch me not care!
			}
			orig[i] = string(buf)
		}
		orig = append(orig, special...)

		enc := make(sort.StringSlice, len(orig))
		b := &bytes.Buffer{}
		for i := range enc {
			b.Reset()
			_, err := WriteString(b, orig[i])
			So(err, ShouldBeNil)
			enc[i] = b.String()
		}

		orig.Sort()
		enc.Sort()

		for i := range orig {
			decoded, _, err := ReadString(bytes.NewBufferString(enc[i]))
			So(err, ShouldBeNil)
			So(decoded, ShouldResemble, orig[i])
		}
	})
}

type StringSliceSlice [][]string

func (s StringSliceSlice) Len() int      { return len(s) }
func (s StringSliceSlice) Swap(i, j int) { s[i], s[j] = s[j], s[i] }
func (s StringSliceSlice) Less(i, j int) bool {
	a, b := s[i], s[j]
	lim := len(a)
	if len(b) < lim {
		lim = len(b)
	}
	for k := 0; k < lim; k++ {
		if a[k] > b[k] {
			return false
		} else if a[k] < b[k] {
			return true
		}
	}
	return len(a) < len(b)
}

func TestConcatenatedStringSortability(t *testing.T) {
	t.Parallel()

	Convey("concatenated strings maintain sort order", t, func() {
		orig := make(StringSliceSlice, randomTestSize)

		r := rand.New(rand.NewSource(*seed))
		for i := range orig {
			count := r.Intn(10)
			for j := 0; j < count; j++ {
				buf := make([]byte, r.Intn(100))
				for j := range buf {
					buf[j] = byte(r.Uint32()) // watch me not care!
				}
				orig[i] = append(orig[i], string(buf))
			}
		}
		orig = append(orig, [][]string{
			nil,
			{"", "aaa"},
			{"a", "aa"},
			{"aa", "a"},
		}...)

		enc := make(sort.StringSlice, len(orig))
		b := &bytes.Buffer{}
		for i, slice := range orig {
			b.Reset()
			for _, s := range slice {
				_, err := WriteString(b, s)
				So(err, ShouldBeNil)
			}
			enc[i] = b.String()
		}

		sort.Sort(orig)
		enc.Sort()

		for i := range orig {
			decoded := []string(nil)
			buf := bytes.NewBufferString(enc[i])
			for {
				dec, _, err := ReadString(buf)
				if err != nil {
					break
				}
				decoded = append(decoded, dec)
			}
			So(decoded, ShouldResemble, orig[i])
		}
	})
}
