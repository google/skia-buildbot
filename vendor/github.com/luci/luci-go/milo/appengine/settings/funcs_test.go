// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package settings

import (
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"
)

func TestFuncs(t *testing.T) {
	t.Parallel()

	Convey("humanDuration", t, func() {
		Convey("3 hrs", func() {
			h := humanDuration(3 * time.Hour)
			So(h, ShouldEqual, "3 hrs")
		})

		Convey("2 hrs 59 mins", func() {
			h := humanDuration(2*time.Hour + 59*time.Minute)
			So(h, ShouldEqual, "2 hrs 59 mins")
		})
	})
}
