// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package tumble

import (
	"testing"

	"github.com/luci/gae/service/datastore"

	"github.com/luci/luci-go/common/logging"
	"github.com/luci/luci-go/common/logging/memlogger"
	. "github.com/smartystreets/goconvey/convey"
)

func TestAddToJournal(t *testing.T) {
	t.Parallel()

	Convey("Test AddToJournal", t, func() {
		ttest := &Testing{}
		c := ttest.Context()
		ds := datastore.Get(c)
		ml := logging.Get(c).(*memlogger.MemLogger)

		Convey("with no mutations", func() {
			So(AddToJournal(c), ShouldBeNil)
			So(ml.HasFunc(func(e *memlogger.LogEntry) bool {
				return e.Msg == "tumble.AddToJournal" && e.Data["count"].(int) == 0
			}), ShouldBeTrue)
		})

		Convey("with some mutations", func() {
			from := &User{Name: "fromUser"}
			So(ds.Put(from), ShouldBeNil)
			So(AddToJournal(c, &WriteMessage{
				from.MakeOutgoingMessage(c, "hey there", "a", "b")}), ShouldBeNil)

			qry := datastore.NewQuery("tumble.Mutation").Ancestor(ds.MakeKey("tumble.temp", "8c60aac4ffd6e66142bef4e745d9d91546c115d18cc8283723699d964422a47a"))
			pmaps := []datastore.PropertyMap{}
			So(ds.GetAll(qry, &pmaps), ShouldBeNil)
			So(len(pmaps), ShouldEqual, 1)
			So(pmaps[0]["$key"][0].Value().(*datastore.Key).String(), ShouldEqual,
				`dev~app::/tumble.temp,"8c60aac4ffd6e66142bef4e745d9d91546c115d18cc8283723699d964422a47a"/tumble.Mutation,"0000000000000000_00000000_00000000"`)
		})
	})
}
