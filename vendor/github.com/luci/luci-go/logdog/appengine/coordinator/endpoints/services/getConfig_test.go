// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package services

import (
	"testing"

	"github.com/luci/luci-go/logdog/api/config/svcconfig"
	"github.com/luci/luci-go/logdog/api/endpoints/coordinator/services/v1"
	ct "github.com/luci/luci-go/logdog/appengine/coordinator/coordinatorTest"

	. "github.com/luci/luci-go/common/testing/assertions"
	. "github.com/smartystreets/goconvey/convey"
)

func TestGetConfig(t *testing.T) {
	t.Parallel()

	Convey(`With a testing configuration`, t, func() {
		c, env := ct.Install()

		svr := New()

		Convey(`Returns Forbidden error if not a service.`, func() {
			_, err := svr.GetConfig(c, nil)
			So(err, ShouldBeRPCPermissionDenied)
		})

		Convey(`When logged in as a service, can retrieve the configuration.`, func() {
			env.JoinGroup("services")

			cr, err := svr.GetConfig(c, nil)
			So(err, ShouldBeRPCOK)
			So(cr, ShouldResemble, &logdog.GetConfigResponse{
				ConfigServiceUrl:  "memory://",
				ConfigSet:         "services/app",
				ServiceConfigPath: svcconfig.ServiceConfigFilename,
			})
		})
	})
}
