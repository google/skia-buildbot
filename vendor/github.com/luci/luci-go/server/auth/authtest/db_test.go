// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package authtest

import (
	"errors"
	"testing"

	"golang.org/x/net/context"

	"github.com/luci/luci-go/server/auth/identity"
	. "github.com/smartystreets/goconvey/convey"
)

func TestFakeDB(t *testing.T) {
	Convey("FakeDB works", t, func() {
		c := context.Background()
		db := FakeDB{
			"user:abc@def.com": []string{"group a", "group b"},
		}

		resp, err := db.IsMember(c, identity.Identity("user:abc@def.com"), "group b")
		So(err, ShouldBeNil)
		So(resp, ShouldBeTrue)

		resp, err = db.IsMember(c, identity.Identity("user:another@def.com"), "group b")
		So(err, ShouldBeNil)
		So(resp, ShouldBeFalse)

		resp, err = db.IsMember(c, identity.Identity("user:abc@def.com"), "another")
		So(err, ShouldBeNil)
		So(resp, ShouldBeFalse)
	})
}

func TestFakeErroringDB(t *testing.T) {
	Convey("FakeErroringDB works", t, func() {
		c := context.Background()
		db := FakeErroringDB{
			FakeDB: FakeDB{"user:abc@def.com": []string{"group a", "group b"}},
			Error:  errors.New("boo"),
		}

		_, err := db.IsMember(c, identity.Identity("user:abc@def.com"), "group a")
		So(err.Error(), ShouldEqual, "boo")

		db.Error = nil
		resp, err := db.IsMember(c, identity.Identity("user:abc@def.com"), "group a")
		So(err, ShouldBeNil)
		So(resp, ShouldBeTrue)
	})
}
