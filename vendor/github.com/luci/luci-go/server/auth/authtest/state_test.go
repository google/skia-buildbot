// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package authtest

import (
	"errors"
	"net"
	"testing"

	"github.com/luci/luci-go/server/auth"
	. "github.com/smartystreets/goconvey/convey"
)

func TestFakeState(t *testing.T) {
	Convey("Default FakeState works", t, func() {
		state := FakeState{}
		So(state.DB(), ShouldResemble, &FakeErroringDB{FakeDB: FakeDB{"anonymous:anonymous": nil}})
		So(state.Method(), ShouldNotBeNil)
		So(state.User(), ShouldResemble, &auth.User{Identity: "anonymous:anonymous"})
		So(state.PeerIdentity(), ShouldEqual, "anonymous:anonymous")
		So(state.PeerIP().String(), ShouldEqual, "127.0.0.1")
	})

	Convey("Non-default FakeState works", t, func() {
		state := FakeState{
			Identity:             "user:abc@def.com",
			IdentityGroups:       []string{"abc"},
			Error:                errors.New("boo"),
			PeerIdentityOverride: "bot:blah",
			PeerIPOverride:       net.ParseIP("192.192.192.192"),
		}
		So(state.DB(), ShouldResemble, &FakeErroringDB{
			FakeDB: FakeDB{"user:abc@def.com": []string{"abc"}},
			Error:  state.Error,
		})
		So(state.Method(), ShouldNotBeNil)
		So(state.User(), ShouldResemble, &auth.User{
			Identity: "user:abc@def.com",
			Email:    "abc@def.com",
		})
		So(state.PeerIdentity(), ShouldEqual, "bot:blah")
		So(state.PeerIP().String(), ShouldEqual, "192.192.192.192")
	})
}
