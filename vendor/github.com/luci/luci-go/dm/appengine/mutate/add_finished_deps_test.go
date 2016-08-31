// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package mutate

import (
	"testing"

	"github.com/luci/gae/impl/memory"
	"github.com/luci/gae/service/datastore"
	"github.com/luci/luci-go/common/errors"
	. "github.com/luci/luci-go/common/testing/assertions"
	"github.com/luci/luci-go/dm/api/service/v1"
	"github.com/luci/luci-go/dm/appengine/model"
	"github.com/luci/luci-go/tumble"
	. "github.com/smartystreets/goconvey/convey"
	"golang.org/x/net/context"
)

func TestAddFinishedDeps(t *testing.T) {
	t.Parallel()

	Convey("AddFinishedDeps", t, func() {
		c := memory.Use(context.Background())
		f := &AddFinishedDeps{
			&dm.Execution_Auth{
				Id:    dm.NewExecutionID("quest", 1, 7),
				Token: []byte("sup"),
			},
			[]*model.Quest{
				{
					ID: "to",
					BuiltBy: model.TemplateInfo{
						*dm.NewTemplateSpec("a", "b", "c", "d"),
					},
				},
			},
			dm.NewAttemptList(map[string][]uint32{
				"to": {1, 2, 3},
			}),
		}

		base := f.Auth.Id.AttemptID()
		fs := model.FwdDepsFromList(c, base, f.FinishedAttempts)

		ds := datastore.Get(c)
		fs[1].ForExecution = 1
		So(ds.Put(fs[1]), ShouldBeNil)

		a := &model.Attempt{ID: *base, State: dm.Attempt_EXECUTING, CurExecution: 7}
		ak := ds.KeyForObj(a)
		e := &model.Execution{
			ID: 7, Attempt: ak, State: dm.Execution_RUNNING, Token: []byte("sup")}
		So(ds.Put(a, e), ShouldBeNil)

		Convey("Root", func() {
			So(f.Root(c).String(), ShouldEqual, `dev~app::/Attempt,"quest|fffffffe"`)
		})

		Convey("RollForward", func() {
			err := ds.Get(fs)
			So(err, ShouldResemble, errors.MultiError{
				datastore.ErrNoSuchEntity,
				nil,
				datastore.ErrNoSuchEntity,
			})

			muts, err := f.RollForward(c)
			So(err, ShouldBeNil)
			So(muts, ShouldResemble, []tumble.Mutation{
				&AddBackDep{Dep: fs[0].Edge()},
				&AddBackDep{Dep: fs[2].Edge()},
				&MergeQuest{f.MergeQuests[0], nil},
			})

			So(ds.Get(fs), ShouldBeNil)
			So(fs[0].ForExecution, ShouldEqual, 7)
			So(fs[1].ForExecution, ShouldEqual, 1)
			So(fs[2].ForExecution, ShouldEqual, 7)

			muts, err = f.RollForward(c)
			So(err, ShouldBeNil)
			So(muts, ShouldBeEmpty)
		})

		Convey("RollForward (bad)", func() {
			So(ds.Delete(ak), ShouldBeNil)
			_, err := f.RollForward(c)
			So(err, ShouldBeRPCInternal, "execution Auth")
		})
	})
}
