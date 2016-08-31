// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package main

import (
	"testing"

	"github.com/luci/luci-go/deploytool/api/deploy"

	. "github.com/smartystreets/goconvey/convey"
)

func TestCloudProjectVersion(t *testing.T) {
	Convey(`A cloud project version`, t, func() {
		b := cloudProjectVersionBuilder{
			currentUser: func() (string, error) {
				return "test-person", nil
			},
		}
		cp := layoutDeploymentCloudProject{
			Deployment_CloudProject: &deploy.Deployment_CloudProject{
				VersionScheme: deploy.Deployment_CloudProject_DEFAULT,
			},
		}
		src := layoutSource{
			FrozenLayout_Source: &deploy.FrozenLayout_Source{
				MinorVersion: "--minor",
				MajorVersion: "major",
			},
			sg: &layoutSourceGroup{
				FrozenLayout_SourceGroup: &deploy.FrozenLayout_SourceGroup{},
			},
		}

		Convey(`Renders and parses without a tainted user.`, func() {
			v, err := b.build(&cp, &src)
			So(err, ShouldBeNil)

			s := v.String()
			So(s, ShouldEqual, "__minor-major")

			parsed, err := parseCloudProjectVersion(deploy.Deployment_CloudProject_DEFAULT, s)
			So(err, ShouldBeNil)
			So(parsed, ShouldResemble, v)
		})

		Convey(`Renders and parses with a tainted user.`, func() {
			src.sg.Tainted = true

			v, err := b.build(&cp, &src)
			So(err, ShouldBeNil)

			s := v.String()
			So(s, ShouldEqual, "__minor-major-tainted-test_person")

			parsed, err := parseCloudProjectVersion(deploy.Deployment_CloudProject_DEFAULT, s)
			So(err, ShouldBeNil)
			So(parsed, ShouldResemble, v)
		})
	})
}
