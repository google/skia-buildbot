// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package bundler

import (
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"
)

func TestDataPool(t *testing.T) {
	Convey(`A data pool registry`, t, func() {
		reg := dataPoolRegistry{}

		Convey(`Can retrieve a 1024-byte data pool.`, func() {
			pool := reg.getPool(1024)
			So(pool.size, ShouldEqual, 1024)

			Convey(`Subsequent requests return the same pool.`, func() {
				So(reg.getPool(1024), ShouldEqual, pool)
			})
		})
	})
}

func TestData(t *testing.T) {
	Convey(`A 512-byte data pool`, t, func() {
		pool := (&dataPoolRegistry{}).getPool(512)

		Convey(`Will allocate a clean 512-byte Data`, func() {
			d := pool.getData().(*streamData)
			defer d.Release()

			So(d, ShouldNotBeNil)
			So(len(d.Bytes()), ShouldEqual, 512)
			So(cap(d.Bytes()), ShouldEqual, 512)

			Convey(`When bound, adjusts its byte size and retains a timestamp.`, func() {
				now := time.Date(2015, 1, 1, 0, 0, 0, 0, time.UTC)
				data := [64]byte{}
				for i := range data {
					data[i] = byte(i)
				}

				copy(d.Bytes(), data[:])
				So(d.Bind(len(data), now), ShouldEqual, d)
				So(d.Bytes(), ShouldResemble, data[:])
				So(d.Len(), ShouldEqual, len(data))
				So(d.Timestamp().Equal(now), ShouldBeTrue)
			})
		})
	})
}
