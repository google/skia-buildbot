// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package mutate

import (
	"testing"

	"github.com/luci/gae/impl/memory"
	"github.com/luci/gae/service/datastore"
	"github.com/luci/luci-go/dm/api/service/v1"
	"github.com/luci/luci-go/dm/appengine/model"
	"github.com/luci/luci-go/tumble"
	. "github.com/smartystreets/goconvey/convey"
	"golang.org/x/net/context"
)

func TestRecordCompletion(t *testing.T) {
	t.Parallel()

	Convey("RecordCompletion", t, func() {
		c := memory.Use(context.Background())
		rc := &RecordCompletion{dm.NewAttemptID("quest", 1)}

		bdg := &model.BackDepGroup{Dependee: *rc.For}

		ds := datastore.Get(c)

		Convey("Root", func() {
			So(rc.Root(c).String(), ShouldEqual, `dev~app::/BackDepGroup,"quest|fffffffe"`)
		})

		Convey("RollForward", func() {

			Convey("Good", func() {
				Convey("No BDG", func() {
					muts, err := rc.RollForward(c)
					So(err, ShouldBeNil)
					So(muts, ShouldBeEmpty)

					So(ds.Get(bdg), ShouldBeNil)
					So(bdg.AttemptFinished, ShouldBeTrue)
				})

				Convey("BDG exists, no deps", func() {
					So(ds.Put(bdg), ShouldBeNil)

					muts, err := rc.RollForward(c)
					So(err, ShouldBeNil)
					So(muts, ShouldBeEmpty)

					So(ds.Get(bdg), ShouldBeNil)
					So(bdg.AttemptFinished, ShouldBeTrue)
				})

				Convey("BDG exists, with unfinished deps", func() {
					bd := &model.BackDep{
						Depender:      *dm.NewAttemptID("from", 1),
						DependeeGroup: rc.Root(c),
					}
					So(ds.Put(bdg, bd), ShouldBeNil)

					muts, err := rc.RollForward(c)
					So(err, ShouldBeNil)
					So(muts, ShouldResemble, []tumble.Mutation{&AckFwdDep{bd.Edge()}})

					So(ds.Get(bdg, bd), ShouldBeNil)
					So(bdg.AttemptFinished, ShouldBeTrue)
					So(bd.Propagated, ShouldBeTrue)
				})

				Convey("BDG exists, with finished deps", func() {
					bd := &model.BackDep{
						Depender:      *dm.NewAttemptID("from", 1),
						DependeeGroup: rc.Root(c),
						Propagated:    true,
					}
					So(ds.Put(bdg, bd), ShouldBeNil)

					muts, err := rc.RollForward(c)
					So(err, ShouldBeNil)
					So(muts, ShouldBeEmpty)

					So(ds.Get(bdg, bd), ShouldBeNil)
					So(bdg.AttemptFinished, ShouldBeTrue)
					So(bd.Propagated, ShouldBeTrue)
				})

				Convey("BDG exists, but too much to do in one TXN!", func() {
					// amtWork is 1.5*completionLimit
					amtWork := completionLimit + (completionLimit >> 1)

					for i := 0; i < amtWork; i++ {
						bd := &model.BackDep{
							Depender:      *dm.NewAttemptID("from", 0),
							DependeeGroup: rc.Root(c),
						}
						bd.Depender.Id = uint32(i + 1)
						So(ds.Put(bd), ShouldBeNil)
					}
					So(ds.Put(bdg), ShouldBeNil)

					muts, err := rc.RollForward(c)
					So(err, ShouldBeNil)
					So(len(muts), ShouldEqual, completionLimit+1)

					So(muts[completionLimit], ShouldResemble, rc)

					muts, err = rc.RollForward(c)
					So(err, ShouldBeNil)
					So(len(muts), ShouldEqual, amtWork-completionLimit)

					muts, err = rc.RollForward(c)
					So(err, ShouldBeNil)
					So(muts, ShouldBeEmpty)
				})
			})
		})
	})
}
