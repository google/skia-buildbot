// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package auth

import (
	"testing"

	"golang.org/x/net/context"

	"github.com/luci/luci-go/server/auth/identity"
	. "github.com/smartystreets/goconvey/convey"
)

func TestState(t *testing.T) {
	Convey("Check empty ctx", t, func() {
		ctx := context.Background()
		So(GetState(ctx), ShouldBeNil)
		So(CurrentUser(ctx).Identity, ShouldEqual, identity.AnonymousIdentity)
		So(CurrentIdentity(ctx), ShouldEqual, identity.AnonymousIdentity)

		res, err := IsMember(ctx, "group")
		So(res, ShouldBeFalse)
		So(err, ShouldEqual, ErrNoAuthState)
	})

	Convey("Check non-empty ctx", t, func() {
		s := state{
			db:        &fakeDB{},
			user:      &User{Identity: "user:abc@example.com"},
			peerIdent: "user:abc@example.com",
		}
		ctx := context.WithValue(context.Background(), stateContextKey(0), &s)
		So(GetState(ctx), ShouldNotBeNil)
		So(GetState(ctx).Method(), ShouldBeNil)
		So(GetState(ctx).PeerIdentity(), ShouldEqual, identity.Identity("user:abc@example.com"))
		So(GetState(ctx).PeerIP(), ShouldBeNil)
		So(CurrentUser(ctx).Identity, ShouldEqual, identity.Identity("user:abc@example.com"))
		So(CurrentIdentity(ctx), ShouldEqual, identity.Identity("user:abc@example.com"))

		res, err := IsMember(ctx, "group")
		So(err, ShouldBeNil)
		So(res, ShouldBeTrue) // fakeDB always returns true
	})
}
