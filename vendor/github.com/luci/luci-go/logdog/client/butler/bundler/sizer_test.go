// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package bundler

import (
	"fmt"
	"testing"

	"github.com/golang/protobuf/proto"
	. "github.com/smartystreets/goconvey/convey"
)

// A proto.Message implementation with test fields.
type testMessage struct {
	U64 uint64 `protobuf:"varint,1,opt,name=u64"`
}

func (t *testMessage) Reset()         {}
func (t *testMessage) String() string { return "" }
func (t *testMessage) ProtoMessage()  {}

func TestFastSizerVarintLength(t *testing.T) {
	Convey(`A test message`, t, func() {
		for _, threshold := range []uint64{
			0,
			0x80,
			0x4000,
			0x200000,
			0x100000000,
			0x800000000,
			0x40000000000,
			0x2000000000000,
			0x100000000000000,
			0x8000000000000000,
		} {

			for _, delta := range []int64{
				-2,
				-1,
				0,
				1,
				2,
			} {
				// Add "delta" to "threshold" in a uint64-aware manner.
				u64 := threshold
				if delta >= 0 {
					u64 += uint64(delta)
				} else {
					if u64 < uint64(-delta) {
						continue
					}
					u64 -= uint64(-delta)
				}

				expected := varintLength(u64)
				Convey(fmt.Sprintf(`Testing threshold 0x%x should encode to varint size %d`, u64, expected), func() {
					m := &testMessage{
						U64: u64,
					}

					expectedSize := proto.Size(m)
					if u64 == 0 {
						// Proto3 doesn't encode default values (0), so the expected size of
						// the number zero is zero.
						expectedSize = 0
					} else {
						// Accommodate the tag ("1").
						expectedSize -= varintLength(1)
					}
					So(expected, ShouldEqual, expectedSize)
				})
			}
		}
	})

	Convey(`Calculates protobuf size.`, t, func() {
		pbuf := &testMessage{
			U64: 0x600dd065,
		}

		So(protoSize(pbuf), ShouldEqual, proto.Size(pbuf))
	})
}
