// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package mutate

import (
	"testing"

	"github.com/luci/gae/service/datastore"
	"github.com/luci/luci-go/dm/api/service/v1"
	"github.com/luci/luci-go/dm/appengine/model"
	"github.com/luci/luci-go/tumble"
	//. "github.com/luci/luci-go/common/testing/assertions"
	. "github.com/smartystreets/goconvey/convey"
)

func TestEnsureQuestAttempts(t *testing.T) {
	t.Parallel()

	Convey("EnsureQuestAttempts", t, func() {
		ttest := &tumble.Testing{}
		c := ttest.Context()
		ds := datastore.Get(c)

		desc := dm.NewQuestDesc("distributor", `{"data":"yes"}`, "{}", nil)
		So(desc.Normalize(), ShouldBeNil)
		qst := model.NewQuest(c, desc)

		eqa := EnsureQuestAttempts{qst, []uint32{1, 2, 3, 4}, false}

		Convey("root", func() {
			So(eqa.Root(c), ShouldResemble, ds.MakeKey("Quest", qst.ID))
		})

		Convey("quest dne", func() {
			muts, err := eqa.RollForward(c)
			So(err, ShouldBeNil)
			So(muts, ShouldResemble, []tumble.Mutation{
				&EnsureAttempt{dm.NewAttemptID(qst.ID, 1)},
				&EnsureAttempt{dm.NewAttemptID(qst.ID, 2)},
				&EnsureAttempt{dm.NewAttemptID(qst.ID, 3)},
				&EnsureAttempt{dm.NewAttemptID(qst.ID, 4)},
			})
		})

		Convey("tail recursion", func() {
			eqa.AIDs = append(eqa.AIDs, []uint32{5, 6, 7, 8, 9, 10, 11, 12}...)
			muts, err := eqa.RollForward(c)
			So(err, ShouldBeNil)
			So(muts, ShouldResemble, []tumble.Mutation{
				&EnsureAttempt{dm.NewAttemptID(qst.ID, 1)},
				&EnsureAttempt{dm.NewAttemptID(qst.ID, 2)},
				&EnsureAttempt{dm.NewAttemptID(qst.ID, 3)},
				&EnsureAttempt{dm.NewAttemptID(qst.ID, 4)},
				&EnsureAttempt{dm.NewAttemptID(qst.ID, 5)},
				&EnsureAttempt{dm.NewAttemptID(qst.ID, 6)},
				&EnsureAttempt{dm.NewAttemptID(qst.ID, 7)},
				&EnsureAttempt{dm.NewAttemptID(qst.ID, 8)},
				&EnsureAttempt{dm.NewAttemptID(qst.ID, 9)},
				&EnsureAttempt{dm.NewAttemptID(qst.ID, 10)},
				&EnsureAttempt{dm.NewAttemptID(qst.ID, 11)},
				&EnsureQuestAttempts{qst, []uint32{12}, true},
			})
		})
	})
}
