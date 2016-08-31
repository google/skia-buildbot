// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package mutate

import (
	"testing"

	"github.com/luci/gae/filter/featureBreaker"
	"github.com/luci/gae/impl/memory"
	"github.com/luci/gae/service/datastore"
	. "github.com/luci/luci-go/common/testing/assertions"
	"github.com/luci/luci-go/dm/api/service/v1"
	"github.com/luci/luci-go/dm/appengine/model"
	"github.com/luci/luci-go/tumble"
	. "github.com/smartystreets/goconvey/convey"
	"golang.org/x/net/context"
)

func TestAddBackDep(t *testing.T) {
	t.Parallel()

	Convey("AddBackDep", t, func() {
		c := memory.Use(context.Background())
		ds := datastore.Get(c)

		abd := &AddBackDep{
			Dep: &model.FwdEdge{
				From: dm.NewAttemptID("quest", 1),
				To:   dm.NewAttemptID("to", 1),
			},
		}

		Convey("Root", func() {
			So(abd.Root(c).String(), ShouldEqual, `dev~app::/BackDepGroup,"to|fffffffe"`)
		})

		Convey("RollForward", func() {
			bdg, bd := abd.Dep.Back(c)
			So(bd.Propagated, ShouldBeFalse)

			Convey("attempt finished", func() {
				bdg.AttemptFinished = true
				So(ds.Put(bdg), ShouldBeNil)

				Convey("no need completion", func() {
					muts, err := abd.RollForward(c)
					So(err, ShouldBeNil)
					So(muts, ShouldBeNil)

					So(ds.Get(bdg, bd), ShouldBeNil)
					So(bd.Edge(), ShouldResemble, abd.Dep)
					So(bd.Propagated, ShouldBeTrue)
				})

				Convey("need completion", func() {
					abd.NeedsAck = true
					muts, err := abd.RollForward(c)
					So(err, ShouldBeNil)
					So(muts, ShouldResemble, []tumble.Mutation{&AckFwdDep{abd.Dep}})

					So(ds.Get(bdg, bd), ShouldBeNil)
					So(bd.Edge(), ShouldResemble, abd.Dep)
					So(bd.Propagated, ShouldBeTrue)
				})
			})

			Convey("attempt not finished, need completion", func() {
				ex, err := ds.Exists(ds.KeyForObj(bdg))
				So(err, ShouldBeNil)
				So(ex.Any(), ShouldBeFalse)

				abd.NeedsAck = true
				muts, err := abd.RollForward(c)
				So(err, ShouldBeNil)
				So(muts, ShouldBeNil)

				// Note that bdg was created as a side effect.
				So(ds.Get(bdg, bd), ShouldBeNil)
				So(bd.Edge(), ShouldResemble, abd.Dep)
				So(bd.Propagated, ShouldBeFalse)
				So(bdg.AttemptFinished, ShouldBeFalse)
			})

			Convey("failure", func() {
				c, fb := featureBreaker.FilterRDS(c, nil)
				fb.BreakFeatures(nil, "PutMulti")

				_, err := abd.RollForward(c)
				So(err, ShouldErrLike, `feature "PutMulti" is broken`)
			})
		})
	})
}
