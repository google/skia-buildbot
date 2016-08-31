// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package proccache

import (
	"fmt"
	"testing"
	"time"

	"github.com/luci/luci-go/common/clock/testclock"
	"golang.org/x/net/context"

	. "github.com/smartystreets/goconvey/convey"
)

func Example() {
	counter := 0
	slowCall := func(c context.Context) (int, error) {
		counter++
		return counter, nil
	}

	cachedCall := Cached("key", func(c context.Context, key interface{}) (interface{}, time.Duration, error) {
		val, err := slowCall(c)
		return val, 0, err
	})

	// Default context silently skips caching.
	ctx := context.Background()
	a, _ := cachedCall(ctx)
	b, _ := cachedCall(ctx)
	fmt.Printf("%d, %d\n", a, b)

	// Injecting *Cache in the context makes it "remember" cached values.
	ctx = Use(context.Background(), &Cache{})
	a, _ = cachedCall(ctx)
	b, _ = cachedCall(ctx)
	fmt.Printf("%d, %d\n", a, b)

	// Output:
	// 1, 2
	// 3, 3
}

func TestCache(t *testing.T) {
	Convey("Put, get and expiration", t, func() {
		ctx, tc := testclock.UseTime(context.Background(), time.Unix(1444945245, 0))
		ctx = Use(ctx, &Cache{})

		val, ok := Get(ctx, "key")
		So(val, ShouldBeNil)
		So(ok, ShouldBeFalse)

		Put(ctx, "key", "value", time.Second)
		val, ok = Get(ctx, "key")
		So(val, ShouldEqual, "value")
		So(ok, ShouldBeTrue)

		// Expired.
		tc.Add(2 * time.Second)
		val, ok = Get(ctx, "key")
		So(val, ShouldBeNil)
		So(ok, ShouldBeFalse)

		// Unexpirable.
		Put(ctx, "key", "value", 0)
		tc.Add(60 * time.Minute)
		val, ok = Get(ctx, "key")
		So(val, ShouldEqual, "value")
		So(ok, ShouldBeTrue)
	})

	Convey("GetOrMake works", t, func() {
		ctx := Use(context.Background(), &Cache{})

		// Errors are not cached.
		val, err := GetOrMake(ctx, "key", func() (interface{}, time.Duration, error) {
			return "fail", 0, fmt.Errorf("fail")
		})
		So(val, ShouldBeNil)
		So(err.Error(), ShouldEqual, "fail")
		_, ok := Get(ctx, "key")
		So(ok, ShouldBeFalse)

		// Cache is cold.
		val, err = GetOrMake(ctx, "key", func() (interface{}, time.Duration, error) {
			return "first", 0, nil
		})
		So(val, ShouldEqual, "first")
		So(err, ShouldBeNil)

		// Cached value is used.
		val, err = GetOrMake(ctx, "key", func() (interface{}, time.Duration, error) {
			return "second", 0, nil
		})
		So(val, ShouldEqual, "first")
		So(err, ShouldBeNil)
	})

	Convey("Mutate works", t, func() {
		c := Cache{}

		c.Mutate("key", func(e *Entry) *Entry {
			So(e, ShouldBeNil)
			return &Entry{Value: "value"}
		})
		So(c.Get("key"), ShouldResemble, &Entry{Value: "value"})

		c.Mutate("key", func(e *Entry) *Entry {
			So(e, ShouldResemble, &Entry{Value: "value"})
			return nil
		})
		So(c.Get("key"), ShouldBeNil)
	})

	Convey("Add works", t, func() {
		ctx, tc := testclock.UseTime(context.Background(), time.Unix(1444945245, 0))
		ctx = Use(ctx, &Cache{})

		set, val := Add(ctx, "key", "value1", time.Second)
		So(set, ShouldBeTrue)
		So(val, ShouldEqual, "value1")

		set, val = Add(ctx, "key", "value2", time.Second)
		So(set, ShouldBeFalse)
		So(val, ShouldEqual, "value1")

		// Expire.
		tc.Add(2 * time.Second)

		set, val = Add(ctx, "key", "value2", time.Second)
		So(set, ShouldBeTrue)
		So(val, ShouldEqual, "value2")
	})
}
