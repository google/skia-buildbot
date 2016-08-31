// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package metric

import (
	"testing"

	"github.com/luci/luci-go/common/tsmon"
	"github.com/luci/luci-go/common/tsmon/distribution"
	"github.com/luci/luci-go/common/tsmon/types"
	"golang.org/x/net/context"

	. "github.com/smartystreets/goconvey/convey"
)

func makeContext() context.Context {
	ret, _ := tsmon.WithDummyInMemory(context.Background())
	return ret
}

func TestMetrics(t *testing.T) {
	Convey("Int", t, func() {
		c := makeContext()
		m := NewIntIn(c, "foo", "description", types.MetricMetadata{})

		v, err := m.Get(c)
		So(v, ShouldEqual, 0)
		So(err, ShouldBeNil)

		v, err = m.Get(c, "field")
		So(err, ShouldNotBeNil)

		err = m.Set(c, 42)
		So(err, ShouldBeNil)

		v, err = m.Get(c)
		So(v, ShouldEqual, 42)
		So(err, ShouldBeNil)

		err = m.Set(c, 42, "field")
		So(err, ShouldNotBeNil)

		So(func() { NewIntIn(c, "foo", "description", types.MetricMetadata{}) }, ShouldPanic)
	})

	Convey("Counter", t, func() {
		c := makeContext()
		m := NewCounterIn(c, "foo", "description", types.MetricMetadata{})

		v, err := m.Get(c)
		So(v, ShouldEqual, 0)
		So(err, ShouldBeNil)

		err = m.Add(c, 3)
		So(err, ShouldBeNil)

		v, err = m.Get(c)
		So(v, ShouldEqual, 3)
		So(err, ShouldBeNil)

		err = m.Add(c, 2)
		So(err, ShouldBeNil)

		v, err = m.Get(c)
		So(v, ShouldEqual, 5)
		So(err, ShouldBeNil)

		So(func() { NewCounterIn(c, "foo", "description", types.MetricMetadata{}) }, ShouldPanic)
	})

	Convey("Float", t, func() {
		c := makeContext()
		m := NewFloatIn(c, "foo", "description", types.MetricMetadata{})

		v, err := m.Get(c)
		So(v, ShouldAlmostEqual, 0.0)
		So(err, ShouldBeNil)

		v, err = m.Get(c, "field")
		So(err, ShouldNotBeNil)

		err = m.Set(c, 42.3)
		So(err, ShouldBeNil)

		v, err = m.Get(c)
		So(v, ShouldAlmostEqual, 42.3)
		So(err, ShouldBeNil)

		err = m.Set(c, 42.3, "field")
		So(err, ShouldNotBeNil)

		So(func() { NewFloatIn(c, "foo", "description", types.MetricMetadata{}) }, ShouldPanic)
	})

	Convey("FloatCounter", t, func() {
		c := makeContext()
		m := NewFloatCounterIn(c, "foo", "description", types.MetricMetadata{})

		v, err := m.Get(c)
		So(v, ShouldAlmostEqual, 0.0)
		So(err, ShouldBeNil)

		err = m.Add(c, 3.1)
		So(err, ShouldBeNil)

		v, err = m.Get(c)
		So(v, ShouldAlmostEqual, 3.1)
		So(err, ShouldBeNil)

		err = m.Add(c, 2.2)
		So(err, ShouldBeNil)

		v, err = m.Get(c)
		So(v, ShouldAlmostEqual, 5.3)
		So(err, ShouldBeNil)

		So(func() { NewFloatCounterIn(c, "foo", "description", types.MetricMetadata{}) }, ShouldPanic)
	})

	Convey("String", t, func() {
		c := makeContext()
		m := NewStringIn(c, "foo", "description", types.MetricMetadata{})

		v, err := m.Get(c)
		So(v, ShouldEqual, "")
		So(err, ShouldBeNil)

		v, err = m.Get(c, "field")
		So(err, ShouldNotBeNil)

		err = m.Set(c, "hello")
		So(err, ShouldBeNil)

		v, err = m.Get(c)
		So(v, ShouldEqual, "hello")
		So(err, ShouldBeNil)

		err = m.Set(c, "hello", "field")
		So(err, ShouldNotBeNil)

		So(func() { NewStringIn(c, "foo", "description", types.MetricMetadata{}) }, ShouldPanic)
	})

	Convey("Bool", t, func() {
		c := makeContext()
		m := NewBoolIn(c, "foo", "description", types.MetricMetadata{})

		v, err := m.Get(c)
		So(v, ShouldEqual, false)
		So(err, ShouldBeNil)

		v, err = m.Get(c, "field")
		So(err, ShouldNotBeNil)

		err = m.Set(c, true)
		So(err, ShouldBeNil)

		v, err = m.Get(c)
		So(v, ShouldEqual, true)
		So(err, ShouldBeNil)

		err = m.Set(c, true, "field")
		So(err, ShouldNotBeNil)

		So(func() { NewBoolIn(c, "foo", "description", types.MetricMetadata{}) }, ShouldPanic)
	})

	Convey("CumulativeDistribution", t, func() {
		c := makeContext()
		m := NewCumulativeDistributionIn(c, "foo", "description", types.MetricMetadata{}, distribution.FixedWidthBucketer(10, 20))

		v, err := m.Get(c)
		So(v, ShouldBeNil)
		So(err, ShouldBeNil)

		So(m.Bucketer().GrowthFactor(), ShouldEqual, 0)
		So(m.Bucketer().Width(), ShouldEqual, 10)
		So(m.Bucketer().NumFiniteBuckets(), ShouldEqual, 20)

		v, err = m.Get(c, "field")
		So(err, ShouldNotBeNil)

		err = m.Add(c, 5)
		So(err, ShouldBeNil)

		v, err = m.Get(c)
		So(v.Bucketer().GrowthFactor(), ShouldEqual, 0)
		So(v.Bucketer().Width(), ShouldEqual, 10)
		So(v.Bucketer().NumFiniteBuckets(), ShouldEqual, 20)
		So(v.Sum(), ShouldEqual, 5)
		So(v.Count(), ShouldEqual, 1)
		So(err, ShouldBeNil)

		So(func() { NewCumulativeDistributionIn(c, "foo", "description", types.MetricMetadata{}, m.Bucketer()) }, ShouldPanic)
	})

	Convey("NonCumulativeDistribution", t, func() {
		c := makeContext()
		m := NewNonCumulativeDistributionIn(c, "foo", "description", types.MetricMetadata{}, distribution.FixedWidthBucketer(10, 20))

		v, err := m.Get(c)
		So(v, ShouldBeNil)
		So(err, ShouldBeNil)

		So(m.Bucketer().GrowthFactor(), ShouldEqual, 0)
		So(m.Bucketer().Width(), ShouldEqual, 10)
		So(m.Bucketer().NumFiniteBuckets(), ShouldEqual, 20)

		v, err = m.Get(c, "field")
		So(err, ShouldNotBeNil)

		d := distribution.New(m.Bucketer())
		d.Add(15)
		err = m.Set(c, d)
		So(err, ShouldBeNil)

		v, err = m.Get(c)
		So(v.Bucketer().GrowthFactor(), ShouldEqual, 0)
		So(v.Bucketer().Width(), ShouldEqual, 10)
		So(v.Bucketer().NumFiniteBuckets(), ShouldEqual, 20)
		So(v.Sum(), ShouldEqual, 15)
		So(v.Count(), ShouldEqual, 1)
		So(err, ShouldBeNil)

		So(func() { NewNonCumulativeDistributionIn(c, "foo", "description", types.MetricMetadata{}, m.Bucketer()) }, ShouldPanic)
	})
}
