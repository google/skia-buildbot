// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package mutate

import (
	"testing"

	"github.com/luci/gae/filter/featureBreaker"
	"github.com/luci/gae/service/datastore"
	. "github.com/luci/luci-go/common/testing/assertions"
	"github.com/luci/luci-go/dm/api/service/v1"
	"github.com/luci/luci-go/dm/appengine/model"
	"github.com/luci/luci-go/tumble"
	. "github.com/smartystreets/goconvey/convey"
)

func TestMergeQuest(t *testing.T) {
	t.Parallel()

	Convey("MergeQuest", t, func() {
		ttest := &tumble.Testing{}
		c := ttest.Context()
		ds := datastore.Get(c)

		desc := dm.NewQuestDesc("distributor", `{"data":"yes"}`, "{}", nil)
		So(desc.Normalize(), ShouldBeNil)
		qst := model.NewQuest(c, desc)
		qst.BuiltBy = append(qst.BuiltBy, *dm.NewTemplateSpec("a", "b", "c", "d"))

		mq := &MergeQuest{qst, nil}

		Convey("root", func() {
			So(mq.Root(c), ShouldResemble, ds.MakeKey("Quest", qst.ID))
		})

		Convey("quest doesn't exist", func() {
			muts, err := mq.RollForward(c)
			So(err, ShouldBeNil)
			So(muts, ShouldBeEmpty)

			q := &model.Quest{ID: qst.ID}
			So(ds.Get(q), ShouldBeNil)
			So(q, ShouldResemble, qst)
		})

		Convey("assuming it exists", func() {
			So(ds.Put(qst), ShouldBeNil)
			Convey("noop merge", func() {
				muts, err := mq.RollForward(c)
				So(err, ShouldBeNil)
				So(muts, ShouldBeEmpty)
			})

			Convey("actual merge", func() {
				ttest.AdvanceTime(c)

				mq.Quest.BuiltBy.Add(*dm.NewTemplateSpec("aa", "bb", "cc", "dd"))
				muts, err := mq.RollForward(c)
				So(err, ShouldBeNil)
				So(muts, ShouldBeEmpty)

				q := &model.Quest{ID: qst.ID}
				So(ds.Get(q), ShouldBeNil)
				So(len(q.BuiltBy), ShouldEqual, 2)
				So(q.Created, ShouldResemble, qst.Created)
			})

			Convey("datastore fail", func() {
				c, fb := featureBreaker.FilterRDS(c, nil)
				fb.BreakFeatures(nil, "GetMulti")
				_, err := mq.RollForward(c)
				So(err, ShouldBeRPCUnknown)
			})
		})

	})
}
