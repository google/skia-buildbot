// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package deps

import (
	"testing"

	"github.com/luci/gae/service/datastore"
	"github.com/luci/luci-go/common/clock/testclock"
	. "github.com/luci/luci-go/common/testing/assertions"
	"github.com/luci/luci-go/dm/api/service/v1"
	"github.com/luci/luci-go/dm/appengine/model"
	. "github.com/smartystreets/goconvey/convey"
)

func TestFinishAttempt(t *testing.T) {
	t.Parallel()

	Convey("FinishAttempt", t, func() {
		_, c, _, s := testSetup()
		ds := datastore.Get(c)

		So(ds.Put(&model.Quest{ID: "quest"}), ShouldBeNil)
		a := &model.Attempt{
			ID:           *dm.NewAttemptID("quest", 1),
			State:        dm.Attempt_EXECUTING,
			CurExecution: 1,
		}
		e := model.ExecutionFromID(c, dm.NewExecutionID("quest", 1, 1))
		ar := &model.AttemptResult{Attempt: ds.KeyForObj(a)}
		So(ds.Put(a), ShouldBeNil)
		So(ds.Put(&model.Execution{
			ID: 1, Attempt: ds.KeyForObj(a), Token: []byte("exKey"),
			State: dm.Execution_RUNNING}), ShouldBeNil)

		req := &dm.FinishAttemptReq{
			Auth: &dm.Execution_Auth{
				Id:    dm.NewExecutionID(a.ID.Quest, a.ID.Id, 1),
				Token: []byte("exKey"),
			},
			Data: dm.NewJsonResult(`{"something": "valid"}`, testclock.TestTimeUTC),
		}

		Convey("bad", func() {
			Convey("bad Token", func() {
				req.Auth.Token = []byte("fake")
				_, err := s.FinishAttempt(c, req)
				So(err, ShouldBeRPCPermissionDenied, "execution Auth")
			})

			Convey("not real json", func() {
				req.Data = dm.NewJsonResult(`i am not valid json`)
				_, err := s.FinishAttempt(c, req)
				So(err, ShouldErrLike, "invalid character 'i'")
			})
		})

		Convey("good", func() {
			_, err := s.FinishAttempt(c, req)
			So(err, ShouldBeNil)

			So(ds.Get(a, ar, e), ShouldBeNil)
			So(a.State, ShouldEqual, dm.Attempt_EXECUTING)
			So(e.State, ShouldEqual, dm.Execution_STOPPING)

			So(a.Result.Data.Size, ShouldEqual, 21)
			So(ar.Data.Size, ShouldEqual, 21)
		})

	})
}
