// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package tsmon

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/luci/gae/service/datastore"
	"github.com/luci/luci-go/common/tsmon"
	"github.com/luci/luci-go/common/tsmon/field"
	"github.com/luci/luci-go/common/tsmon/store"
	"github.com/luci/luci-go/common/tsmon/store/storetest"
	"github.com/luci/luci-go/common/tsmon/target"
	"github.com/luci/luci-go/common/tsmon/types"
	"github.com/luci/luci-go/server/router"

	. "github.com/smartystreets/goconvey/convey"
)

func TestMiddleware(t *testing.T) {
	t.Parallel()
	metric := &storetest.FakeMetric{
		types.MetricInfo{"m", "", []field.Field{}, types.CumulativeIntType},
		types.MetricMetadata{}}

	f := func(c *router.Context) {
		So(store.IsNilStore(tsmon.Store(c.Context)), ShouldBeFalse)
		tsmon.Register(c.Context, metric)
		So(tsmon.Store(c.Context).Incr(c.Context, metric, time.Time{}, []interface{}{}, int64(1)), ShouldBeNil)
	}

	Convey("Creates instance entity", t, func() {
		c, _ := buildGAETestContext()
		state, monitor := buildTestState()
		ds := datastore.Get(c)

		exists, err := ds.Exists(ds.NewKey("Instance", instanceEntityID(c), 0, nil))
		So(err, ShouldBeNil)
		So(exists.All(), ShouldBeFalse)

		rec := httptest.NewRecorder()
		router.RunMiddleware(
			&router.Context{Context: c, Writer: rec, Request: &http.Request{}},
			router.NewMiddlewareChain(state.Middleware),
			f,
		)
		So(rec.Code, ShouldEqual, http.StatusOK)

		exists, err = ds.Exists(ds.NewKey("Instance", instanceEntityID(c), 0, nil))
		So(err, ShouldBeNil)
		So(exists.All(), ShouldBeTrue)

		// Shouldn't flush since the instance entity doesn't have a task number yet.
		So(len(monitor.Cells), ShouldEqual, 0)
	})

	Convey("Flushes after 2 minutes", t, func() {
		c, clock := buildGAETestContext()
		state, monitor := buildTestState()

		ds := datastore.Get(c)

		i := instance{
			ID:          instanceEntityID(c),
			TaskNum:     0,
			LastUpdated: clock.Now().Add(-2 * time.Minute).UTC(),
		}
		So(ds.Put(&i), ShouldBeNil)

		state.lastFlushed = clock.Now().Add(-2 * time.Minute)

		rec := httptest.NewRecorder()
		router.RunMiddleware(
			&router.Context{Context: c, Writer: rec, Request: &http.Request{}},
			router.NewMiddlewareChain(state.Middleware),
			f,
		)
		So(rec.Code, ShouldEqual, http.StatusOK)

		So(len(monitor.Cells), ShouldEqual, 1)
		So(monitor.Cells[0][0].Name, ShouldEqual, "m")
		So(monitor.Cells[0][0].Value, ShouldEqual, int64(1))

		// Flushing should update the LastUpdated time.
		inst, err := getOrCreateInstanceEntity(c)
		So(err, ShouldBeNil)
		So(inst.LastUpdated, ShouldResemble, clock.Now().Round(time.Second))

		// The value should still be set.
		value, err := tsmon.Store(c).Get(c, metric, time.Time{}, []interface{}{})
		So(err, ShouldBeNil)
		So(value, ShouldEqual, int64(1))
	})

	Convey("Resets cumulative metrics", t, func() {
		c, clock := buildGAETestContext()
		state, monitor := buildTestState()

		state.lastFlushed = clock.Now().Add(-2 * time.Minute)

		rec := httptest.NewRecorder()
		router.RunMiddleware(
			&router.Context{Context: c, Writer: rec, Request: &http.Request{}},
			router.NewMiddlewareChain(state.Middleware),
			func(c *router.Context) {
				f(c)
				// Override the TaskNum here - it's created just before this handler runs
				// and used just after.
				tar := tsmon.Store(c.Context).DefaultTarget().(*target.Task)
				tar.TaskNum = proto.Int32(int32(0))
				tsmon.Store(c.Context).SetDefaultTarget(tar)
			},
		)
		So(rec.Code, ShouldEqual, http.StatusOK)

		So(len(tsmon.GetState(c).RegisteredMetrics), ShouldEqual, 1)
		So(len(monitor.Cells), ShouldEqual, 0)

		// Value should be reset.
		value, err := tsmon.Store(c).Get(c, metric, time.Time{}, []interface{}{})
		So(err, ShouldBeNil)
		So(value, ShouldBeNil)
	})

	Convey("Dynamic enable and disable works", t, func() {
		c, _ := buildGAETestContext()
		state, _ := buildTestState()

		// Enabled. Store is not nil.
		rec := httptest.NewRecorder()
		router.RunMiddleware(
			&router.Context{Context: c, Writer: rec, Request: &http.Request{}},
			router.NewMiddlewareChain(state.Middleware),
			func(c *router.Context) {
				So(store.IsNilStore(tsmon.Store(c.Context)), ShouldBeFalse)
			},
		)
		So(rec.Code, ShouldEqual, http.StatusOK)

		// Disabled. Store is nil.
		state.testingSettings.Enabled = false
		router.RunMiddleware(
			&router.Context{Context: c, Writer: rec, Request: &http.Request{}},
			router.NewMiddlewareChain(state.Middleware),
			func(c *router.Context) {
				So(store.IsNilStore(tsmon.Store(c.Context)), ShouldBeTrue)
			},
		)
		So(rec.Code, ShouldEqual, http.StatusOK)
	})
}
