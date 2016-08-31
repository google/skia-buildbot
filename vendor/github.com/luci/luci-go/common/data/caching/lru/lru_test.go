// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package lru

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

// If put is true, test the Put command; if false, test the PutIfMissing
// command.
func TestCache(t *testing.T) {
	t.Parallel()

	Convey(`An locking LRU cache with size heuristic 3`, t, func() {
		cache := New(3)

		Convey(`A Get() returns nil.`, func() {
			So(cache.Get("test"), ShouldBeNil)
		})

		// Adds values to the cache sequentially, blocking on the values being
		// processed.
		addCacheValues := func(values ...string) {
			for _, v := range values {
				isPresent := (cache.Peek(v) != nil)
				So(cache.Put(v, v+"v"), ShouldEqual, isPresent)
			}
		}

		shouldHaveValues := func(actual interface{}, expected ...interface{}) string {
			cache := actual.(*Cache)

			actualSnapshot := cache.snapshot()

			expectedSnapshot := snapshot{}
			for _, k := range expected {
				expectedSnapshot[k] = k.(string) + "v"
			}
			return ShouldResemble(actualSnapshot, expectedSnapshot)
		}

		Convey(`With three values, {a, b, c}`, func() {
			addCacheValues("a", "b", "c")
			So(cache.Len(), ShouldEqual, 3)

			Convey(`Is empty after a purge.`, func() {
				cache.Purge()
				So(cache.Len(), ShouldEqual, 0)
			})

			Convey(`Can retrieve each of those values.`, func() {
				So(cache.Get("a"), ShouldEqual, "av")
				So(cache.Get("b"), ShouldEqual, "bv")
				So(cache.Get("c"), ShouldEqual, "cv")
			})

			Convey(`Get()ting "a", then adding "d" will cause "b" to be evicted.`, func() {
				So(cache.Get("a"), ShouldEqual, "av")
				addCacheValues("d")
				So(cache, shouldHaveValues, "a", "c", "d")
			})

			Convey(`Peek()ing "a", then adding "d" will cause "a" to be evicted.`, func() {
				So(cache.Peek("a"), ShouldEqual, "av")
				addCacheValues("d")
				So(cache, shouldHaveValues, "b", "c", "d")
			})
		})

		Convey(`When adding {a, b, c, d}, "a" will be evicted.`, func() {
			addCacheValues("a", "b", "c", "d")
			So(cache.Len(), ShouldEqual, 3)

			So(cache, shouldHaveValues, "b", "c", "d")

			Convey(`Requests for "a" will be nil.`, func() {
				So(cache.Get("a"), ShouldBeNil)
			})
		})

		Convey(`When adding {a, b, c, a, d}, "b" will be evicted.`, func() {
			addCacheValues("a", "b", "c", "a", "d")
			So(cache.Len(), ShouldEqual, 3)

			So(cache, shouldHaveValues, "a", "c", "d")

			Convey(`When removing "c", will contain {a, d}.`, func() {
				So(cache.Remove("c"), ShouldEqual, "cv")
				So(cache, shouldHaveValues, "a", "d")

				Convey(`When adding {e, f}, "a" will be evicted.`, func() {
					addCacheValues("e", "f")
					So(cache, shouldHaveValues, "d", "e", "f")
				})
			})
		})

		Convey(`When removing a value that isn't there, returns nil.`, func() {
			So(cache.Remove("foo"), ShouldBeNil)
		})
	})
}
