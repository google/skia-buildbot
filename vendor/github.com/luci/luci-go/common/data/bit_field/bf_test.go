// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package bit_field

import (
	"encoding/binary"
	"math/rand"
	"testing"

	. "github.com/luci/luci-go/common/testing/assertions"

	. "github.com/smartystreets/goconvey/convey"
)

func TestBitField(t *testing.T) {
	Convey("BitField", t, func() {
		bf := Make(2000)

		Convey("Should be sized right", func() {
			So(bf.Size(), ShouldEqual, 2000)
			So(len(bf.data), ShouldEqual, 250)
		})

		Convey("Should be empty", func() {
			So(bf.All(false), ShouldBeTrue)
			So(bf.data[0], ShouldEqual, 0)
			So(bf.CountSet(), ShouldEqual, 0)
			So(bf.IsSet(20), ShouldBeFalse)
			So(bf.IsSet(200001), ShouldBeFalse)

			Convey("and be unset", func() {
				for i := uint32(0); i < 20; i++ {
					So(bf.IsSet(i), ShouldBeFalse)
				}
			})
		})

		Convey("Boundary conditions are caught", func() {
			So(func() { bf.Set(2000) }, ShouldPanicLike, "cannot set bit 2000")
			So(func() { bf.Clear(2000) }, ShouldPanicLike, "cannot clear bit 2000")
		})

		Convey("and setting [0, 1, 19, 197, 4]", func() {
			bf.Set(0)
			bf.Set(1)
			bf.Set(19)
			bf.Set(197)
			bf.Set(1999)
			bf.Set(4)

			Convey("should count correctly", func() {
				So(bf.CountSet(), ShouldEqual, 6)
			})

			Convey("should retrieve correctly", func() {
				So(bf.IsSet(2), ShouldBeFalse)
				So(bf.IsSet(18), ShouldBeFalse)

				So(bf.IsSet(0), ShouldBeTrue)
				So(bf.IsSet(1), ShouldBeTrue)
				So(bf.IsSet(4), ShouldBeTrue)
				So(bf.IsSet(19), ShouldBeTrue)
				So(bf.IsSet(197), ShouldBeTrue)
				So(bf.IsSet(1999), ShouldBeTrue)
			})

			Convey("should clear correctly", func() {
				bf.Clear(3)
				bf.Clear(4)
				bf.Clear(197)

				So(bf.IsSet(2), ShouldBeFalse)
				So(bf.IsSet(3), ShouldBeFalse)
				So(bf.IsSet(18), ShouldBeFalse)

				So(bf.IsSet(4), ShouldBeFalse)
				So(bf.IsSet(197), ShouldBeFalse)

				So(bf.IsSet(0), ShouldBeTrue)
				So(bf.IsSet(1), ShouldBeTrue)
				So(bf.IsSet(19), ShouldBeTrue)

				So(bf.CountSet(), ShouldEqual, 4)
			})

			Convey("should reset correctly", func() {
				bf.Reset()
				So(bf.Size(), ShouldEqual, 0)
				So(bf.data, ShouldBeEmpty)
			})
		})

		Convey("Can interact with datastore", func() {
			Convey("encodes to a []byte", func() {
				p, err := bf.ToProperty()
				So(err, ShouldBeNil)

				bval := make([]byte, 252)
				// varint encoding of 2000
				bval[0] = 208
				bval[1] = 15
				So(p.Value(), ShouldResemble, bval)

				Convey("decodes as well", func() {
					nbf := BitField{}
					So(nbf.FromProperty(p), ShouldBeNil)
					So(nbf, ShouldResemble, bf)
				})
			})

			Convey("zero-length BitField has small representation", func() {
				bf = Make(0)
				p, err := bf.ToProperty()
				So(err, ShouldBeNil)
				So(p.Value(), ShouldResemble, []byte{0})

				Convey("decodes as well", func() {
					nbf := BitField{}
					So(nbf.FromProperty(p), ShouldBeNil)
					So(nbf, ShouldResemble, bf)
				})
			})

			Convey("setting bits round-trips", func() {
				bf.Set(0)
				bf.Set(1)
				bf.Set(19)
				bf.Set(197)
				bf.Set(1999)
				bf.Set(4)

				p, err := bf.ToProperty()
				So(err, ShouldBeNil)

				bval := make([]byte, 252)
				// varint encoding of 2000
				bval[0] = 208
				bval[1] = 15
				// various bits set
				bval[2] = 19    // 0 and 1 and 4
				bval[4] = 8     // 19
				bval[26] = 32   // 197
				bval[251] = 128 // 1999
				So(p.Value(), ShouldResemble, bval)

				nbf := BitField{}
				So(nbf.FromProperty(p), ShouldBeNil)
				So(nbf, ShouldResemble, bf)

				So(nbf.IsSet(2), ShouldBeFalse)
				So(nbf.IsSet(18), ShouldBeFalse)

				So(nbf.IsSet(0), ShouldBeTrue)
				So(nbf.IsSet(1), ShouldBeTrue)
				So(nbf.IsSet(4), ShouldBeTrue)
				So(nbf.IsSet(19), ShouldBeTrue)
				So(nbf.IsSet(197), ShouldBeTrue)
				So(nbf.IsSet(1999), ShouldBeTrue)
			})

			Convey("empty sets have canonical representation", func() {
				bf = Make(0)
				p, err := bf.ToProperty()
				So(err, ShouldBeNil)
				So(p.Value(), ShouldResemble, []byte{0})

				nbf := BitField{}
				So(nbf.FromProperty(p), ShouldBeNil)
				So(nbf, ShouldResemble, bf)
			})

			Convey("small sets correctly encode", func() {
				bf = Make(2)
				bf.Set(0)
				p, err := bf.ToProperty()
				So(err, ShouldBeNil)
				So(p.Value(), ShouldResemble, []byte{2, 1})

				nbf := BitField{}
				So(nbf.FromProperty(p), ShouldBeNil)
				So(nbf, ShouldResemble, bf)
			})
		})
	})
}

func BenchmarkCount(b *testing.B) {
	b.StopTimer()
	r := rand.New(rand.NewSource(193482))
	bf := Make(1000000)
	if len(bf.data) != 125000 {
		b.Fatalf("unexpected length of bf.data: %d", len(bf.data))
	}
	for i := 0; i < len(bf.data); i += 4 {
		binary.BigEndian.PutUint32(bf.data[i:i+4], r.Uint32())
	}
	b.StartTimer()

	for i := 0; i < b.N; i++ {
		num := bf.CountSet()
		if num != 500188 {
			b.Fatalf("expected to see %d set, got %d", 500188, num)
		}
	}
}
