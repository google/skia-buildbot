// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package mutate

import (
	"testing"

	"github.com/luci/gae/impl/memory"
	"github.com/luci/gae/service/datastore"
	"github.com/luci/luci-go/common/data/bit_field"
	. "github.com/luci/luci-go/common/testing/assertions"
	"github.com/luci/luci-go/dm/api/service/v1"
	"github.com/luci/luci-go/dm/appengine/model"
	"github.com/luci/luci-go/tumble"
	. "github.com/smartystreets/goconvey/convey"
	"golang.org/x/net/context"
)

func TestAckFwdDep(t *testing.T) {
	t.Parallel()

	Convey("AckFwdDep", t, func() {
		c := memory.Use(context.Background())
		ds := datastore.Get(c)

		afd := &AckFwdDep{
			Dep: &model.FwdEdge{
				From: dm.NewAttemptID("quest", 1),
				To:   dm.NewAttemptID("to", 1),
			},
		}

		Convey("Root", func() {
			So(afd.Root(c), ShouldResemble, ds.MakeKey("Attempt", "quest|fffffffe"))
		})

		Convey("RollForward", func() {
			a, fwd := afd.Dep.Fwd(c)

			Convey("AddingDeps", func() {
				Convey("good", func() {
					a.State = dm.Attempt_WAITING
					a.DepMap = bit_field.Make(2)
					So(ds.Put(a, fwd), ShouldBeNil)

					Convey("not-last", func() {
						muts, err := afd.RollForward(c)
						So(err, ShouldBeNil)
						So(muts, ShouldBeNil)

						So(ds.Get(a, fwd), ShouldBeNil)
						So(a.State, ShouldEqual, dm.Attempt_WAITING)
						So(a.DepMap.CountSet(), ShouldEqual, 1)
					})

					Convey("last-finished", func() {
						a.DepMap.Set(1)
						So(ds.Put(a), ShouldBeNil)

						muts, err := afd.RollForward(c)
						So(err, ShouldBeNil)
						So(muts, ShouldResemble, []tumble.Mutation{
							&ScheduleExecution{&a.ID}})

						So(ds.Get(a, fwd), ShouldBeNil)
						So(a.State, ShouldEqual, dm.Attempt_SCHEDULING)
						So(a.DepMap.CountSet(), ShouldEqual, 0) // was reset
					})
				})

				Convey("bad", func() {
					a.State = dm.Attempt_WAITING
					a.DepMap = bit_field.Make(2)
					a.CurExecution = 1
					So(ds.Put(a, fwd), ShouldBeNil)

					Convey("CurExecution mismatch -> NOP", func() {
						muts, err := afd.RollForward(c)
						So(err, ShouldBeNil)
						So(muts, ShouldBeNil)

						So(ds.Get(a, fwd), ShouldBeNil)
						So(a.State, ShouldEqual, dm.Attempt_WAITING)
						So(a.DepMap.CountSet(), ShouldEqual, 0)
					})

					Convey("Missing data", func() {
						So(ds.Delete(ds.KeyForObj(a)), ShouldBeNil)

						_, err := afd.RollForward(c)
						So(err, ShouldErrLike, datastore.ErrNoSuchEntity)
					})
				})

			})
		})
	})
}
