// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package metric

import (
	"testing"
	"time"

	"github.com/luci/luci-go/common/tsmon"
	"golang.org/x/net/context"

	. "github.com/smartystreets/goconvey/convey"
)

func TestStandardMetrics(t *testing.T) {
	t.Parallel()

	durMillis := 5.0
	dur := time.Duration(durMillis * float64(time.Millisecond))

	Convey("UpdatePresenceMetrics updates presenceMetric", t, func() {
		c, m := tsmon.WithDummyInMemory(context.Background())
		registerCallbacks(c)

		So(tsmon.Flush(c), ShouldBeNil)

		So(len(m.Cells), ShouldEqual, 1)
		So(len(m.Cells[0]), ShouldEqual, 1)
		So(m.Cells[0][0].Name, ShouldEqual, "presence/up")
		So(m.Cells[0][0].Value, ShouldEqual, true)
	})

	Convey("UpdateHTTPMetrics updates client metrics", t, func() {
		c := makeContext()
		name, client := "test_name", "test_client"
		d, err := requestBytesMetric.Get(c, name, client)
		So(d, ShouldBeNil)
		So(err, ShouldBeNil)
		d, err = responseBytesMetric.Get(c, name, client)
		So(d, ShouldBeNil)
		So(err, ShouldBeNil)
		d, err = requestDurationsMetric.Get(c, name, client)
		So(d, ShouldBeNil)
		So(err, ShouldBeNil)
		v, errV := responseStatusMetric.Get(c, 200, name, client)
		So(v, ShouldEqual, 0)
		So(errV, ShouldBeNil)

		UpdateHTTPMetrics(c, name, client, 200, dur, 123, 321)
		d, err = requestBytesMetric.Get(c, name, client)
		So(d.Sum(), ShouldEqual, 123)
		So(err, ShouldBeNil)
		d, err = responseBytesMetric.Get(c, name, client)
		So(d.Sum(), ShouldEqual, 321)
		So(err, ShouldBeNil)
		d, err = requestDurationsMetric.Get(c, name, client)
		So(d.Sum(), ShouldEqual, durMillis)
		So(err, ShouldBeNil)
		v, errV = responseStatusMetric.Get(c, 200, name, client)
		So(v, ShouldEqual, 1)
		So(err, ShouldBeNil)
	})

	Convey("UpdateServerMetrics updates server metrics", t, func() {
		c := makeContext()
		code, name, isRobot := 200, "test_client", false
		d, err := serverDurationsMetric.Get(c, code, name, isRobot)
		So(d, ShouldBeNil)
		So(err, ShouldBeNil)
		d, err = serverRequestBytesMetric.Get(c, code, name, isRobot)
		So(d, ShouldBeNil)
		So(err, ShouldBeNil)
		d, err = serverResponseBytesMetric.Get(c, code, name, isRobot)
		So(d, ShouldBeNil)
		So(err, ShouldBeNil)
		v, errV := serverResponseStatusMetric.Get(c, code, name, isRobot)
		So(v, ShouldEqual, 0)
		So(err, ShouldBeNil)

		Convey("for a robot user agent", func() {

			isRobot = true
			userAgent := "I am a GoogleBot."

			UpdateServerMetrics(c, name, code, dur, 123, 321, userAgent)

			d, err = serverDurationsMetric.Get(c, code, name, isRobot)
			So(d.Sum(), ShouldEqual, durMillis)
			So(err, ShouldBeNil)
			d, err = serverRequestBytesMetric.Get(c, code, name, isRobot)
			So(d.Sum(), ShouldEqual, 123)
			So(err, ShouldBeNil)
			d, err = serverResponseBytesMetric.Get(c, code, name, isRobot)
			So(d.Sum(), ShouldEqual, 321)
			So(err, ShouldBeNil)
			v, errV = serverResponseStatusMetric.Get(c, code, name, isRobot)
			So(v, ShouldEqual, 1)
			So(err, ShouldBeNil)
		})

		Convey("for a non-robot user agent", func() {

			isRobot = false
			userAgent := "I am a human."

			UpdateServerMetrics(c, name, code, dur, 123, 321, userAgent)

			d, err = serverDurationsMetric.Get(c, code, name, isRobot)
			So(d.Sum(), ShouldEqual, durMillis)
			So(err, ShouldBeNil)
			d, err = serverRequestBytesMetric.Get(c, code, name, isRobot)
			So(d.Sum(), ShouldEqual, 123)
			So(err, ShouldBeNil)
			d, err = serverResponseBytesMetric.Get(c, code, name, isRobot)
			So(d.Sum(), ShouldEqual, 321)
			So(err, ShouldBeNil)
			v, errV = serverResponseStatusMetric.Get(c, code, name, isRobot)
			So(v, ShouldEqual, 1)
			So(err, ShouldBeNil)
		})
	})
}
