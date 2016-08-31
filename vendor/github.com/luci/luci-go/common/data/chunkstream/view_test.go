// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package chunkstream

import (
	"bytes"
	"fmt"
	"io"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestView(t *testing.T) {
	Convey(`An empty Buffer with generation 42`, t, func() {
		b := Buffer{}

		for _, chunks := range [][]*testChunk{
			[]*testChunk(nil),
			{tc()},
			{tc(0)},
			{tc(0, 0, 1)},
			{tc(0), tc(0), tc(0)},
			{tc(1, 2, 3, 0)},
			{tc(1, 2), tc(3, 0, 4, 0), tc(0, 5)},
		} {
			Convey(fmt.Sprintf(`With Chunks %v`, chunks), func() {
				aggregate := []byte{}
				size := int64(0)
				for _, c := range chunks {
					aggregate = append(aggregate, c.Bytes()...)
					size += int64(c.Len())
					b.Append(c)
				}

				// This is pretty stupid to do, but for testing it's a nice edge case
				// to hammer.
				Convey(`A View with limit 0`, func() {
					br := b.ViewLimit(0)

					So(br.Remaining(), ShouldEqual, 0)
					So(br.Consumed(), ShouldEqual, 0)

					Convey(`Read() returns EOF.`, func() {
						buf := make([]byte, 16)
						a, err := br.Read(buf)
						So(err, ShouldEqual, io.EOF)
						So(a, ShouldEqual, 0)
					})

					Convey(`ReadByte() returns EOF.`, func() {
						_, err := br.ReadByte()
						So(err, ShouldEqual, io.EOF)
					})

					Convey(`Skip() panics.`, func() {
						So(func() { br.Skip(1) }, ShouldPanic)
					})

					Convey(`Index() returns -1.`, func() {
						So(br.Index([]byte{0}), ShouldEqual, -1)
					})
				})

				Convey(`An unlimited View`, func() {
					br := b.View()
					So(br.Remaining(), ShouldEqual, b.Len())
					So(br.Consumed(), ShouldEqual, 0)

					Convey(`Can Read() the full block of data.`, func() {
						buf := make([]byte, len(aggregate))
						amt, err := br.Read(buf)
						So(amt, ShouldEqual, len(aggregate))
						So(err, ShouldEqual, io.EOF)
						So(buf[:amt], ShouldResemble, aggregate)

						Convey(`Subsequent Read() will return io.EOF.`, func() {
							amt, err := br.Read(buf)
							So(amt, ShouldEqual, 0)
							So(err, ShouldEqual, io.EOF)
						})
					})

					Convey(`Can Read() the full block of data byte-by-byte.`, func() {
						buf := make([]byte, 1)
						for i, d := range aggregate {
							amt, err := br.Read(buf)
							if i == len(aggregate)-1 {
								So(err, ShouldEqual, io.EOF)
							} else {
								So(err, ShouldBeNil)
							}

							So(amt, ShouldEqual, 1)
							So(buf[0], ShouldEqual, d)
						}

						Convey(`Subsequent Read() will return io.EOF.`, func() {
							amt, err := br.Read(buf)
							So(amt, ShouldEqual, 0)
							So(err, ShouldEqual, io.EOF)
						})
					})

					Convey(`Can ReadByte() the full block of data.`, func() {
						for _, d := range aggregate {
							b, err := br.ReadByte()
							So(err, ShouldBeNil)
							So(b, ShouldEqual, d)
						}

						Convey(`Subsequent ReadByte() will return io.EOF.`, func() {
							_, err := br.ReadByte()
							So(err, ShouldEqual, io.EOF)
						})
					})

					for _, needle := range [][]byte{
						{0},
						{0, 0},
						{0, 0, 0, 0},
					} {
						expected := bytes.Index(aggregate, needle)
						Convey(fmt.Sprintf(`Index() of %v returns %v.`, needle, expected), func() {
							So(br.Index(needle), ShouldEqual, expected)
						})
					}
				})
			})
		}

		Convey(`An unlimited View`, func() {
			br := b.View()

			Convey(`Has chunksRemaining() value of 0.`, func() {
				So(br.chunkRemaining(), ShouldEqual, 0)
			})
		})

		Convey(`With chunks [{0x01, 0x02, 0x00}, {0x00, 0x03}, {0x00}, {0x00, 0x00}, {0x04}]`, func() {
			for _, c := range []Chunk{tc(1, 2, 0), tc(0, 3), tc(0), tc(0, 0), tc(4)} {
				b.Append(c)
			}

			Convey(`An unlimited View`, func() {
				br := b.View()

				Convey(`Should have Remaining() value of 9.`, func() {
					So(br.Remaining(), ShouldEqual, 9)
				})

				Convey(`Can spawn a limited clone.`, func() {
					buf := bytes.Buffer{}
					_, err := buf.ReadFrom(br.CloneLimit(7))
					So(err, ShouldBeNil)
					So(buf.Bytes(), ShouldResemble, []byte{0x01, 0x02, 0x00, 0x00, 0x03, 0x00, 0x00})
				})

				for _, s := range []struct {
					needle []byte
					index  int64
				}{
					{[]byte(nil), 0},
					{[]byte{0x01}, 0},
					{[]byte{0x01, 0x02}, 0},
					{[]byte{0x02, 0x00}, 1},
					{[]byte{0x00}, 2},
					{[]byte{0x00, 0x00}, 2},
					{[]byte{0x00, 0x00, 0x00}, 5},
					{[]byte{0x03, 0x00, 0x00}, 4},
				} {
					Convey(fmt.Sprintf(`Has Index %v for needle %v`, s.index, s.needle), func() {
						So(br.Index(s.needle), ShouldEqual, s.index)
					})
				}
			})

			Convey(`A View with a limit of 6`, func() {
				br := b.ViewLimit(6)

				Convey(`Should have Remaining() value of 6.`, func() {
					So(br.Remaining(), ShouldEqual, 6)
				})

				Convey(`Has index of -1 for needle [0x00, 0x04]`, func() {
					So(br.Index([]byte{0x00, 0x04}), ShouldEqual, -1)
				})
			})

			Convey(`A View with a limit of 20`, func() {
				br := b.ViewLimit(20)

				Convey(`Should have Remaining() value of 9.`, func() {
					So(br.Remaining(), ShouldEqual, 9)
				})
			})
		})

		Convey(`With chunks [{0x0F}..{0x00}]`, func() {
			for i := 0x0F; i >= 0x00; i-- {
				b.Append(tc(byte(i)))
			}
			br := b.View()

			Convey(`Has index of -1 for needle [0x0f, 0x10]`, func() {
				So(br.Index([]byte{0x0f, 0x10}), ShouldEqual, -1)
			})

			for _, s := range []struct {
				needle []byte
				index  int64
			}{
				{[]byte{0x0F}, 0},
				{[]byte{0x04, 0x03, 0x02}, 11},
				{[]byte{0x01, 0x00}, 14},
				{[]byte{0x00}, 15},
				{[]byte{0x00, 0xFF}, -1},
			} {
				Convey(fmt.Sprintf(`Has Index %v for needle %v`, s.index, s.needle), func() {
					So(br.Index(s.needle), ShouldEqual, s.index)
				})
			}
		})
	})
}
