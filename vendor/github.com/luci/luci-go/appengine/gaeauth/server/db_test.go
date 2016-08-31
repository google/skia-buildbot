// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package server

import (
	"fmt"
	"testing"

	"golang.org/x/net/context"

	"github.com/luci/gae/service/datastore"
	"github.com/luci/luci-go/appengine/gaeauth/server/internal/authdbimpl"
	"github.com/luci/luci-go/appengine/gaetesting"
	"github.com/luci/luci-go/server/auth/authdb"
	"github.com/luci/luci-go/server/auth/service"
	"github.com/luci/luci-go/server/auth/service/protocol"

	. "github.com/smartystreets/goconvey/convey"
)

func TestGetAuthDB(t *testing.T) {
	Convey("Unconfigured", t, func() {
		c := gaetesting.TestingContext()
		authDB, err := GetAuthDB(c, nil)
		So(err, ShouldBeNil)
		So(authDB, ShouldHaveSameTypeAs, devServerDB{})
	})

	Convey("Reuses instance if no changes", t, func() {
		c := gaetesting.TestingContext()

		bumpAuthDB(c, 123)
		authDB, err := GetAuthDB(c, nil)
		So(err, ShouldBeNil)
		So(authDB, ShouldHaveSameTypeAs, &authdb.SnapshotDB{})
		So(authDB.(*authdb.SnapshotDB).Rev, ShouldEqual, 123)

		newOne, err := GetAuthDB(c, authDB)
		So(err, ShouldBeNil)
		So(newOne, ShouldEqual, authDB) // exact same pointer

		bumpAuthDB(c, 124)
		anotherOne, err := GetAuthDB(c, authDB)
		So(err, ShouldBeNil)
		So(anotherOne, ShouldHaveSameTypeAs, &authdb.SnapshotDB{})
		So(anotherOne.(*authdb.SnapshotDB).Rev, ShouldEqual, 124)
	})
}

///

func strPtr(s string) *string {
	return &s
}

func bumpAuthDB(c context.Context, rev int64) {
	blob, err := service.DeflateAuthDB(&protocol.AuthDB{
		OauthClientId:     strPtr(fmt.Sprintf("client-id-for-rev-%d", rev)),
		OauthClientSecret: strPtr("secret"),
	})
	if err != nil {
		panic(err)
	}
	info := authdbimpl.SnapshotInfo{
		AuthServiceURL: "https://fake-auth-service",
		Rev:            rev,
	}
	ds := datastore.Get(c)
	if err = ds.Put(&info); err != nil {
		panic(err)
	}
	err = ds.Put(&authdbimpl.Snapshot{
		ID:             info.GetSnapshotID(),
		AuthDBDeflated: blob,
	})
	if err != nil {
		panic(err)
	}
}
