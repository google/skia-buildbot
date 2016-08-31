// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package model

import (
	"testing"

	"golang.org/x/net/context"

	. "github.com/smartystreets/goconvey/convey"

	"github.com/luci/gae/impl/memory"
	"github.com/luci/gae/service/datastore"
	"github.com/luci/luci-go/common/clock/testclock"
	google_pb "github.com/luci/luci-go/common/proto/google"
	. "github.com/luci/luci-go/common/testing/assertions"

	dm "github.com/luci/luci-go/dm/api/service/v1"
)

func TestQuest(t *testing.T) {
	t.Parallel()

	Convey("Quest", t, func() {
		c := memory.Use(context.Background())
		c, _ = testclock.UseTime(c, testclock.TestTimeUTC)

		Convey("QuestDescriptor", func() {
			Convey("good", func() {
				Convey("normal (normalized)", func() {
					qd := dm.NewQuestDesc("swarming", `{  "key"  :  ["value"]}`, "{  }", nil)
					So(qd.Normalize(), ShouldBeNil)
					So(NewQuest(c, qd), ShouldResemble, &Quest{
						"1258phYs8GW6qM5AQopQ_L3A5cZhO7iaYQZyFkNusVw",
						*qd,
						nil,
						testclock.TestTimeUTC,
					})
				})

				Convey("extra data", func() {
					qd := dm.NewQuestDesc("swarming", `{"key":["value"]} foof`, "{  }", nil)
					So(qd.Normalize(), ShouldErrLike, "extra junk")
				})

				Convey("data ordering", func() {
					qd := dm.NewQuestDesc("swarming", `{"key":["value"], "abc": true}`, "{  }", nil)
					So(qd.Normalize(), ShouldBeNil)
					So(NewQuest(c, qd), ShouldResemble, &Quest{
						"IMTBeXfkZgGntgNfWMuLa_YQA62o9dzxi0EoLCYXbsE",
						*qd,
						nil,
						testclock.TestTimeUTC,
					})
				})

			})

			Convey("bad", func() {
				Convey("payload too large", func() {
					payload := make([]byte, 512*1000)
					qd := dm.NewQuestDesc("swarming", string(payload), "{}", nil)
					So(qd.Normalize(), ShouldErrLike, "too large: 512002 > 262144")
				})

				Convey("json with null byte", func() {
					qd := dm.NewQuestDesc("swarming", "{\"key\": \"\x00\"}", "{}", nil)
					So(qd.Normalize(), ShouldErrLike, "invalid character")
				})

				Convey("not a dictionary", func() {
					qd := dm.NewQuestDesc("swarming", "[]", "{}", nil)
					So(qd.Normalize(), ShouldErrLike, "cannot unmarshal array")
				})
			})
		})

		Convey("ToProto", func() {
			qd := dm.NewQuestDesc("swarming", `{"key": ["value"]}`, "{}", nil)
			So(qd.Normalize(), ShouldBeNil)
			q := NewQuest(c, qd)
			p := q.ToProto()
			So(p, ShouldResemble, &dm.Quest{
				Id: dm.NewQuestID("1258phYs8GW6qM5AQopQ_L3A5cZhO7iaYQZyFkNusVw"),
				Data: &dm.Quest_Data{
					Created: google_pb.NewTimestamp(testclock.TestTimeUTC),
					Desc:    &q.Desc,
					BuiltBy: []*dm.Quest_TemplateSpec{},
				},
			})
			So(p.Data.Desc.Parameters, ShouldResemble, `{"key":["value"]}`)
		})

		Convey("QueryAttemptsForQuest", func() {
			qd := dm.NewQuestDesc("swarming", `{"key": ["value"]}`, "{}", nil)
			So(qd.Normalize(), ShouldBeNil)
			q := NewQuest(c, qd)
			ds := datastore.Get(c)
			So(ds.Put(q), ShouldBeNil)
			ds.Testable().CatchupIndexes()

			as := []*Attempt(nil)
			So(ds.GetAll(QueryAttemptsForQuest(c, q.ID), &as), ShouldBeNil)
			So(as, ShouldBeNil)

			a := &Attempt{ID: *dm.NewAttemptID(q.ID, 1)}
			So(ds.Put(a), ShouldBeNil)
			a.ID.Id = 2
			So(ds.Put(a), ShouldBeNil)
			a.ID.Quest = "eMpqiyje5ItTX8IistN7IlAMVxyCsJcez4DAHKvhm7X" // one less
			a.ID.Id = 1
			So(ds.Put(a), ShouldBeNil)
			a.ID.Quest = "eMpqiyje5ItTX8IistN7IlAMVxyCsJcez4DAHKvhm7Z" // one more
			So(ds.Put(a), ShouldBeNil)

			as = nil
			So(ds.GetAll(QueryAttemptsForQuest(c, q.ID), &as), ShouldBeNil)
			So(as, ShouldBeNil)

			ds.Testable().CatchupIndexes()
			as = nil
			So(ds.GetAll(QueryAttemptsForQuest(c, q.ID), &as), ShouldBeNil)
			So(as, ShouldResemble, []*Attempt{
				{ID: *dm.NewAttemptID("1258phYs8GW6qM5AQopQ_L3A5cZhO7iaYQZyFkNusVw", 2)},
				{ID: *dm.NewAttemptID("1258phYs8GW6qM5AQopQ_L3A5cZhO7iaYQZyFkNusVw", 1)},
			})

		})
	})
}
