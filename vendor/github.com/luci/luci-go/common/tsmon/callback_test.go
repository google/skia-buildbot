// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package tsmon

import (
	"testing"
	"time"

	"golang.org/x/net/context"

	"github.com/luci/luci-go/common/tsmon/field"
	"github.com/luci/luci-go/common/tsmon/types"

	. "github.com/smartystreets/goconvey/convey"
)

func TestCallbacks(t *testing.T) {
	c := context.Background()

	Convey("Register global callback without metrics panics", t, func() {
		So(func() {
			RegisterGlobalCallbackIn(c, func(context.Context) {})
		}, ShouldPanic)
	})

	Convey("Callback is run on Flush", t, func() {
		c, s, m := WithFakes(c)

		RegisterCallbackIn(c, func(c context.Context) {
			s.Cells = append(s.Cells, types.Cell{
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
}
