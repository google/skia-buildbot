// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package gaeconfig

import (
	"testing"
	"time"

	"golang.org/x/net/context"

	"github.com/luci/gae/filter/featureBreaker"
	"github.com/luci/gae/impl/memory"
	"github.com/luci/gae/service/memcache"
	"github.com/luci/luci-go/common/clock/testclock"
	"github.com/luci/luci-go/common/data/caching/proccache"
	. "github.com/luci/luci-go/common/testing/assertions"
	. "github.com/smartystreets/goconvey/convey"
)

func TestCache(t *testing.T) {
	t.Parallel()

	Convey("Test cache", t, func() {
		c := context.Background()
		c = memory.Use(c)

		pc := &proccache.Cache{}
		c = proccache.Use(c, pc)
		c, clk := testclock.UseTime(c, testclock.TestTimeUTC)
		_ = clk

		mc := memcache.Get(c)
		c, mcFB := featureBreaker.FilterMC(c, nil)

		cache := &cache{}

		Convey("Should be able to store stuff", func() {
			Convey("memcache+proccache", func() {
				cache.Store(c, "item", time.Second, []byte("foobar"))
				itm, err := mc.Get(string(cacheKey("item")))
				So(err, ShouldBeNil)
				So(itm.Value(), ShouldResemble, []byte("\xff\xf4\xea\x9c\xf7\xd8\xde\xdc\x01foobar"))
				val, ok := proccache.Get(c, cacheKey("item"))
				So(ok, ShouldBeTrue)
				So(val, ShouldResemble, []byte("foobar"))

				Convey("and get it back", func() {
					So(cache.Retrieve(c, "item"), ShouldResemble, []byte("foobar"))

					Convey("unless it's expired", func() {
						clk.Add(time.Second * 2)
						So(cache.Retrieve(c, "item"), ShouldBeNil)
					})

					Convey("when missing from proccache", func() {
						proccache.GetCache(c).Mutate(cacheKey("item"), func(*proccache.Entry) *proccache.Entry {
							return nil
						})
						So(cache.Retrieve(c, "item"), ShouldResemble, []byte("foobar"))
					})

					Convey("unless memcache is corrupt", func() {
						proccache.GetCache(c).Mutate(cacheKey("item"), func(*proccache.Entry) *proccache.Entry {
							return nil
						})
						err := mc.Set(mc.NewItem(string(cacheKey("item"))).SetValue([]byte("\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff")))
						So(err, ShouldBeNil)

						So(cache.Retrieve(c, "item"), ShouldBeNil)
					})

				})
			})

			Convey("if memcache is down", func() {
				mcFB.BreakFeatures(nil, "SetMulti")

				cache.Store(c, "item", time.Second, []byte("foobar"))
				_, err := mc.Get(string(cacheKey("item")))
				So(err, ShouldErrLike, memcache.ErrCacheMiss)

				val, ok := proccache.Get(c, cacheKey("item"))
				So(ok, ShouldBeTrue)
				So(val, ShouldResemble, []byte("foobar"))

				Convey("and still get it back", func() {
					So(cache.Retrieve(c, "item"), ShouldResemble, []byte("foobar"))

					Convey("unless it's expired", func() {
						clk.Add(time.Second * 2)
						So(cache.Retrieve(c, "item"), ShouldBeNil)
					})
				})
			})
		})
	})
}
