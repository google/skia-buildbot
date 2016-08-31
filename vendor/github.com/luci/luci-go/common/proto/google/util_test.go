// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package google

import (
	"testing"
	"time"

	"github.com/luci/luci-go/common/clock/testclock"

	. "github.com/smartystreets/goconvey/convey"
)

func TestTimestamp(t *testing.T) {
	t.Parallel()

	Convey(`Can convert to/from time.Time instances.`, t, func() {
		for _, v := range []time.Time{
			{},
			testclock.TestTimeLocal,
			testclock.TestTimeUTC,
		} {
			So(NewTimestamp(v).Time().UTC(), ShouldResemble, v.UTC())
		}
	})

	Convey(`A zero time.Time produces a nil Timestamp.`, t, func() {
		So(NewTimestamp(time.Time{}), ShouldBeNil)
	})

	Convey(`A nil Timestamp produces a zero time.Time.`, t, func() {
		So((*Timestamp)(nil).Time().IsZero(), ShouldBeTrue)
	})
}

func TestDuration(t *testing.T) {
	t.Parallel()

	Convey(`Can convert to/from time.Duration instances.`, t, func() {
		for _, v := range []time.Duration{
			-10 * time.Second,
			0,
			10 * time.Second,
		} {
			So(NewDuration(v).Duration(), ShouldEqual, v)
		}
	})

	Convey(`A zero time.Duration produces a nil Duration.`, t, func() {
		So(NewDuration(0), ShouldBeNil)
	})

	Convey(`A nil Duration produces a zero time.Duration.`, t, func() {
		So((*Duration)(nil).Duration(), ShouldEqual, time.Duration(0))
	})
}
