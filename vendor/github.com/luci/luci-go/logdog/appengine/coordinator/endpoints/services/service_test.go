// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package services

import (
	"testing"

	"github.com/luci/luci-go/logdog/api/endpoints/coordinator/services/v1"
	ct "github.com/luci/luci-go/logdog/appengine/coordinator/coordinatorTest"
	"github.com/luci/luci-go/server/auth"

	. "github.com/luci/luci-go/common/testing/assertions"
	. "github.com/smartystreets/goconvey/convey"
)

func TestServiceAuth(t *testing.T) {
	t.Parallel()

	Convey(`With a testing configuration`, t, func() {
		c, env := ct.Install()

		svr := New().(*logdog.DecoratedServices)

		Convey(`Will reject all traffic if no configuration is present.`, func() {
			env.ClearCoordinatorConfig(c)

			_, err := svr.Prelude(c, "test", nil)
			So(err, ShouldBeRPCInternal)
		})

		Convey(`With an application config installed`, func() {
			Convey(`Will reject users if there is an authentication error (no state).`, func() {
				c = auth.WithState(c, nil)

				_, err := svr.Prelude(c, "test", nil)
				So(err, ShouldBeRPCInternal)
			})

			Convey(`With an authentication state`, func() {
				Convey(`Will reject users who are not logged in.`, func() {
					_, err := svr.Prelude(c, "test", nil)
					So(err, ShouldBeRPCPermissionDenied)
				})

				Convey(`When a user is logged in`, func() {
					env.AuthState.Identity = "user:user@example.com"

					Convey(`Will reject users who are not members of the service group.`, func() {
						_, err := svr.Prelude(c, "test", nil)
						So(err, ShouldBeRPCPermissionDenied)
					})

					Convey(`Will allow users who are members of the service group.`, func() {
						env.JoinGroup("services")

						_, err := svr.Prelude(c, "test", nil)
						So(err, ShouldBeNil)
					})
				})
			})
		})
	})
}
