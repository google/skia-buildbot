// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package dm

import (
	"testing"

	. "github.com/luci/luci-go/common/testing/assertions"
	. "github.com/smartystreets/goconvey/convey"
)

func TestAttemptState(t *testing.T) {
	t.Parallel()

	Convey("Evolve", t, func() {
		Convey("Identity", func() {
			s := Attempt_SCHEDULING
			So(s.Evolve(Attempt_SCHEDULING), ShouldBeNil)
			So(s, ShouldEqual, Attempt_SCHEDULING)
		})

		Convey("Transition", func() {
			s := Attempt_EXECUTING
			So(s.Evolve(Attempt_WAITING), ShouldBeNil)
			So(s, ShouldEqual, Attempt_WAITING)
		})

		Convey("Invalid starting transistion", func() {
			s := Attempt_SCHEDULING
			So(s.Evolve(Attempt_FINISHED), ShouldErrLike, "invalid state transition SCHEDULING -> FINISHED")
			So(s, ShouldEqual, Attempt_SCHEDULING)
		})

		Convey("Invalid ending transistion", func() {
			s := Attempt_WAITING
			So(s.Evolve(Attempt_FINISHED), ShouldErrLike, "invalid state transition WAITING -> FINISHED")
			So(s, ShouldEqual, Attempt_WAITING)
		})

		Convey("MustEvolve", func() {
			s := Attempt_FINISHED
			So(func() { s.MustEvolve(Attempt_SCHEDULING) }, ShouldPanic)
		})
	})
}
