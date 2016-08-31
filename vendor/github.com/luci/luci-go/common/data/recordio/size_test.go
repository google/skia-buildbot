// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package recordio

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestFrameHeaderSize(t *testing.T) {
	t.Parallel()

	Convey(`Testing FrameHeaderSize`, t, func() {

		Convey(`Matches actual written frame size`, func() {
			prev := -1
			for i := 0; i < 3; i++ {
				base := 1 << uint64(7*i)
				for _, delta := range []int{-1, 0, 1} {
					base += delta
					if base <= prev {
						// Over/underflow, skip.
						continue
					}
					prev = base

					Convey(fmt.Sprintf(`Frame size %d.`, base), func() {
						data := bytes.Repeat([]byte{0x55}, int(base))

						amt, err := WriteFrame(ioutil.Discard, data)
						if err != nil {
							panic(err)
						}

						So(amt-len(data), ShouldEqual, FrameHeaderSize(int64(len(data))))
					})
				}
			}
		})

		Convey(`Matches written frame header size (no alloc)`, func() {
			prev, first := int64(0), true
			for i := 0; i < 9; i++ {
				for _, delta := range []int64{-1, 0, 1} {
					base := int64(1<<uint64(7*i)) + delta
					if (!first) && base <= prev {
						// Repeated value, skip.
						continue
					}
					prev, first = base, false

					Convey(fmt.Sprintf(`Frame size %d.`, base), func() {
						amt, err := writeFrameHeader(ioutil.Discard, uint64(base))
						if err != nil {
							panic(err)
						}

						So(amt, ShouldEqual, FrameHeaderSize(int64(base)))
					})
				}
			}
		})
	})
}
