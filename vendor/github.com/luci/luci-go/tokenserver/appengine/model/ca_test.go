// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package model

import (
	"testing"
	"time"

	"github.com/luci/luci-go/appengine/gaetesting"
	"github.com/luci/luci-go/common/clock/testclock"
	"github.com/luci/luci-go/common/data/caching/proccache"

	. "github.com/smartystreets/goconvey/convey"
)

func TestCAUniqueIDToCNMapLoadStore(t *testing.T) {
	Convey("CAUniqueIDToCNMap Load and Store works", t, func() {
		ctx := gaetesting.TestingContext()

		// Empty.
		mapping, err := LoadCAUniqueIDToCNMap(ctx)
		So(err, ShouldEqual, nil)
		So(len(mapping), ShouldEqual, 0)

		// Store some.
		toStore := map[int64]string{
			1: "abc",
			2: "def",
		}
		err = StoreCAUniqueIDToCNMap(ctx, toStore)
		So(err, ShouldBeNil)

		// Not empty now.
		mapping, err = LoadCAUniqueIDToCNMap(ctx)
		So(err, ShouldEqual, nil)
		So(mapping, ShouldResemble, toStore)
	})
}

func TestGetCAByUniqueID(t *testing.T) {
	Convey("GetCAByUniqueID works", t, func() {
		ctx := gaetesting.TestingContext()
		ctx = proccache.Use(ctx, &proccache.Cache{})
		ctx, clk := testclock.UseTime(ctx, testclock.TestTimeUTC)

		// Empty now.
		val, err := GetCAByUniqueID(ctx, 1)
		So(err, ShouldBeNil)
		So(val, ShouldEqual, "")

		// Add some.
		err = StoreCAUniqueIDToCNMap(ctx, map[int64]string{
			1: "abc",
			2: "def",
		})
		So(err, ShouldBeNil)

		// Still empty (cached old value).
		val, err = GetCAByUniqueID(ctx, 1)
		So(err, ShouldBeNil)
		So(val, ShouldEqual, "")

		// Updated after cache expires.
		clk.Add(2 * time.Minute)
		val, err = GetCAByUniqueID(ctx, 1)
		So(err, ShouldBeNil)
		So(val, ShouldEqual, "abc")
	})
}
