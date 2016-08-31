// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package model

import (
	"math"
	"testing"
	"time"

	"golang.org/x/net/context"

	. "github.com/smartystreets/goconvey/convey"

	"github.com/luci/luci-go/common/clock/testclock"
	"github.com/luci/luci-go/common/data/bit_field"
	google_pb "github.com/luci/luci-go/common/proto/google"
	. "github.com/luci/luci-go/common/testing/assertions"

	dm "github.com/luci/luci-go/dm/api/service/v1"
)

func TestAttempt(t *testing.T) {
	t.Parallel()

	Convey("Attempt", t, func() {
		c := context.Background()
		c, clk := testclock.UseTime(c, testclock.TestTimeUTC)

		Convey("ModifyState", func() {
			a := MakeAttempt(c, dm.NewAttemptID("quest", 5))
			So(a.State, ShouldEqual, dm.Attempt_SCHEDULING)
			So(a.ModifyState(c, dm.Attempt_FINISHED), ShouldErrLike, "invalid state transition")
			So(a.Modified, ShouldResemble, testclock.TestTimeUTC)

			clk.Add(time.Second)

			So(a.ModifyState(c, dm.Attempt_EXECUTING), ShouldBeNil)
			So(a.State, ShouldEqual, dm.Attempt_EXECUTING)
			So(a.Modified, ShouldResemble, clk.Now())

			So(a.ModifyState(c, dm.Attempt_WAITING), ShouldBeNil)
			So(a.ModifyState(c, dm.Attempt_WAITING), ShouldBeNil)
			So(a.ModifyState(c, dm.Attempt_SCHEDULING), ShouldBeNil)
			So(a.ModifyState(c, dm.Attempt_EXECUTING), ShouldBeNil)
			So(a.ModifyState(c, dm.Attempt_FINISHED), ShouldBeNil)

			So(a.ModifyState(c, dm.Attempt_SCHEDULING), ShouldErrLike, "invalid")
			So(a.State, ShouldEqual, dm.Attempt_FINISHED)
		})

		Convey("ToProto", func() {
			Convey("NeedsExecution", func() {
				a := MakeAttempt(c, dm.NewAttemptID("quest", 10))

				atmpt := dm.NewAttemptScheduling()
				atmpt.Id = dm.NewAttemptID("quest", 10)
				atmpt.Data.Created = google_pb.NewTimestamp(testclock.TestTimeUTC)
				atmpt.Data.Modified = google_pb.NewTimestamp(testclock.TestTimeUTC)

				So(a.ToProto(true), ShouldResemble, atmpt)
			})

			Convey("Executing", func() {
				a := MakeAttempt(c, dm.NewAttemptID("quest", 10))
				clk.Add(10 * time.Second)
				a.CurExecution = 1
				So(a.ModifyState(c, dm.Attempt_EXECUTING), ShouldBeNil)

				So(a.ToProto(true), ShouldResemble, &dm.Attempt{
					Id: &dm.Attempt_ID{Quest: "quest", Id: 10},
					Data: &dm.Attempt_Data{
						Created:       google_pb.NewTimestamp(testclock.TestTimeUTC),
						Modified:      google_pb.NewTimestamp(clk.Now()),
						NumExecutions: 1,
						AttemptType: &dm.Attempt_Data_Executing_{Executing: &dm.Attempt_Data_Executing{
							CurExecutionId: 1}}},
				})
			})

			Convey("Waiting", func() {
				a := MakeAttempt(c, dm.NewAttemptID("quest", 10))
				clk.Add(10 * time.Second)
				a.CurExecution = 1
				So(a.ModifyState(c, dm.Attempt_EXECUTING), ShouldBeNil)
				clk.Add(10 * time.Second)
				So(a.ModifyState(c, dm.Attempt_WAITING), ShouldBeNil)
				a.DepMap = bit_field.Make(4)
				a.DepMap.Set(2)

				atmpt := dm.NewAttemptWaiting(3)
				atmpt.Id = dm.NewAttemptID("quest", 10)
				atmpt.Data.Created = google_pb.NewTimestamp(testclock.TestTimeUTC)
				atmpt.Data.Modified = google_pb.NewTimestamp(clk.Now())
				atmpt.Data.NumExecutions = 1

				So(a.ToProto(true), ShouldResemble, atmpt)
			})

			Convey("Finished", func() {
				a := MakeAttempt(c, dm.NewAttemptID("quest", 10))
				a.State = dm.Attempt_FINISHED
				a.CurExecution = math.MaxUint32
				a.DepMap = bit_field.Make(20)
				a.Result.Data = dm.NewJsonResult("", testclock.TestTimeUTC.Add(10*time.Second))

				a.DepMap.Set(1)
				a.DepMap.Set(5)
				a.DepMap.Set(7)

				So(a.ToProto(true), ShouldResemble, &dm.Attempt{
					Id: &dm.Attempt_ID{Quest: "quest", Id: 10},
					Data: &dm.Attempt_Data{
						Created:       google_pb.NewTimestamp(testclock.TestTimeUTC),
						Modified:      google_pb.NewTimestamp(testclock.TestTimeUTC),
						NumExecutions: math.MaxUint32,
						AttemptType: &dm.Attempt_Data_Finished_{Finished: &dm.Attempt_Data_Finished{
							Data: &dm.JsonResult{
								Expiration: google_pb.NewTimestamp(testclock.TestTimeUTC.Add(10 * time.Second))}}},
					},
				})
			})
		})
	})
}
