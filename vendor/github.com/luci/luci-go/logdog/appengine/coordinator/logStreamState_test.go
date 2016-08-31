// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package coordinator

import (
	"bytes"
	"testing"
	"time"

	"github.com/luci/gae/impl/memory"
	ds "github.com/luci/gae/service/datastore"
	"github.com/luci/luci-go/common/clock/testclock"
	"github.com/luci/luci-go/logdog/common/types"
	"golang.org/x/net/context"

	. "github.com/luci/luci-go/common/testing/assertions"
	. "github.com/smartystreets/goconvey/convey"
)

func TestLogStreamState(t *testing.T) {
	t.Parallel()

	Convey(`A testing log stream state`, t, func() {
		c, tc := testclock.UseTime(context.Background(), testclock.TestTimeLocal)
		c = memory.Use(c)

		if err := WithProjectNamespace(&c, "proj-foo", NamespaceAccessAllTesting); err != nil {
			panic(err)
		}
		di := ds.Get(c)
		di.Testable().Consistent(true)

		now := ds.RoundTime(tc.Now().UTC())
		ls := LogStream{ID: LogStreamID("testing/+/log/stream")}
		lst := ls.State(di)

		lst.Schema = CurrentSchemaVersion
		lst.Created = now.UTC()
		lst.Updated = now.UTC()
		lst.Secret = bytes.Repeat([]byte{0x55}, types.PrefixSecretLength)
		lst.TerminalIndex = -1

		Convey(`Will validate`, func() {
			So(lst.Validate(), ShouldBeNil)
		})

		Convey(`Will not validate`, func() {
			Convey(`Without a valid created time`, func() {
				lst.Created = time.Time{}
				So(lst.Validate(), ShouldErrLike, "missing created time")
			})
			Convey(`Without a valid updated time`, func() {
				lst.Updated = time.Time{}
				So(lst.Validate(), ShouldErrLike, "missing updated time")
			})
			Convey(`Without a valid stream secret`, func() {
				lst.Secret = nil
				So(lst.Validate(), ShouldErrLike, "invalid prefix secret length")
			})
			Convey(`With a terminal index, will not validate without a TerminatedTime.`, func() {
				lst.ArchivalKey = []byte{0xd0, 0x65}
				lst.TerminalIndex = 1337
				So(lst.Validate(), ShouldErrLike, "missing terminated time")

				lst.TerminatedTime = now
				So(lst.Validate(), ShouldBeNil)
			})
		})

		Convey(`Can write to the Datastore.`, func() {
			So(di.Put(lst), ShouldBeNil)

			Convey(`Can be queried`, func() {
				q := ds.NewQuery("LogStreamState")

				// runQuery will run the query, panic if it fails, and return whether the
				// single record, "lst", was returned.
				runQuery := func(q *ds.Query) bool {
					var states []*LogStreamState
					if err := di.GetAll(q, &states); err != nil {
						panic(err)
					}
					return len(states) > 0
				}

				Convey(`A non-terminated stream will satisfy !Terminated, !Archived`, func() {
					So(runQuery(q.Eq("_Terminated", false)), ShouldBeTrue)
					So(runQuery(q.Eq("_ArchivalState", NotArchived)), ShouldBeTrue)
				})

				Convey(`A terminated stream will satisfy Terminated, but !Archived`, func() {
					lst.TerminalIndex = 1337
					lst.TerminatedTime = now
					So(di.Put(lst), ShouldBeNil)

					So(runQuery(q.Eq("_Terminated", true)), ShouldBeTrue)
					So(runQuery(q.Eq("_ArchivalState", NotArchived)), ShouldBeTrue)
				})

				Convey(`An incomplete archived stream will satisfy Terminated, and ArchivedPartial`, func() {
					lst.TerminalIndex = 10
					lst.ArchiveLogEntryCount = 9
					lst.ArchivedTime = now
					So(di.Put(lst), ShouldBeNil)

					So(runQuery(q.Eq("_Terminated", true)), ShouldBeTrue)
					So(runQuery(q.Eq("_ArchivalState", ArchivedPartial)), ShouldBeTrue)
				})

				Convey(`A complete archived stream will satisfy Terminated, and ArchivedComplete`, func() {
					lst.TerminalIndex = 10
					lst.ArchiveLogEntryCount = 11
					lst.ArchivedTime = now
					So(di.Put(lst), ShouldBeNil)

					So(runQuery(q.Eq("_Terminated", true)), ShouldBeTrue)
					So(runQuery(q.Eq("_ArchivalState", ArchivedComplete)), ShouldBeTrue)
				})
			})
		})
	})
}
