// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package schedule

import (
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"
)

var epoch = time.Unix(1442270520, 0).UTC()

func TestAbsoluteSchedule(t *testing.T) {
	Convey("Parsing success", t, func() {
		sched, err := Parse("* * * * * *", 0)
		So(err, ShouldBeNil)
		So(sched.String(), ShouldEqual, "* * * * * *")
	})

	Convey("Parsing error", t, func() {
		sched, err := Parse("not a schedule", 0)
		So(err, ShouldNotBeNil)
		So(sched, ShouldBeNil)
	})

	Convey("Next works", t, func() {
		sched, _ := Parse("*/15 * * * * * *", 0)
		So(sched.IsAbsolute(), ShouldBeTrue)
		So(sched.Next(epoch, time.Time{}), ShouldResemble, epoch.Add(15*time.Second))
		So(sched.Next(epoch.Add(15*time.Second), epoch), ShouldResemble, epoch.Add(30*time.Second))
	})
}

func TestRelativeSchedule(t *testing.T) {
	Convey("Parsing success", t, func() {
		sched, err := Parse("with 15s interval", 0)
		So(err, ShouldBeNil)
		So(sched.String(), ShouldEqual, "with 15s interval")
	})

	Convey("Parsing error", t, func() {
		sched, err := Parse("with bladasdafier", 0)
		So(err, ShouldNotBeNil)
		So(sched, ShouldBeNil)

		sched, err = Parse("with -1s interval", 0)
		So(err, ShouldNotBeNil)
		So(sched, ShouldBeNil)

		sched, err = Parse("with NaNs interval", 0)
		So(err, ShouldNotBeNil)
		So(sched, ShouldBeNil)
	})

	Convey("Next works", t, func() {
		sched, _ := Parse("with 15s interval", 0)
		So(sched.IsAbsolute(), ShouldBeFalse)

		// First tick is pseudorandom.
		So(sched.Next(epoch, time.Time{}), ShouldResemble, epoch.Add(14*time.Second+177942239*time.Nanosecond))

		// Next tick is 15s from prev one, or now if it's too late.
		So(sched.Next(epoch.Add(16*time.Second), epoch.Add(15*time.Second)), ShouldResemble, epoch.Add(30*time.Second))
		So(sched.Next(epoch.Add(31*time.Second), epoch.Add(15*time.Second)), ShouldResemble, epoch.Add(31*time.Second))
	})
}
