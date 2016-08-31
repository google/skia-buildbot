// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

// Package storetest is imported exclusively by tests for Store implementations.
package storetest

import (
	"bytes"
	"fmt"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/luci/luci-go/common/clock/testclock"
	"github.com/luci/luci-go/common/tsmon/distribution"
	"github.com/luci/luci-go/common/tsmon/field"
	"github.com/luci/luci-go/common/tsmon/monitor"
	"github.com/luci/luci-go/common/tsmon/target"
	"github.com/luci/luci-go/common/tsmon/types"
	"golang.org/x/net/context"

	pb "github.com/luci/luci-go/common/tsmon/ts_mon_proto"
	. "github.com/smartystreets/goconvey/convey"
)

// Store is a store under test.
//
// It is a copy of store.Store interface to break module dependency cycle.
type Store interface {
	Register(m types.Metric)
	Unregister(m types.Metric)

	DefaultTarget() types.Target
	SetDefaultTarget(t types.Target)

	Get(c context.Context, m types.Metric, resetTime time.Time, fieldVals []interface{}) (value interface{}, err error)
	Set(c context.Context, m types.Metric, resetTime time.Time, fieldVals []interface{}, value interface{}) error
	Incr(c context.Context, m types.Metric, resetTime time.Time, fieldVals []interface{}, delta interface{}) error

	GetAll(c context.Context) []types.Cell

	Reset(c context.Context, m types.Metric)
}

// TestOptions contains options for RunStoreImplementationTests.
type TestOptions struct {
	// Factory creates and returns a new Store implementation.
	Factory func() Store

	// RegistrationFinished is called after all metrics have been registered.
	// Store implementations that need to do expensive initialization (that would
	// otherwise be done in iface.go) can do that here.
	RegistrationFinished func(Store)

	// GetNumRegisteredMetrics returns the number of metrics registered in the
	// store.
	GetNumRegisteredMetrics func(Store) int
}

// RunStoreImplementationTests runs all the standard tests that all store
// implementations are expected to pass.  When you write a new Store
// implementation you should ensure you run these tests against it.
func RunStoreImplementationTests(t *testing.T, ctx context.Context, opts TestOptions) {
	distOne := distribution.New(distribution.DefaultBucketer)
	distOne.Add(4.2)

	distTwo := distribution.New(distribution.DefaultBucketer)
	distTwo.Add(5.6)
	distTwo.Add(1.0)

	tests := []struct {
		typ      types.ValueType
		bucketer *distribution.Bucketer

		values           []interface{}
		wantSetSuccess   bool
		wantSetValidator func(interface{}, interface{})

		deltas            []interface{}
		wantIncrSuccess   bool
		wantIncrValue     interface{}
		wantIncrValidator func(interface{})

		wantStartTimestamp bool
	}{
		{
			typ:                types.CumulativeIntType,
			values:             makeInterfaceSlice(int64(3), int64(4)),
			deltas:             makeInterfaceSlice(int64(3), int64(4)),
			wantSetSuccess:     true,
			wantIncrSuccess:    true,
			wantIncrValue:      int64(7),
			wantStartTimestamp: true,
		},
		{
			typ:                types.CumulativeFloatType,
			values:             makeInterfaceSlice(float64(3.2), float64(4.3)),
			deltas:             makeInterfaceSlice(float64(3.2), float64(4.3)),
			wantSetSuccess:     true,
			wantIncrSuccess:    true,
			wantIncrValue:      float64(7.5),
			wantStartTimestamp: true,
		},
		{
			typ:             types.CumulativeDistributionType,
			bucketer:        distribution.DefaultBucketer,
			values:          makeInterfaceSlice(distOne, distTwo),
			deltas:          makeInterfaceSlice(float64(3.2), float64(5.3)),
			wantSetSuccess:  true,
			wantIncrSuccess: true,
			wantIncrValidator: func(v interface{}) {
				d := v.(*distribution.Distribution)
				So(d.Buckets(), ShouldResemble, []int64{0, 0, 0, 0, 1, 1})
			},
			wantStartTimestamp: true,
		},
		{
			typ:             types.NonCumulativeIntType,
			values:          makeInterfaceSlice(int64(3), int64(4)),
			deltas:          makeInterfaceSlice(int64(3), int64(4)),
			wantSetSuccess:  true,
			wantIncrSuccess: false,
		},
		{
			typ:             types.NonCumulativeFloatType,
			values:          makeInterfaceSlice(float64(3.2), float64(4.3)),
			deltas:          makeInterfaceSlice(float64(3.2), float64(4.3)),
			wantSetSuccess:  true,
			wantIncrSuccess: false,
		},
		{
			typ:             types.NonCumulativeDistributionType,
			bucketer:        distribution.DefaultBucketer,
			values:          makeInterfaceSlice(distOne, distTwo),
			deltas:          makeInterfaceSlice(float64(3.2), float64(5.3)),
			wantSetSuccess:  true,
			wantIncrSuccess: false,
			wantSetValidator: func(got, want interface{}) {
				// The distribution might be serialized/deserialized and lose sums and
				// counts.
				So(got.(*distribution.Distribution).Buckets(), ShouldResemble,
					want.(*distribution.Distribution).Buckets())
			},
		},
		{
			typ:             types.StringType,
			values:          makeInterfaceSlice("hello", "world"),
			deltas:          makeInterfaceSlice("hello", "world"),
			wantSetSuccess:  true,
			wantIncrSuccess: false,
		},
		{
			typ:             types.BoolType,
			values:          makeInterfaceSlice(true, false),
			deltas:          makeInterfaceSlice(true, false),
			wantSetSuccess:  true,
			wantIncrSuccess: false,
		},
	}

	Convey("Set and get", t, func() {
		Convey("With no fields", func() {
			for i, test := range tests {
				Convey(fmt.Sprintf("%d. %s", i, test.typ), func() {
					var m types.Metric
					if test.bucketer != nil {
						m = &fakeDistributionMetric{FakeMetric{
							types.MetricInfo{"m", "", []field.Field{}, test.typ},
							types.MetricMetadata{}}, test.bucketer}
					} else {
						m = &FakeMetric{
							types.MetricInfo{"m", "", []field.Field{}, test.typ},
							types.MetricMetadata{},
						}
					}

					s := opts.Factory()
					s.Register(m)

					// Value should be nil initially.
					v, err := s.Get(ctx, m, time.Time{}, []interface{}{})
					So(err, ShouldBeNil)
					So(v, ShouldBeNil)

					// Set and get the value.
					err = s.Set(ctx, m, time.Time{}, []interface{}{}, test.values[0])
					if !test.wantSetSuccess {
						So(err, ShouldNotBeNil)
					} else {
						So(err, ShouldBeNil)
						v, err := s.Get(ctx, m, time.Time{}, []interface{}{})
						So(err, ShouldBeNil)
						if test.wantSetValidator != nil {
							test.wantSetValidator(v, test.values[0])
						} else {
							So(v, ShouldEqual, test.values[0])
						}
					}
				})
			}
		})

		Convey("With fields", func() {
			for i, test := range tests {
				Convey(fmt.Sprintf("%d. %s", i, test.typ), func() {
					var m types.Metric
					if test.bucketer != nil {
						m = &fakeDistributionMetric{FakeMetric{
							types.MetricInfo{"m", "", []field.Field{field.String("f")}, test.typ},
							types.MetricMetadata{}}, test.bucketer}
					} else {
						m = &FakeMetric{
							types.MetricInfo{"m", "", []field.Field{field.String("f")}, test.typ},
							types.MetricMetadata{}}
					}

					s := opts.Factory()
					s.Register(m)

					// Values should be nil initially.
					v, err := s.Get(ctx, m, time.Time{}, makeInterfaceSlice("one"))
					So(err, ShouldBeNil)
					So(v, ShouldBeNil)
					v, err = s.Get(ctx, m, time.Time{}, makeInterfaceSlice("two"))
					So(err, ShouldBeNil)
					So(v, ShouldBeNil)

					// Set and get the values.
					err = s.Set(ctx, m, time.Time{}, makeInterfaceSlice("one"), test.values[0])
					if !test.wantSetSuccess {
						So(err, ShouldNotBeNil)
					} else {
						So(err, ShouldBeNil)
					}

					err = s.Set(ctx, m, time.Time{}, makeInterfaceSlice("two"), test.values[1])
					if !test.wantSetSuccess {
						So(err, ShouldNotBeNil)
						return
					}
					So(err, ShouldBeNil)

					v, err = s.Get(ctx, m, time.Time{}, makeInterfaceSlice("one"))
					So(err, ShouldBeNil)
					if test.wantSetValidator != nil {
						test.wantSetValidator(v, test.values[0])
					} else {
						So(v, ShouldEqual, test.values[0])
					}
					v, err = s.Get(ctx, m, time.Time{}, makeInterfaceSlice("two"))
					So(err, ShouldBeNil)
					if test.wantSetValidator != nil {
						test.wantSetValidator(v, test.values[1])
					} else {
						So(v, ShouldEqual, test.values[1])
					}
				})
			}
		})

		Convey("With a fixed reset time", func() {
			for i, test := range tests {
				if !test.wantSetSuccess {
					continue
				}

				Convey(fmt.Sprintf("%d. %s", i, test.typ), func() {
					var m types.Metric
					if test.bucketer != nil {
						m = &fakeDistributionMetric{FakeMetric{
							types.MetricInfo{"m", "", []field.Field{}, test.typ},
							types.MetricMetadata{}}, test.bucketer}
					} else {
						m = &FakeMetric{
							types.MetricInfo{"m", "", []field.Field{}, test.typ},
							types.MetricMetadata{}}
					}

					s := opts.Factory()
					s.Register(m)
					opts.RegistrationFinished(s)

					// Do the set with a fixed time.
					t := time.Date(1972, 5, 6, 7, 8, 9, 0, time.UTC)
					So(s.Set(ctx, m, t, []interface{}{}, test.values[0]), ShouldBeNil)

					v, err := s.Get(ctx, m, time.Time{}, []interface{}{})
					So(err, ShouldBeNil)
					if test.wantSetValidator != nil {
						test.wantSetValidator(v, test.values[0])
					} else {
						So(v, ShouldEqual, test.values[0])
					}

					// Check the time in the Cell is the same.
					all := s.GetAll(ctx)
					So(len(all), ShouldEqual, 1)

					msg := monitor.SerializeCell(all[0])
					if test.wantStartTimestamp {
						So(time.Unix(0, int64(msg.GetStartTimestampUs()*uint64(time.Microsecond))).UTC().String(),
							ShouldEqual, t.String())
					} else {
						So(msg.GetStartTimestampUs(), ShouldEqual, 0)
					}
				})
			}
		})

		Convey("With a target set in the context", func() {
			for i, test := range tests {
				if !test.wantSetSuccess {
					continue
				}

				Convey(fmt.Sprintf("%d. %s", i, test.typ), func() {
					var m types.Metric
					if test.bucketer != nil {
						m = &fakeDistributionMetric{FakeMetric{
							types.MetricInfo{"m", "", []field.Field{}, test.typ},
							types.MetricMetadata{}}, test.bucketer}
					} else {
						m = &FakeMetric{
							types.MetricInfo{"m", "", []field.Field{}, test.typ},
							types.MetricMetadata{}}
					}

					s := opts.Factory()
					s.Register(m)
					opts.RegistrationFinished(s)

					// Create a context with a different target.
					t := target.Task{}
					t.AsProto().ServiceName = proto.String("foo")
					ctxWithTarget := target.Set(ctx, &t)

					// Set the first value on the default target, second value on the
					// different target.
					So(s.Set(ctx, m, time.Time{}, []interface{}{}, test.values[0]), ShouldBeNil)
					So(s.Set(ctxWithTarget, m, time.Time{}, []interface{}{}, test.values[1]), ShouldBeNil)

					// Get should return different values for different contexts.
					v, err := s.Get(ctx, m, time.Time{}, []interface{}{})
					So(err, ShouldBeNil)
					if test.wantSetValidator != nil {
						test.wantSetValidator(v, test.values[0])
					} else {
						So(v, ShouldEqual, test.values[0])
					}

					v, err = s.Get(ctxWithTarget, m, time.Time{}, []interface{}{})
					So(err, ShouldBeNil)
					if test.wantSetValidator != nil {
						test.wantSetValidator(v, test.values[1])
					} else {
						So(v, ShouldEqual, test.values[1])
					}

					// The targets should be set in the Cells.
					all := s.GetAll(ctx)
					So(len(all), ShouldEqual, 2)

					coll := monitor.SerializeCells(all)
					sort.Sort(sortableDataSlice(coll.Data))
					So(coll.Data[0].Task.GetServiceName(), ShouldEqual,
						s.DefaultTarget().(*target.Task).AsProto().GetServiceName())
					So(coll.Data[1].Task.GetServiceName(), ShouldEqual, t.AsProto().GetServiceName())
				})
			}
		})

		Convey("With a decreasing value", func() {
			for i, test := range tests {
				if !test.typ.IsCumulative() || test.bucketer != nil {
					continue
				}
				Convey(fmt.Sprintf("%d. %s", i, test.typ), func() {
					m := &FakeMetric{
						types.MetricInfo{"m", "", []field.Field{}, test.typ},
						types.MetricMetadata{}}
					s := opts.Factory()
					s.Register(m)

					// Set the bigger value.
					So(s.Set(ctx, m, time.Time{}, []interface{}{}, test.values[1]), ShouldBeNil)

					// Setting the smaller value should fail.
					So(s.Set(ctx, m, time.Time{}, []interface{}{}, test.values[0]), ShouldNotBeNil)
				})
			}
		})
	})

	Convey("Increment and get", t, func() {
		Convey("With no fields", func() {
			for i, test := range tests {
				Convey(fmt.Sprintf("%d. %s", i, test.typ), func() {
					var m types.Metric
					if test.bucketer != nil {
						m = &fakeDistributionMetric{FakeMetric{
							types.MetricInfo{"m", "", []field.Field{}, test.typ},
							types.MetricMetadata{}}, test.bucketer}

					} else {
						m = &FakeMetric{
							types.MetricInfo{"m", "", []field.Field{}, test.typ},
							types.MetricMetadata{}}
					}

					s := opts.Factory()
					s.Register(m)

					// Value should be nil initially.
					v, err := s.Get(ctx, m, time.Time{}, []interface{}{})
					So(err, ShouldBeNil)
					So(v, ShouldBeNil)

					// Increment the metric.
					for _, delta := range test.deltas {
						err = s.Incr(ctx, m, time.Time{}, []interface{}{}, delta)

						if !test.wantIncrSuccess {
							So(err, ShouldNotBeNil)
						} else {
							So(err, ShouldBeNil)
						}
					}

					// Get the final value.
					v, err = s.Get(ctx, m, time.Time{}, []interface{}{})
					So(err, ShouldBeNil)

					if test.wantIncrValue != nil {
						So(v, ShouldEqual, test.wantIncrValue)
					} else if test.wantIncrValidator != nil {
						test.wantIncrValidator(v)
					}
				})
			}
		})

		Convey("With fields", func() {
			for i, test := range tests {
				Convey(fmt.Sprintf("%d. %s", i, test.typ), func() {
					var m types.Metric
					if test.bucketer != nil {
						m = &fakeDistributionMetric{FakeMetric{
							types.MetricInfo{"m", "", []field.Field{field.String("f")}, test.typ},
							types.MetricMetadata{}}, test.bucketer}
					} else {
						m = &FakeMetric{
							types.MetricInfo{"m", "", []field.Field{field.String("f")}, test.typ},
							types.MetricMetadata{}}
					}

					s := opts.Factory()
					s.Register(m)

					// Values should be nil initially.
					v, err := s.Get(ctx, m, time.Time{}, makeInterfaceSlice("one"))
					So(err, ShouldBeNil)
					So(v, ShouldBeNil)
					v, err = s.Get(ctx, m, time.Time{}, makeInterfaceSlice("two"))
					So(err, ShouldBeNil)
					So(v, ShouldBeNil)

					// Increment one cell.
					for _, delta := range test.deltas {
						err = s.Incr(ctx, m, time.Time{}, makeInterfaceSlice("one"), delta)

						if !test.wantIncrSuccess {
							So(err, ShouldNotBeNil)
						} else {
							So(err, ShouldBeNil)
						}
					}

					// Get the final value.
					v, err = s.Get(ctx, m, time.Time{}, makeInterfaceSlice("one"))
					So(err, ShouldBeNil)

					if test.wantIncrValue != nil {
						So(v, ShouldEqual, test.wantIncrValue)
					} else if test.wantIncrValidator != nil {
						test.wantIncrValidator(v)
					}

					// Another cell should still be nil.
					v, err = s.Get(ctx, m, time.Time{}, makeInterfaceSlice("two"))
					So(err, ShouldBeNil)
					So(v, ShouldBeNil)
				})
			}
		})

		Convey("With a fixed reset time", func() {
			for i, test := range tests {
				if !test.wantIncrSuccess {
					continue
				}

				Convey(fmt.Sprintf("%d. %s", i, test.typ), func() {
					var m types.Metric
					if test.bucketer != nil {
						m = &fakeDistributionMetric{FakeMetric{
							types.MetricInfo{"m", "", []field.Field{}, test.typ},
							types.MetricMetadata{}}, test.bucketer}
					} else {
						m = &FakeMetric{
							types.MetricInfo{"m", "", []field.Field{}, test.typ},
							types.MetricMetadata{}}
					}

					s := opts.Factory()
					s.Register(m)
					opts.RegistrationFinished(s)

					// Do the incr with a fixed time.
					t := time.Date(1972, 5, 6, 7, 8, 9, 0, time.UTC)
					So(s.Incr(ctx, m, t, []interface{}{}, test.deltas[0]), ShouldBeNil)

					// Check the time in the Cell is the same.
					all := s.GetAll(ctx)
					So(len(all), ShouldEqual, 1)

					msg := monitor.SerializeCell(all[0])
					if test.wantStartTimestamp {
						So(time.Unix(0, int64(msg.GetStartTimestampUs()*uint64(time.Microsecond))).UTC().String(),
							ShouldEqual, t.String())
					} else {
						So(msg.GetStartTimestampUs(), ShouldEqual, 0)
					}
				})
			}
		})

		Convey("With a target set in the context", func() {
			for i, test := range tests {
				if !test.wantIncrSuccess {
					continue
				}

				Convey(fmt.Sprintf("%d. %s", i, test.typ), func() {
					var m types.Metric
					if test.bucketer != nil {
						m = &fakeDistributionMetric{
							FakeMetric{types.MetricInfo{"m", "", []field.Field{}, test.typ},
								types.MetricMetadata{}},
							test.bucketer}
					} else {
						m = &FakeMetric{
							types.MetricInfo{"m", "", []field.Field{}, test.typ},
							types.MetricMetadata{}}
					}

					s := opts.Factory()
					s.Register(m)
					opts.RegistrationFinished(s)

					// Create a context with a different target.
					t := target.Task{}
					t.AsProto().ServiceName = proto.String("foo")
					ctxWithTarget := target.Set(ctx, &t)

					// Incr the first delta on the default target, second delta on the
					// different target.
					So(s.Incr(ctx, m, time.Time{}, []interface{}{}, test.deltas[0]), ShouldBeNil)
					So(s.Incr(ctxWithTarget, m, time.Time{}, []interface{}{}, test.deltas[1]), ShouldBeNil)

					// Get should return different values for different contexts.
					v1, err := s.Get(ctx, m, time.Time{}, []interface{}{})
					So(err, ShouldBeNil)
					v2, err := s.Get(ctxWithTarget, m, time.Time{}, []interface{}{})
					So(err, ShouldBeNil)
					So(v1, ShouldNotEqual, v2)

					// The targets should be set in the Cells.
					all := s.GetAll(ctx)
					So(len(all), ShouldEqual, 2)

					coll := monitor.SerializeCells(all)
					sort.Sort(sortableDataSlice(coll.Data))
					So(coll.Data[0].Task.GetServiceName(), ShouldEqual,
						s.DefaultTarget().(*target.Task).AsProto().GetServiceName())
					So(coll.Data[1].Task.GetServiceName(), ShouldEqual, t.AsProto().GetServiceName())
				})
			}
		})
	})

	Convey("GetAll", t, func() {
		ctx, tc := testclock.UseTime(context.Background(), testclock.TestTimeLocal)

		s := opts.Factory()
		foo := &FakeMetric{
			types.MetricInfo{"foo", "", []field.Field{}, types.NonCumulativeIntType},
			types.MetricMetadata{}}
		bar := &FakeMetric{
			types.MetricInfo{"bar", "", []field.Field{field.String("f")}, types.StringType},
			types.MetricMetadata{}}
		baz := &FakeMetric{
			types.MetricInfo{"baz", "", []field.Field{field.String("f")}, types.NonCumulativeFloatType},
			types.MetricMetadata{}}
		s.Register(foo)
		s.Register(bar)
		s.Register(baz)
		opts.RegistrationFinished(s)

		// Add test records. We increment the test clock each time so that the added
		// records sort deterministically using sortableCellSlice.
		for _, m := range []struct {
			metric    types.Metric
			fieldvals []interface{}
			value     interface{}
		}{
			{foo, []interface{}{}, int64(42)},
			{bar, makeInterfaceSlice("one"), "hello"},
			{bar, makeInterfaceSlice("two"), "world"},
			{baz, makeInterfaceSlice("three"), 1.23},
			{baz, makeInterfaceSlice("four"), 4.56},
		} {
			So(s.Set(ctx, m.metric, time.Time{}, m.fieldvals, m.value), ShouldBeNil)
			tc.Add(time.Second)
		}

		got := s.GetAll(ctx)
		sort.Sort(sortableCellSlice(got))
		want := []types.Cell{
			{
				types.MetricInfo{
					Name:      "foo",
					Fields:    []field.Field{},
					ValueType: types.NonCumulativeIntType,
				},
				types.MetricMetadata{},
				types.CellData{
					FieldVals: []interface{}{},
					Value:     int64(42),
				},
			},
			{
				types.MetricInfo{
					Name:      "bar",
					Fields:    []field.Field{field.String("f")},
					ValueType: types.StringType,
				},
				types.MetricMetadata{},
				types.CellData{
					FieldVals: makeInterfaceSlice("one"),
					Value:     "hello",
				},
			},
			{
				types.MetricInfo{
					Name:      "bar",
					Fields:    []field.Field{field.String("f")},
					ValueType: types.StringType,
				},
				types.MetricMetadata{},
				types.CellData{
					FieldVals: makeInterfaceSlice("two"),
					Value:     "world",
				},
			},
			{
				types.MetricInfo{
					Name:      "baz",
					Fields:    []field.Field{field.String("f")},
					ValueType: types.NonCumulativeFloatType,
				},
				types.MetricMetadata{},
				types.CellData{
					FieldVals: makeInterfaceSlice("three"),
					Value:     1.23,
				},
			},
			{
				types.MetricInfo{
					Name:      "baz",
					Fields:    []field.Field{field.String("f")},
					ValueType: types.NonCumulativeFloatType,
				},
				types.MetricMetadata{},
				types.CellData{
					FieldVals: makeInterfaceSlice("four"),
					Value:     4.56,
				},
			},
		}
		So(len(got), ShouldEqual, len(want))

		for i, g := range got {
			w := want[i]

			Convey(fmt.Sprintf("%d", i), func() {
				So(g.Name, ShouldEqual, w.Name)
				So(len(g.Fields), ShouldEqual, len(w.Fields))
				So(g.ValueType, ShouldEqual, w.ValueType)
				So(g.FieldVals, ShouldResemble, w.FieldVals)
				So(g.Value, ShouldEqual, w.Value)
			})
		}
	})

	Convey("Concurrency", t, func() {
		const numIterations = 100
		const numGoroutines = 32

		Convey("Incr", func(c C) {
			s := opts.Factory()
			m := &FakeMetric{
				types.MetricInfo{"m", "", []field.Field{}, types.CumulativeIntType},
				types.MetricMetadata{}}
			s.Register(m)

			wg := sync.WaitGroup{}
			f := func(n int) {
				defer wg.Done()
				for i := 0; i < numIterations; i++ {
					c.So(s.Incr(ctx, m, time.Time{}, []interface{}{}, int64(1)), ShouldBeNil)
				}
			}

			for n := 0; n < numGoroutines; n++ {
				wg.Add(1)
				go f(n)
			}
			wg.Wait()

			val, err := s.Get(ctx, m, time.Time{}, []interface{}{})
			So(val, ShouldEqual, numIterations*numGoroutines)
			So(err, ShouldBeNil)
		})
	})

	Convey("Different targets", t, func() {
		Convey("Gets from context", func() {
			s := opts.Factory()
			m := &FakeMetric{
				types.MetricInfo{"m", "", []field.Field{}, types.NonCumulativeIntType},
				types.MetricMetadata{}}
			s.Register(m)
			opts.RegistrationFinished(s)

			t := target.Task{}
			t.AsProto().ServiceName = proto.String("foo")
			ctxWithTarget := target.Set(ctx, &t)

			So(s.Set(ctx, m, time.Time{}, []interface{}{}, int64(42)), ShouldBeNil)
			So(s.Set(ctxWithTarget, m, time.Time{}, []interface{}{}, int64(43)), ShouldBeNil)

			val, err := s.Get(ctx, m, time.Time{}, []interface{}{})
			So(err, ShouldBeNil)
			So(val, ShouldEqual, 42)

			val, err = s.Get(ctxWithTarget, m, time.Time{}, []interface{}{})
			So(err, ShouldBeNil)
			So(val, ShouldEqual, 43)

			all := s.GetAll(ctx)
			So(len(all), ShouldEqual, 2)

			// The order is undefined.
			if all[0].Value.(int64) == 42 {
				So(all[0].Target, ShouldResemble, s.DefaultTarget())
				So(all[1].Target, ShouldResemble, &t)
			} else {
				So(all[0].Target, ShouldResemble, &t)
				So(all[1].Target, ShouldResemble, s.DefaultTarget())
			}
		})
	})
}

func makeInterfaceSlice(v ...interface{}) []interface{} {
	return v
}

// FakeMetric is a fake Metric implementation.
type FakeMetric struct {
	types.MetricInfo
	types.MetricMetadata
}

// Info implements Metric.Info
func (m *FakeMetric) Info() types.MetricInfo { return m.MetricInfo }

// Metadata implements Metric.Metadata
func (m *FakeMetric) Metadata() types.MetricMetadata { return m.MetricMetadata }

// SetFixedResetTime implements Metric.SetFixedResetTime.
func (m *FakeMetric) SetFixedResetTime(t time.Time) {}

type fakeDistributionMetric struct {
	FakeMetric

	bucketer *distribution.Bucketer
}

func (m *fakeDistributionMetric) Bucketer() *distribution.Bucketer { return m.bucketer }

type sortableCellSlice []types.Cell

func (s sortableCellSlice) Len() int { return len(s) }
func (s sortableCellSlice) Less(i, j int) bool {
	return s[i].ResetTime.UnixNano() < s[j].ResetTime.UnixNano()
}
func (s sortableCellSlice) Swap(i, j int) { s[i], s[j] = s[j], s[i] }

type sortableDataSlice []*pb.MetricsData

func (s sortableDataSlice) Len() int { return len(s) }
func (s sortableDataSlice) Less(i, j int) bool {
	a, _ := proto.Marshal(s[i])
	b, _ := proto.Marshal(s[j])
	return bytes.Compare(a, b) > 0
}
func (s sortableDataSlice) Swap(i, j int) { s[i], s[j] = s[j], s[i] }
