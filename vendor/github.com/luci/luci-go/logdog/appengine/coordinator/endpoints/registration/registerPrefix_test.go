// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package registration

import (
	"errors"
	"testing"
	"time"

	"github.com/luci/gae/filter/featureBreaker"
	ds "github.com/luci/gae/service/datastore"
	"github.com/luci/luci-go/common/clock"
	"github.com/luci/luci-go/common/config"
	"github.com/luci/luci-go/common/data/rand/cryptorand"
	"github.com/luci/luci-go/common/proto/google"
	"github.com/luci/luci-go/logdog/api/config/svcconfig"
	"github.com/luci/luci-go/logdog/api/endpoints/coordinator/registration/v1"
	"github.com/luci/luci-go/logdog/appengine/coordinator"
	ct "github.com/luci/luci-go/logdog/appengine/coordinator/coordinatorTest"
	"github.com/luci/luci-go/logdog/appengine/coordinator/hierarchy"
	"github.com/luci/luci-go/logdog/common/types"

	"golang.org/x/net/context"

	. "github.com/luci/luci-go/common/testing/assertions"
	. "github.com/smartystreets/goconvey/convey"
)

func TestRegisterPrefix(t *testing.T) {
	t.Parallel()

	Convey(`With a testing configuration`, t, func() {
		c, env := ct.Install()
		c, fb := featureBreaker.FilterRDS(c, nil)
		ds.Get(c).Testable().Consistent(true)

		// Mock random number generator so we can predict secrets.
		c = cryptorand.MockForTest(c, 0)
		randSecret := []byte{
			250, 18, 249, 42, 251, 224, 15, 133, 8, 208, 232, 59,
			171, 156, 248, 206, 191, 66, 226, 94, 139, 20, 234, 252,
			129, 234, 224, 208, 15, 44, 173, 228, 193, 124, 22, 209,
		}

		svr := New()

		const project = config.ProjectName("proj-foo")
		req := logdog.RegisterPrefixRequest{
			Project:    string(project),
			Prefix:     "testing/prefix",
			SourceInfo: []string{"unit test"},
		}
		pfx := &coordinator.LogPrefix{ID: coordinator.LogPrefixID(types.StreamName(req.Prefix))}

		Convey(`Returns PermissionDenied error if not user does not have write access.`, func() {
			// "proj-bar" does not have anonymous write.
			req.Project = "proj-bar"

			_, err := svr.RegisterPrefix(c, &req)
			So(err, ShouldBeRPCPermissionDenied)
		})

		Convey(`Will register a new prefix.`, func() {
			resp, err := svr.RegisterPrefix(c, &req)
			So(err, ShouldBeNil)
			So(resp, ShouldResemble, &logdog.RegisterPrefixResponse{
				LogBundleTopic: "projects/app/topics/test-topic",
				Secret:         randSecret,
			})

			ct.WithProjectNamespace(c, project, func(c context.Context) {
				So(ds.Get(c).Get(pfx), ShouldBeNil)
			})
			So(pfx, ShouldResemble, &coordinator.LogPrefix{
				Schema:  coordinator.CurrentSchemaVersion,
				ID:      pfx.ID,
				Prefix:  "testing/prefix",
				Created: ds.RoundTime(clock.Now(c)),
				Source:  []string{"unit test"},
				Secret:  randSecret,

				// 24 hours is default service prefix expiration.
				Expiration: ds.RoundTime(clock.Now(c).Add(24 * time.Hour)),
			})

			// Should have registered path components.
			getComponents := func(b string) []string {
				l, err := hierarchy.Get(c, hierarchy.Request{Project: "proj-foo", PathBase: b})
				if err != nil {
					panic(err)
				}
				names := make([]string, len(l.Comp))
				for i, e := range l.Comp {
					names[i] = e.Name
				}
				return names
			}
			So(getComponents(""), ShouldResemble, []string{"testing"})
			So(getComponents("testing"), ShouldResemble, []string{"prefix"})
			So(getComponents("testing/prefix"), ShouldResemble, []string{"+"})
			So(getComponents("testing/prefix/+"), ShouldResemble, []string{})

			Convey(`Will refuse to register it again.`, func() {
				_, err := svr.RegisterPrefix(c, &req)
				So(err, ShouldBeRPCAlreadyExists)
			})
		})

		Convey(`Uses the correct prefix expiration`, func() {

			Convey(`When service, project, and request have expiration, chooses smallest.`, func() {
				env.ModProjectConfig(c, project, func(pcfg *svcconfig.ProjectConfig) {
					pcfg.PrefixExpiration = google.NewDuration(time.Hour)
				})
				req.Expiration = google.NewDuration(time.Minute)

				_, err := svr.RegisterPrefix(c, &req)
				So(err, ShouldBeNil)

				ct.WithProjectNamespace(c, project, func(c context.Context) {
					So(ds.Get(c).Get(pfx), ShouldBeNil)
				})
				So(pfx.Expiration, ShouldResemble, clock.Now(c).Add(time.Minute))
			})

			Convey(`When service, and project have expiration, chooses smallest.`, func() {
				env.ModProjectConfig(c, project, func(pcfg *svcconfig.ProjectConfig) {
					pcfg.PrefixExpiration = google.NewDuration(time.Hour)
				})

				_, err := svr.RegisterPrefix(c, &req)
				So(err, ShouldBeNil)

				ct.WithProjectNamespace(c, project, func(c context.Context) {
					So(ds.Get(c).Get(pfx), ShouldBeNil)
				})
				So(pfx.Expiration, ShouldResemble, clock.Now(c).Add(time.Hour))
			})

			Convey(`When no expiration is defined, failed with internal error.`, func() {
				env.ModServiceConfig(c, func(cfg *svcconfig.Config) {
					cfg.Coordinator.PrefixExpiration = nil
				})

				_, err := svr.RegisterPrefix(c, &req)
				So(err, ShouldBeRPCInvalidArgument, "no prefix expiration defined")
			})
		})

		Convey(`Will fail to register the prefix if Put is broken.`, func() {
			fb.BreakFeatures(errors.New("test error"), "PutMulti")
			_, err := svr.RegisterPrefix(c, &req)
			So(err, ShouldBeRPCInternal)
		})
	})
}
