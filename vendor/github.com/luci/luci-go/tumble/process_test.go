// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package tumble

import (
	"fmt"
	"testing"
	"time"

	"github.com/luci/gae/service/datastore/serialize"
	"github.com/luci/gae/service/memcache"
	"github.com/luci/luci-go/common/clock"
	"github.com/luci/luci-go/common/logging"
	"github.com/luci/luci-go/common/logging/memlogger"
	. "github.com/smartystreets/goconvey/convey"
)

func TestTumbleFiddlyBits(t *testing.T) {
	t.Parallel()

	Convey("Fiddly bits", t, func() {
		tt := &Testing{}
		ctx := tt.Context()
		l := logging.Get(ctx).(*memlogger.MemLogger)

		Convey("early exit logic works", func() {
			mc := memcache.Get(ctx)
			future := clock.Now(ctx).UTC().Add(time.Hour)
			itm := mc.NewItem(fmt.Sprintf("%s.%d.last", baseName, 10)).SetValue(serialize.ToBytes(future))
			So(mc.Set(itm), ShouldBeNil)

			So(fireTasks(ctx, tt.GetConfig(ctx), map[taskShard]struct{}{
				taskShard{10, minTS}: {},
			}), ShouldBeTrue)
			tt.Drain(ctx)

			So(l.Has(logging.Info,
				"Processing tumble shard.", map[string]interface{}{"shard": uint64(10)}),
				ShouldBeTrue)
			So(l.Has(logging.Info,
				"early exit, 0001-02-03 05:05:10 +0000 UTC > 0001-02-03 04:05:12 +0000 UTC", nil),
				ShouldBeTrue)
		})
	})
}
