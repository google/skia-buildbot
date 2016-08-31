// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package chunkstream

import (
	"fmt"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestBuffer(t *testing.T) {
	Convey(`An empty Buffer instance`, t, func() {
		b := Buffer{}

		Convey(`Has a length of zero.`, func() {
			So(b.Len(), ShouldEqual, 0)
		})

		Convey(`Has a FirstChunk of nil.`, func() {
			So(b.FirstChunk(), ShouldEqual, nil)
		})

		Convey(`Will panic if more than zero bytes are consumed.`, func() {
			So(func() { b.Consume(1) }, ShouldPanic)
		})

		Convey(`When Appending an empty chunk`, func() {
			c := tc()
			b.Append(c)

			Convey(`Has a FirstChunk of nil.`, func() {
				So(b.FirstChunk(), ShouldEqual, nil)
			})

			Convey(`The Chunk is released.`, func() {
				So(c.released, ShouldBeTrue)
			})
		})

		for _, chunks := range [][]*testChunk{
			{},
			{tc()},
			{tc(0, 1, 2), tc(), tc(3, 4, 5)},
		} {
			Convey(fmt.Sprintf(`With chunks %v, can append.`, chunks), func() {
				size := int64(0)
				coalesced := []byte(nil)
				for _, c := range chunks {
					b.Append(c)
					coalesced = append(coalesced, c.Bytes()...)
					size += int64(c.Len())
					So(b.Len(), ShouldEqual, size)
				}
				So(b.Bytes(), ShouldResemble, coalesced)

				Convey(`Can consume chunk-at-a-time.`, func() {
					for i, c := range chunks {
						if c.Len() > 0 {
							So(b.FirstChunk(), ShouldEqual, chunks[i])
						}
						So(b.Len(), ShouldEqual, size)
						b.Consume(int64(c.Len()))
						size -= int64(c.Len())
					}
					So(b.Len(), ShouldEqual, 0)

					Convey(`All chunks are released.`, func() {
						for _, c := range chunks {
							So(c.released, ShouldBeTrue)
						}
					})
				})

				Convey(`Can consume byte-at-a-time.`, func() {
					for i := int64(0); i < size; i++ {
						So(b.Len(), ShouldEqual, (size - i))
						So(b.Bytes(), ShouldResemble, coalesced[i:])
						b.Consume(1)
					}
					So(b.Len(), ShouldEqual, 0)

					Convey(`All chunks are released.`, func() {
						for _, c := range chunks {
							So(c.released, ShouldBeTrue)
						}
					})
				})

				Convey(`Can consume two bytes at a time.`, func() {
					for i := int64(0); i < size; i += 2 {
						// Final byte(s), make sure we don't over-consume.
						if b.Len() < 2 {
							i = b.Len()
						}

						So(b.Len(), ShouldEqual, (size - i))
						So(b.Bytes(), ShouldResemble, coalesced[i:])
						b.Consume(2)
					}
					So(b.Len(), ShouldEqual, 0)

					Convey(`All chunks are released.`, func() {
						for _, c := range chunks {
							So(c.released, ShouldBeTrue)
						}
					})
				})

				Convey(`Can consume all at once.`, func() {
					b.Consume(size)
					So(b.Len(), ShouldEqual, 0)

					Convey(`All chunks are released.`, func() {
						for _, c := range chunks {
							So(c.released, ShouldBeTrue)
						}
					})
				})
			})
		}
	})
}
