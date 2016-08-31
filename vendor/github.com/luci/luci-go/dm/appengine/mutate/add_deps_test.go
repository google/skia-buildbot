// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package mutate

import (
	"testing"

	"github.com/luci/gae/impl/memory"
	"github.com/luci/gae/service/datastore"
	. "github.com/luci/luci-go/common/testing/assertions"
	dm "github.com/luci/luci-go/dm/api/service/v1"
	"github.com/luci/luci-go/dm/appengine/model"
	. "github.com/smartystreets/goconvey/convey"
	"golang.org/x/net/context"
)

func TestAddDeps(t *testing.T) {
	t.Parallel()

	Convey("AddDeps", t, func() {
		c := memory.Use(context.Background())

		ds := datastore.Get(c)

		aid := dm.NewAttemptID("quest", 1)
		a := model.MakeAttempt(c, aid)
		a.CurExecution = 1
		a.State = dm.Attempt_EXECUTING
		ex := &model.Execution{
			ID: 1, Attempt: ds.KeyForObj(a), Token: []byte("sup"),
			State: dm.Execution_RUNNING}

		ad := &AddDeps{
			Auth: &dm.Execution_Auth{
				Id:    dm.NewExecutionID("quest", 1, 1),
				Token: []byte("sup"),
			},
			Deps: dm.NewAttemptList(map[string][]uint32{
				"to":    {1, 2, 3},
				"top":   {1},
				"tp":    {1},
				"zebra": {17},
			}),
		}
		fds := model.FwdDepsFromList(c, aid, ad.Deps)

		Convey("Root", func() {
			So(ad.Root(c).String(), ShouldEqual, `dev~app::/Attempt,"quest|fffffffe"`)
		})

		Convey("RollForward", func() {

			Convey("Bad", func() {
				Convey("Bad ExecutionKey", func() {
					So(ds.Put(a, ex), ShouldBeNil)

					ad.Auth.Token = []byte("nerp")
					muts, err := ad.RollForward(c)
					So(err, ShouldBeRPCPermissionDenied, "execution Auth")
					So(muts, ShouldBeEmpty)
				})
			})

			Convey("Good", func() {
				So(ds.Put(a, ex), ShouldBeNil)

				Convey("All added already", func() {
					So(ds.Put(fds), ShouldBeNil)

					muts, err := ad.RollForward(c)
					So(err, ShouldBeNil)
					So(muts, ShouldBeEmpty)
				})

				Convey("None added already", func() {
					muts, err := ad.RollForward(c)
					So(err, ShouldBeNil)
					So(len(muts), ShouldEqual, len(fds))

					So(muts[0], ShouldResemble, &AddBackDep{
						Dep: fds[0].Edge(), NeedsAck: true})

					So(ds.Get(a, fds), ShouldBeNil)
					So(a.DepMap.Size(), ShouldEqual, len(fds))
					So(a.State, ShouldEqual, dm.Attempt_EXECUTING)
					So(fds[0].ForExecution, ShouldEqual, 1)

					muts, err = (&FinishExecution{
						ad.Auth.Id, &dm.Result{Data: dm.NewJsonResult(`{"hi": true}`)},
					}).RollForward(c)
					So(err, ShouldBeNil)
					So(muts, ShouldBeNil)

					So(ds.Get(a), ShouldBeNil)
					So(a.State, ShouldEqual, dm.Attempt_WAITING)
				})

				Convey("adding new Attempts at the same time", func() {
					ad.Attempts = dm.NewAttemptList(map[string][]uint32{
						"to": {2, 3},
						"tp": {1},
					})

					muts, err := ad.RollForward(c)
					So(err, ShouldBeNil)
					So(len(muts), ShouldEqual, len(fds)+3)

					So(muts[0], ShouldResemble, &EnsureAttempt{dm.NewAttemptID("to", 3)})
					So(muts[1], ShouldResemble, &AddBackDep{
						Dep: fds[0].Edge(), NeedsAck: true})

					So(ds.Get(a, fds), ShouldBeNil)
					So(a.DepMap.Size(), ShouldEqual, len(fds))
					So(a.State, ShouldEqual, dm.Attempt_EXECUTING)
					So(fds[0].ForExecution, ShouldEqual, 1)

					muts, err = (&FinishExecution{
						ad.Auth.Id, &dm.Result{Data: dm.NewJsonResult(`{"hi":true}`)},
					}).RollForward(c)
					So(err, ShouldBeNil)
					So(muts, ShouldBeNil)

					So(ds.Get(a), ShouldBeNil)
					So(a.State, ShouldEqual, dm.Attempt_WAITING)
				})
			})
		})
	})
}
