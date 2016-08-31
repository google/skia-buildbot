// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package tumble

import (
	"math"
	"testing"
	"time"

	"github.com/luci/gae/service/taskqueue"
	"github.com/luci/luci-go/common/clock/testclock"
	"github.com/luci/luci-go/common/logging"
	"github.com/luci/luci-go/common/logging/memlogger"

	. "github.com/smartystreets/goconvey/convey"
)

func TestShardCalculation(t *testing.T) {
	t.Parallel()

	Convey("shard calculation", t, func() {
		tt := &Testing{}
		ctx := tt.Context()
		cfg := tt.GetConfig(ctx)
		cfg.NumShards = 11
		tt.UpdateSettings(ctx, cfg)

		l := logging.Get(ctx).(*memlogger.MemLogger)

		Convey("ExpandedShard->Shard covers the full int64 range", func() {
			tcs := []struct {
				es int64
				s  uint64
			}{
				{math.MinInt64, 0},
				{0, cfg.NumShards / 2},
				{math.MaxInt64, cfg.NumShards - 1},
			}

			for _, tc := range tcs {
				So((&realMutation{ExpandedShard: tc.es}).shard(cfg).shard, ShouldEqual, tc.s)
				low, high := expandedShardBounds(ctx, cfg, tc.s)
				So(tc.es, ShouldBeGreaterThanOrEqualTo, low)
				So(tc.es, ShouldBeLessThanOrEqualTo, high)
				So(l.Messages(), ShouldBeEmpty)
			}
		})

		Convey("expandedShardsPerShard returns crossed ranges on shard reduction", func() {
			low, high := expandedShardBounds(ctx, cfg, 256)
			So(low, ShouldBeGreaterThan, high)
			So(l, memlogger.ShouldHaveLog, logging.Warning, "Invalid shard: 256")
		})
	})
}

func TestFireTasks(t *testing.T) {
	t.Parallel()

	Convey("fireTasks works as expected", t, func() {
		tt := &Testing{}
		ctx := tt.Context()
		tq := taskqueue.Get(ctx)

		Convey("empty", func() {
			So(fireTasks(ctx, tt.GetConfig(ctx), nil), ShouldBeTrue)
			So(len(tq.Testable().GetScheduledTasks()[baseName]), ShouldEqual, 0)
		})

		Convey("basic", func() {
			So(fireTasks(ctx, tt.GetConfig(ctx), map[taskShard]struct{}{
				taskShard{2, minTS}: {},
				taskShard{7, minTS}: {},

				// since DelayedMutations is false, this timew will be reset
				taskShard{5, mkTimestamp(tt.GetConfig(ctx), testclock.TestTimeUTC.Add(time.Minute))}: {},
			}), ShouldBeTrue)
			q := tq.Testable().GetScheduledTasks()[baseName]
			So(q["-62132730888_2"], ShouldResemble, &taskqueue.Task{
				Name:   "-62132730888_2",
				Method: "POST",
				Path:   processURL(-62132730888, 2),
				ETA:    testclock.TestTimeUTC.Add(6 * time.Second).Round(time.Second),
			})
			So(q["-62132730888_7"], ShouldResemble, &taskqueue.Task{
				Name:   "-62132730888_7",
				Method: "POST",
				Path:   processURL(-62132730888, 7),
				ETA:    testclock.TestTimeUTC.Add(6 * time.Second).Round(time.Second),
			})
			So(q["-62132730888_5"], ShouldResemble, &taskqueue.Task{
				Name:   "-62132730888_5",
				Method: "POST",
				Path:   processURL(-62132730888, 5),
				ETA:    testclock.TestTimeUTC.Add(6 * time.Second).Round(time.Second),
			})
		})

		Convey("delayed", func() {
			cfg := tt.GetConfig(ctx)
			cfg.DelayedMutations = true
			tt.UpdateSettings(ctx, cfg)
			delayedTS := mkTimestamp(cfg, testclock.TestTimeUTC.Add(time.Minute*10))
			So(fireTasks(ctx, cfg, map[taskShard]struct{}{
				taskShard{1, delayedTS}: {},
			}), ShouldBeTrue)
			q := tq.Testable().GetScheduledTasks()[baseName]
			So(q["-62132730288_1"], ShouldResemble, &taskqueue.Task{
				Name:   "-62132730288_1",
				Method: "POST",
				Path:   processURL(-62132730288, 1),
				ETA:    testclock.TestTimeUTC.Add(time.Minute * 10).Add(6 * time.Second).Round(time.Second),
			})
		})
	})
}
