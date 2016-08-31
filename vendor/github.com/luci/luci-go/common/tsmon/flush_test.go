// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package tsmon

import (
	"testing"
	"time"

	"github.com/golang/protobuf/proto"
	"golang.org/x/net/context"

	"github.com/luci/luci-go/common/clock"
	"github.com/luci/luci-go/common/clock/testclock"
	"github.com/luci/luci-go/common/tsmon/field"
	"github.com/luci/luci-go/common/tsmon/target"
	"github.com/luci/luci-go/common/tsmon/types"

	pb "github.com/luci/luci-go/common/tsmon/ts_mon_proto"

	. "github.com/smartystreets/goconvey/convey"
)

func TestFlush(t *testing.T) {
	c := context.Background()

	defaultTarget := (*target.Task)(&pb.Task{
		ServiceName: proto.String("test"),
	})

	Convey("Sends a metric", t, func() {
		c, s, m := WithFakes(c)
		s.Cells = []types.Cell{
			{
				types.MetricInfo{
					Name:      "foo",
					Fields:    []field.Field{},
					ValueType: types.StringType,
				},
				types.MetricMetadata{},
				types.CellData{
					FieldVals: []interface{}{},
					ResetTime: time.Unix(1234, 1000),
					Value:     "bar",
				},
			},
		}
		s.DT = defaultTarget
		m.CS = 42

		So(Flush(c), ShouldBeNil)

		So(len(m.Cells), ShouldEqual, 1)
		So(len(m.Cells[0]), ShouldEqual, 1)
		So(m.Cells[0][0], ShouldResemble, types.Cell{
			types.MetricInfo{
				Name:      "foo",
				Fields:    []field.Field{},
				ValueType: types.StringType,
			},
			types.MetricMetadata{},
			types.CellData{
				FieldVals: []interface{}{},
				ResetTime: time.Unix(1234, 1000),
				Value:     "bar",
			},
		})
	})

	Convey("Splits up ChunkSize metrics", t, func() {
		c, s, m := WithFakes(c)
		s.Cells = make([]types.Cell, 43)
		s.DT = defaultTarget
		m.CS = 42

		for i := 0; i < 43; i++ {
			s.Cells[i] = types.Cell{
				types.MetricInfo{
					Name:      "foo",
					Fields:    []field.Field{},
					ValueType: types.StringType,
				},
				types.MetricMetadata{},
				types.CellData{
					FieldVals: []interface{}{},
					ResetTime: time.Unix(1234, 1000),
					Value:     "bar",
				},
			}
		}

		So(Flush(c), ShouldBeNil)

		So(len(m.Cells), ShouldEqual, 2)
		So(len(m.Cells[0]), ShouldEqual, 42)
		So(len(m.Cells[1]), ShouldEqual, 1)
	})

	Convey("Doesn't split metrics when ChunkSize is 0", t, func() {
		c, s, m := WithFakes(c)
		s.Cells = make([]types.Cell, 43)
		s.DT = defaultTarget
		m.CS = 0

		for i := 0; i < 43; i++ {
			s.Cells[i] = types.Cell{
				types.MetricInfo{
					Name:      "foo",
					Fields:    []field.Field{},
					ValueType: types.StringType,
				},
				types.MetricMetadata{},
				types.CellData{
					FieldVals: []interface{}{},
					ResetTime: time.Unix(1234, 1000),
					Value:     "bar",
				},
			}
		}

		So(Flush(c), ShouldBeNil)

		So(len(m.Cells), ShouldEqual, 1)
		So(len(m.Cells[0]), ShouldEqual, 43)
	})

	Convey("No Monitor configured", t, func() {
		c, _, _ := WithFakes(c)
		state := GetState(c)
		state.M = nil

		So(Flush(c), ShouldNotBeNil)
	})

	Convey("Auto flush works", t, func() {
		start := time.Unix(1454561232, 0)
		c, tc := testclock.UseTime(c, start)
		tc.SetTimerCallback(func(d time.Duration, t clock.Timer) {
			tc.Add(d)
		})

		moments := make(chan int)
		flusher := autoFlusher{
			flush: func(c context.Context) error {
				select {
				case <-c.Done():
				case moments <- int(clock.Now(c).Sub(start).Seconds()):
				}
				return nil
			},
		}

		flusher.start(c, time.Second)

		// Each 'flush' gets blocked on sending into 'moments'. Once unblocked, it
		// advances timer by 'interval' sec (1 sec in the test).
		So(<-moments, ShouldEqual, 1)
		So(<-moments, ShouldEqual, 2)
		// and so on ...

		// Doesn't timeout => works.
		flusher.stop()
	})
}
