// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package deps

import (
	"testing"

	"github.com/luci/gae/service/datastore"
	. "github.com/luci/luci-go/common/testing/assertions"
	dm "github.com/luci/luci-go/dm/api/service/v1"
	"github.com/luci/luci-go/dm/appengine/model"
	. "github.com/smartystreets/goconvey/convey"
)

func TestEnsureAttempt(t *testing.T) {
	t.Parallel()

	Convey("EnsureGraphData (Ensuring attempts)", t, func() {
		ttest, c, _, s := testSetup()
		ds := datastore.Get(c)

		Convey("bad", func() {
			Convey("no quest", func() {
				_, err := s.EnsureGraphData(writer(c), &dm.EnsureGraphDataReq{
					RawAttempts: dm.NewAttemptList(map[string][]uint32{"quest": {1}}),
				})
				So(err, ShouldBeRPCInvalidArgument,
					`cannot create attempts for absent quest "quest"`)
			})
			Convey("mismatched quest", func() {
				_, err := s.EnsureGraphData(writer(c), &dm.EnsureGraphDataReq{
					Quest: []*dm.Quest_Desc{dm.NewQuestDesc("fakeDistributor", "{}", "{}", nil)},
				})
				So(err, ShouldErrLike, "mismatched quest_attempt v. quest lengths")
			})
			Convey("no auth", func() {
				desc := dm.NewQuestDesc("fakeDistributor", `{"hi": "there"}`, "{}", nil)
				_, err := s.EnsureGraphData(c, &dm.EnsureGraphDataReq{
					Quest:        []*dm.Quest_Desc{desc},
					QuestAttempt: []*dm.AttemptList_Nums{{Nums: []uint32{1}}},
				})
				So(err, ShouldBeRPCPermissionDenied, `not authorized`)
			})
		})

		Convey("good", func() {
			desc := dm.NewQuestDesc("fakeDistributor", `{"hi": "there"}`, "{}", nil)
			So(desc.Normalize(), ShouldBeNil)
			q := model.NewQuest(c, desc)
			rsp, err := s.EnsureGraphData(writer(c), &dm.EnsureGraphDataReq{
				Quest:        []*dm.Quest_Desc{desc},
				QuestAttempt: []*dm.AttemptList_Nums{{Nums: []uint32{1}}},
			})
			So(err, ShouldBeNil)
			So(rsp.Accepted, ShouldBeTrue)
			ttest.Drain(c)
			So(ds.Get(&model.Attempt{ID: *dm.NewAttemptID(q.ID, 1)}), ShouldBeNil)
		})

	})
}
