// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package settings

import (
	"testing"

	"github.com/luci/gae/impl/memory"
	lucicfg "github.com/luci/luci-go/common/config"
	memcfg "github.com/luci/luci-go/common/config/impl/memory"
	"github.com/luci/luci-go/common/logging/gologger"
	"github.com/luci/luci-go/server/auth"
	"github.com/luci/luci-go/server/auth/authtest"
	"github.com/luci/luci-go/server/auth/identity"
	"golang.org/x/net/context"

	. "github.com/smartystreets/goconvey/convey"
)

func TestACL(t *testing.T) {
	t.Parallel()

	Convey("Test Environment", t, func() {
		c := memory.UseWithAppID(context.Background(), "dev~luci-milo")
		c = gologger.StdConfig.Use(c)

		Convey("Set up projects", func() {
			c = lucicfg.SetImplementation(c, memcfg.New(aclConfgs))
			err := update(c)
			So(err, ShouldBeNil)

			Convey("Anon wants to...", func() {
				c = auth.WithState(c, &authtest.FakeState{
					Identity:       identity.AnonymousIdentity,
					IdentityGroups: []string{"all"},
				})
				Convey("Read public project", func() {
					ok, err := IsAllowed(c, "opensource")
					So(ok, ShouldEqual, true)
					So(err, ShouldBeNil)
				})
				Convey("Read private project", func() {
					ok, err := IsAllowed(c, "secret")
					So(ok, ShouldEqual, false)
					So(err, ShouldBeNil)
				})

			})
			Convey("alicebob@google.com wants to...", func() {
				c = auth.WithState(c, &authtest.FakeState{
					Identity:       "user:alicebob@google.com",
					IdentityGroups: []string{"google.com", "all"},
				})
				Convey("Read private project", func() {
					ok, err := IsAllowed(c, "secret")
					So(ok, ShouldEqual, true)
					So(err, ShouldBeNil)
				})
			})

			Convey("eve@notgoogle.com wants to...", func() {
				c = auth.WithState(c, &authtest.FakeState{
					Identity:       "user:eve@notgoogle.com",
					IdentityGroups: []string{"all"},
				})
				Convey("Read public project", func() {
					ok, err := IsAllowed(c, "opensource")
					So(ok, ShouldEqual, true)
					So(err, ShouldBeNil)
				})
				Convey("Read private project", func() {
					ok, err := IsAllowed(c, "secret")
					So(ok, ShouldEqual, false)
					So(err, ShouldBeNil)
				})
			})
		})
	})
}

var secretProjectCfg = `
ID: "secret"
Readers: "google.com"
`

var publicProjectCfg = `
ID: "opensource"
Readers: "all"
`

var aclConfgs = map[string]memcfg.ConfigSet{
	"projects/secret.git": {
		"luci-milo.cfg": secretProjectCfg,
	},
	"projects/opensource.git": {
		"luci-milo.cfg": publicProjectCfg,
	},
}
