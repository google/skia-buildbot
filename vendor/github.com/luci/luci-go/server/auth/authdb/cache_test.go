// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package authdb

import (
	"testing"
	"time"

	"golang.org/x/net/context"

	"github.com/luci/luci-go/common/clock/testclock"

	. "github.com/smartystreets/goconvey/convey"
)

func TestDBCache(t *testing.T) {
	Convey("DBCache works", t, func() {
		c := context.Background()
		c, tc := testclock.UseTime(c, time.Unix(1442540000, 0))

		factory := NewDBCache(func(c context.Context, prev DB) (DB, error) {
			if prev == nil {
				return &SnapshotDB{Rev: 1}, nil
			}
			cpy := *prev.(*SnapshotDB)
			cpy.Rev++
			return &cpy, nil
		})

		// Initial fetch.
		db, err := factory(c)
		So(err, ShouldBeNil)
		So(db.(*SnapshotDB).Rev, ShouldEqual, 1)

		// Refetch, using cached copy.
		db, err = factory(c)
		So(err, ShouldBeNil)
		So(db.(*SnapshotDB).Rev, ShouldEqual, 1)

		// Advance time to expire cache.
		tc.Add(11 * time.Second)

		// Returns new copy now.
		db, err = factory(c)
		So(err, ShouldBeNil)
		So(db.(*SnapshotDB).Rev, ShouldEqual, 2)
	})
}
