// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package mathrand

import (
	"math/rand"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"golang.org/x/net/context"
)

func Test(t *testing.T) {
	t.Parallel()

	Convey("test mathrand", t, func() {
		c := context.Background()

		Convey("unset", func() {
			// Just ensure doesn't crash.
			So(Get(c).Int()+1 > 0, ShouldBeTrue)
		})

		Convey("set persistance", func() {
			c = Set(c, rand.New(rand.NewSource(12345)))
			r := rand.New(rand.NewSource(12345))
			So(Get(c).Int(), ShouldEqual, r.Int())
			So(Get(c).Int(), ShouldEqual, r.Int())
		})

		Convey("nil set", func() {
			c = Set(c, nil)
			// Just ensure doesn't crash.
			So(Get(c).Int()+1 > 0, ShouldBeTrue)
		})
	})
}
