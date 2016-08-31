// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package deps

import (
	"errors"
	"math"
	"runtime"
	"testing"

	"github.com/luci/luci-go/common/sync/parallel"
	. "github.com/smartystreets/goconvey/convey"
)

func TestSizeLimit(t *testing.T) {
	t.Parallel()

	Convey("Test SizeLimit", t, func() {
		Convey("no max", func() {
			l := sizeLimit{}
			So(l.PossiblyOK(100), ShouldBeTrue)
			So(l.Add(100), ShouldBeTrue)
		})

		Convey("some max", func() {
			l := sizeLimit{max: 100}
			So(l.PossiblyOK(100), ShouldBeTrue)
			So(l.PossiblyOK(101), ShouldBeFalse)
			So(l.Add(101), ShouldBeFalse)
			So(l.Add(20), ShouldBeTrue)
			So(l.PossiblyOK(100), ShouldBeFalse)
			So(l.Add(20), ShouldBeTrue)
			So(l.Add(20), ShouldBeTrue)
			So(l.Add(20), ShouldBeTrue)
			So(l.Add(20), ShouldBeTrue)
			So(l.Add(20), ShouldBeFalse)
		})

		Convey("check overflow", func() {
			l := sizeLimit{current: math.MaxUint32 - 5, max: math.MaxUint32}
			So(l.PossiblyOK(math.MaxUint32), ShouldBeFalse)
			So(l.Add(math.MaxUint32), ShouldBeFalse)
			So(l.PossiblyOK(5), ShouldBeTrue)
			So(l.PossiblyOK(6), ShouldBeFalse)
			So(l.Add(4), ShouldBeTrue)
			So(l.PossiblyOK(5), ShouldBeFalse)
			So(l.PossiblyOK(1), ShouldBeTrue)
			So(l.Add(1), ShouldBeTrue)
			So(l.Add(1), ShouldBeFalse)
			So(l.Add(math.MaxUint32), ShouldBeFalse)
			So(l.PossiblyOK(1), ShouldBeFalse)
		})

		Convey("concurrency", func() {
			prev := runtime.GOMAXPROCS(512)
			defer runtime.GOMAXPROCS(prev)
			sl := sizeLimit{max: 512}
			err := parallel.FanOutIn(func(pool chan<- func() error) {
				for i := 0; i < 512; i++ {
					pool <- func() error {
						if !sl.Add(1) {
							return errors.New("failed to add")
						}
						return nil
					}
				}
			})
			So(err, ShouldBeNil)
			So(sl.current, ShouldEqual, 512)
		})
	})
}
