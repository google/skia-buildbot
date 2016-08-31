// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package mutate

import (
	"fmt"
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"

	"github.com/luci/gae/service/datastore"
	"github.com/luci/luci-go/common/errors"
	"github.com/luci/luci-go/common/proto/google"
	. "github.com/luci/luci-go/common/testing/assertions"
	"github.com/luci/luci-go/dm/api/service/v1"
	"github.com/luci/luci-go/dm/appengine/distributor/fake"
	"github.com/luci/luci-go/dm/appengine/model"
)

func TestScheduleExecution(t *testing.T) {
	t.Parallel()

	Convey("ScheduleExecution", t, func() {
		_, c, dist := fake.Setup(FinishExecutionFn)

		qdesc := fake.QuestDesc("quest")
		qdesc.Meta.Timeouts.Start = google.NewDuration(time.Minute * 5)
		qid := qdesc.QuestID()
		se := &ScheduleExecution{dm.NewAttemptID(qid, 1)}

		Convey("Root", func() {
			So(se.Root(c).String(), ShouldEqual, fmt.Sprintf(`dev~app::/Attempt,"%s|fffffffe"`, qid))
		})

		Convey("RollForward", func() {
			ds := datastore.Get(c)
			q := &model.Quest{ID: qid, Desc: *qdesc}
			a := &model.Attempt{
				ID:    *se.For,
				State: dm.Attempt_SCHEDULING,
			}
			e := model.ExecutionFromID(c, dm.NewExecutionID(qid, 1, 1))
			So(ds.Put(q, a), ShouldBeNil)

			Convey("basic", func() {
				muts, err := se.RollForward(c)
				So(err, ShouldBeNil)
				So(muts, ShouldBeNil)

				So(ds.Get(a, e), ShouldBeNil)
				Convey("distributor information is saved", func() {
					tok := fake.MkToken(dm.NewExecutionID(qid, 1, 1))

					So(a.State, ShouldEqual, dm.Attempt_EXECUTING)
					So(e.State, ShouldEqual, dm.Execution_SCHEDULING)
					So(e.DistributorConfigName, ShouldEqual, "fakeDistributor")
					So(e.DistributorToken, ShouldEqual, tok)
					So(e.DistributorConfigVersion, ShouldEqual, "testing")
					So(e.TimeToStart, ShouldEqual, time.Minute*5)
				})
				Convey("a timeout is set", func() {
					ex, err := ds.Exists(ds.MakeKey(
						"Attempt", a.ID.DMEncoded(),
						"Execution", 1,
						"tumble.Mutation", "n:timeout"))
					So(err, ShouldBeNil)
					So(ex.All(), ShouldBeTrue)
				})
			})

			Convey("transient", func() {
				dist.RunError = errors.WrapTransient(errors.New("transient failure"))

				muts, err := se.RollForward(c)
				So(err, ShouldErrLike, "transient")
				So(muts, ShouldBeNil)
			})

			Convey("rejection", func() {
				dist.RunError = errors.New("no soup for you")

				muts, err := se.RollForward(c)
				So(err, ShouldBeNil)
				So(muts, ShouldBeNil)

				So(ds.Get(a, e), ShouldBeNil)
				So(a.State, ShouldEqual, dm.Attempt_ABNORMAL_FINISHED)
				So(a.Result.AbnormalFinish.Status, ShouldEqual, dm.AbnormalFinish_REJECTED)
				So(a.Result.AbnormalFinish.Reason, ShouldContainSubstring, "non-transient")

				So(e.State, ShouldEqual, dm.Execution_ABNORMAL_FINISHED)
				So(e.Result.AbnormalFinish.Status, ShouldEqual, dm.AbnormalFinish_REJECTED)
				So(e.Result.AbnormalFinish.Reason, ShouldContainSubstring, "non-transient")
			})

		})
	})
}
