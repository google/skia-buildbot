// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package deps

import (
	"testing"

	"github.com/luci/gae/service/datastore"
	. "github.com/luci/luci-go/common/testing/assertions"
	dm "github.com/luci/luci-go/dm/api/service/v1"
	"github.com/luci/luci-go/dm/appengine/distributor/fake"
	"github.com/luci/luci-go/dm/appengine/model"
	. "github.com/smartystreets/goconvey/convey"
)

func TestAddDeps(t *testing.T) {
	t.Parallel()

	Convey("EnsureGraphData (Adding deps)", t, func() {
		ttest, c, dist, s := testSetup()
		c = writer(c)
		ds := datastore.Get(c)

		qid := s.ensureQuest(c, "quest", 1)

		Convey("Bad", func() {
			req := &dm.EnsureGraphDataReq{
				RawAttempts: dm.NewAttemptList(map[string][]uint32{
					"fakeQuestId": {1},
				}),
			}

			Convey("No such originating attempt", func() {
				dist.RunTask(c, dm.NewExecutionID(qid, 1, 1), func(tsk *fake.Task) error {
					aTsk := tsk.MustActivate(c, s)

					_, err := aTsk.EnsureGraphData(req)
					So(err, ShouldBeRPCInvalidArgument, `cannot create attempts for absent quest "-asJkTOx8ORdkGZsg7Bc-w2Z_0FIB4vgD1afzInkwNE"`)
					return nil
				})
			})
		})

		toQuest := s.ensureQuest(c, "to", 1)
		toQuestDesc := fake.QuestDesc("to")
		req := &dm.EnsureGraphDataReq{
			RawAttempts: dm.NewAttemptList(map[string][]uint32{
				toQuest: {1},
			}),
		}
		fwd := model.FwdDepsFromList(c,
			dm.NewAttemptID(qid, 1),
			dm.NewAttemptList(map[string][]uint32{toQuest: {1}}),
		)[0]
		ttest.Drain(c)

		Convey("Good", func() {
			Convey("deps already exist", func() {
				err := dist.RunTask(c, dm.NewExecutionID(qid, 1, 1), func(tsk *fake.Task) error {
					aTsk := tsk.MustActivate(c, s)

					rsp, err := aTsk.EnsureGraphData(req)
					So(err, ShouldBeNil)
					rsp.Result.PurgeTimestamps()
					So(rsp, ShouldResemble, &dm.EnsureGraphDataRsp{
						Accepted:   true,
						ShouldHalt: true,
					})
					return nil
				})
				So(err, ShouldBeNil)
			})

			Convey("deps already done", func() {
				err := dist.RunTask(c, dm.NewExecutionID(toQuest, 1, 1), func(tsk *fake.Task) error {
					tsk.MustActivate(c, s).Finish(`{"done":true}`)
					return nil
				})
				So(err, ShouldBeNil)
				ttest.Drain(c)

				err = dist.RunTask(c, dm.NewExecutionID(qid, 1, 1), func(tsk *fake.Task) error {
					aTsk := tsk.MustActivate(c, s)

					req.Normalize() // to ensure the next assignment works
					req.Include.Attempt.Result = true

					rsp, err := aTsk.EnsureGraphData(req)
					So(err, ShouldBeNil)
					rsp.Result.PurgeTimestamps()
					exAttempt := dm.NewAttemptFinished(dm.NewJsonResult(`{"done":true}`))
					exAttempt.Data.NumExecutions = 1
					So(rsp, ShouldResemble, &dm.EnsureGraphDataRsp{
						Accepted: true,
						Result: &dm.GraphData{Quests: map[string]*dm.Quest{
							toQuest: {
								Data: &dm.Quest_Data{
									Desc:    toQuestDesc,
									BuiltBy: []*dm.Quest_TemplateSpec{},
								},
								Attempts: map[uint32]*dm.Attempt{1: exAttempt},
							},
						}},
					})
					return nil
				})
				So(err, ShouldBeNil)

				ttest.Drain(c)

				So(ds.Get(fwd), ShouldBeNil)
			})

			Convey("adding new deps", func() {
				err := dist.RunTask(c, dm.NewExecutionID(qid, 1, 1), func(tsk *fake.Task) error {
					aTsk := tsk.MustActivate(c, s)

					rsp, err := aTsk.EnsureGraphData(req)
					So(err, ShouldBeNil)
					So(rsp, ShouldResemble, &dm.EnsureGraphDataRsp{
						Accepted:   true,
						ShouldHalt: true,
					})

					So(ds.Get(fwd), ShouldBeNil)
					a := model.AttemptFromID(dm.NewAttemptID(qid, 1))
					So(ds.Get(a), ShouldBeNil)
					So(a.State, ShouldEqual, dm.Attempt_EXECUTING)
					e := model.ExecutionFromID(c, dm.NewExecutionID(qid, 1, 1))
					So(ds.Get(e), ShouldBeNil)
					So(e.State, ShouldEqual, dm.Execution_STOPPING)
					return nil
				})
				So(err, ShouldBeNil)
			})

		})
	})
}
