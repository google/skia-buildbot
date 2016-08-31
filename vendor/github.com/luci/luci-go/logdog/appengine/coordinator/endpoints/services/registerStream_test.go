// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package services

import (
	"errors"
	"testing"
	"time"

	"github.com/luci/gae/filter/featureBreaker"
	ds "github.com/luci/gae/service/datastore"
	"github.com/luci/luci-go/common/proto/google"
	"github.com/luci/luci-go/logdog/api/config/svcconfig"
	"github.com/luci/luci-go/logdog/api/endpoints/coordinator/services/v1"
	"github.com/luci/luci-go/logdog/api/logpb"
	"github.com/luci/luci-go/logdog/appengine/coordinator"
	ct "github.com/luci/luci-go/logdog/appengine/coordinator/coordinatorTest"
	"github.com/luci/luci-go/logdog/appengine/coordinator/hierarchy"
	"golang.org/x/net/context"

	. "github.com/luci/luci-go/common/testing/assertions"
	. "github.com/smartystreets/goconvey/convey"
)

func TestRegisterStream(t *testing.T) {
	t.Parallel()

	Convey(`With a testing configuration`, t, func() {
		c, env := ct.Install()

		// Set our archival delays. The project delay is smaller than the service
		// delay, so it should be used.
		env.ModServiceConfig(c, func(cfg *svcconfig.Config) {
			cfg.Coordinator.ArchiveSettleDelay = google.NewDuration(10 * time.Minute)
			cfg.Coordinator.ArchiveDelayMax = google.NewDuration(24 * time.Hour)
		})
		env.ModProjectConfig(c, "proj-foo", func(pcfg *svcconfig.ProjectConfig) {
			pcfg.MaxStreamAge = google.NewDuration(time.Hour)
		})

		// By default, the testing user is a service.
		env.JoinGroup("services")

		svr := New()

		Convey(`Returns Forbidden error if not a service.`, func() {
			env.LeaveAllGroups()

			_, err := svr.RegisterStream(c, &logdog.RegisterStreamRequest{})
			So(err, ShouldBeRPCPermissionDenied)
		})

		Convey(`When registering a testing log sream, "testing/+/foo/bar"`, func() {
			tls := ct.MakeStream(c, "proj-foo", "testing/+/foo/bar")

			req := logdog.RegisterStreamRequest{
				Project:       string(tls.Project),
				Secret:        tls.Prefix.Secret,
				ProtoVersion:  logpb.Version,
				Desc:          tls.DescBytes(),
				TerminalIndex: -1,
			}

			Convey(`Returns FailedPrecondition when the Prefix is not registered.`, func() {
				_, err := svr.RegisterStream(c, &req)
				So(err, ShouldBeRPCFailedPrecondition)
			})

			Convey(`When the Prefix is registered`, func() {
				tls.WithProjectNamespace(c, func(c context.Context) {
					if err := ds.Get(c).Put(tls.Prefix); err != nil {
						panic(err)
					}
				})

				expResp := &logdog.RegisterStreamResponse{
					Id: string(tls.Stream.ID),
					State: &logdog.LogStreamState{
						Secret:        tls.Prefix.Secret,
						ProtoVersion:  logpb.Version,
						TerminalIndex: -1,
					},
				}

				Convey(`Can register the stream.`, func() {
					created := ds.RoundTime(env.Clock.Now())

					resp, err := svr.RegisterStream(c, &req)
					So(err, ShouldBeRPCOK)
					So(resp, ShouldResemble, expResp)
					ds.Get(c).Testable().CatchupIndexes()
					env.IterateTumbleAll(c)

					So(tls.Get(c), ShouldBeNil)

					// Registers the log stream.
					So(tls.Stream.Created, ShouldResemble, created)

					// Registers the log stream state.
					So(tls.State.Created, ShouldResemble, created)
					So(tls.State.Updated, ShouldResemble, created)
					So(tls.State.Secret, ShouldResemble, req.Secret)
					So(tls.State.TerminalIndex, ShouldEqual, -1)
					So(tls.State.Terminated(), ShouldBeFalse)
					So(tls.State.ArchivalState(), ShouldEqual, coordinator.NotArchived)

					// Should also register the log stream Prefix.
					So(tls.Prefix.Created, ShouldResemble, created)
					So(tls.Prefix.Secret, ShouldResemble, req.Secret)

					// No archival request yet.
					So(env.ArchivalPublisher.Hashes(), ShouldResemble, []string{})

					// Should have name components.
					getNameComponents := func(b string) []string {
						l, err := hierarchy.Get(c, hierarchy.Request{Project: string(tls.Project), PathBase: b})
						if err != nil {
							panic(err)
						}
						names := make([]string, len(l.Comp))
						for i, e := range l.Comp {
							names[i] = e.Name
							if e.Stream {
								names[i] += "$"
							}
						}
						return names
					}
					So(getNameComponents(""), ShouldResemble, []string{"testing"})
					So(getNameComponents("testing"), ShouldResemble, []string{"+"})
					So(getNameComponents("testing/+"), ShouldResemble, []string{"foo"})
					So(getNameComponents("testing/+/foo"), ShouldResemble, []string{"bar$"})

					Convey(`Can register the stream again (idempotent).`, func() {
						env.Clock.Set(created.Add(10 * time.Minute))

						resp, err := svr.RegisterStream(c, &req)
						So(err, ShouldBeRPCOK)
						So(resp, ShouldResemble, expResp)

						tls.WithProjectNamespace(c, func(c context.Context) {
							So(ds.Get(c).Get(tls.Stream, tls.State), ShouldBeNil)
						})
						So(tls.State.Created, ShouldResemble, created)
						So(tls.Stream.Created, ShouldResemble, created)

						// No archival request yet.
						So(env.ArchivalPublisher.Hashes(), ShouldResemble, []string{})

						Convey(`Forces an archival request after first archive expiration.`, func() {
							env.Clock.Set(created.Add(time.Hour)) // 1 hour after initial registration.
							env.IterateTumbleAll(c)

							So(env.ArchivalPublisher.Hashes(), ShouldResemble, []string{string(tls.Stream.ID)})
						})
					})

					Convey(`Will not re-register if secrets don't match.`, func() {
						req.Secret[0] = 0xAB
						_, err := svr.RegisterStream(c, &req)
						So(err, ShouldBeRPCInvalidArgument, "invalid secret")
					})
				})

				Convey(`Can register a terminal stream.`, func() {
					created := ds.RoundTime(env.Clock.Now())
					req.TerminalIndex = 1337
					expResp.State.TerminalIndex = 1337

					resp, err := svr.RegisterStream(c, &req)
					So(err, ShouldBeRPCOK)
					So(resp, ShouldResemble, expResp)
					ds.Get(c).Testable().CatchupIndexes()
					env.IterateTumbleAll(c)

					So(tls.Get(c), ShouldBeNil)

					// Registers the log stream.
					So(tls.Stream.Created, ShouldResemble, created)

					// Registers the log stream state.
					So(tls.State.Created, ShouldResemble, created)
					So(tls.State.Updated, ShouldResemble, created)
					So(tls.State.Secret, ShouldResemble, req.Secret)
					So(tls.State.TerminalIndex, ShouldEqual, 1337)
					So(tls.State.TerminatedTime, ShouldResemble, created)
					So(tls.State.Terminated(), ShouldBeTrue)
					So(tls.State.ArchivalState(), ShouldEqual, coordinator.NotArchived)

					// Should also register the log stream Prefix.
					So(tls.Prefix.Created, ShouldResemble, created)
					So(tls.Prefix.Secret, ShouldResemble, req.Secret)

					// No pending archival requests.
					env.IterateTumbleAll(c)
					So(env.ArchivalPublisher.Hashes(), ShouldResemble, []string{})

					// When we advance to our settle delay, an archival task is scheduled.
					env.Clock.Add(10 * time.Minute)
					env.IterateTumbleAll(c)

					// Has a pending archival request.
					So(env.ArchivalPublisher.Hashes(), ShouldResemble, []string{string(tls.Stream.ID)})
				})

				Convey(`Will schedule the correct archival expiration delay`, func() {
					Convey(`When there is no project config delay.`, func() {
						env.ModProjectConfig(c, "proj-foo", func(pcfg *svcconfig.ProjectConfig) {
							pcfg.MaxStreamAge = nil
						})

						_, err := svr.RegisterStream(c, &req)
						So(err, ShouldBeRPCOK)
						ds.Get(c).Testable().CatchupIndexes()

						// The cleanup archival should be scheduled for 24 hours, so advance
						// 12, confirm no archival, then advance another 12 and confirm that
						// archival was tasked.
						env.Clock.Add(12 * time.Hour)
						env.IterateTumbleAll(c)
						So(env.ArchivalPublisher.Hashes(), ShouldHaveLength, 0)

						env.Clock.Add(12 * time.Hour)
						env.IterateTumbleAll(c)
						So(env.ArchivalPublisher.Hashes(), ShouldResemble, []string{string(tls.Stream.ID)})
					})

					Convey(`When there is no service or project config delay.`, func() {
						env.ModServiceConfig(c, func(cfg *svcconfig.Config) {
							cfg.Coordinator.ArchiveDelayMax = nil
						})
						env.ModProjectConfig(c, "proj-foo", func(pcfg *svcconfig.ProjectConfig) {
							pcfg.MaxStreamAge = nil
						})

						_, err := svr.RegisterStream(c, &req)
						So(err, ShouldBeRPCOK)
						ds.Get(c).Testable().CatchupIndexes()

						// The cleanup archival should be scheduled immediately.
						env.IterateTumbleAll(c)
						So(env.ArchivalPublisher.Hashes(), ShouldResemble, []string{string(tls.Stream.ID)})
					})
				})

				Convey(`Returns internal server error if the datastore Get() fails.`, func() {
					c, fb := featureBreaker.FilterRDS(c, nil)
					fb.BreakFeatures(errors.New("test error"), "GetMulti")

					_, err := svr.RegisterStream(c, &req)
					So(err, ShouldBeRPCInternal)
				})

				Convey(`Returns internal server error if the Prefix Put() fails.`, func() {
					c, fb := featureBreaker.FilterRDS(c, nil)
					fb.BreakFeatures(errors.New("test error"), "PutMulti")

					_, err := svr.RegisterStream(c, &req)
					So(err, ShouldBeRPCInternal)
				})

				Convey(`Registration failure cases`, func() {
					Convey(`Will not register a stream if its prefix has expired.`, func() {
						env.Clock.Set(tls.Prefix.Expiration)

						_, err := svr.RegisterStream(c, &req)
						So(err, ShouldBeRPCFailedPrecondition, "prefix has expired")
					})

					Convey(`Will not register a stream without a protobuf version.`, func() {
						req.ProtoVersion = ""
						_, err := svr.RegisterStream(c, &req)
						So(err, ShouldBeRPCInvalidArgument, "Unrecognized protobuf version")
					})

					Convey(`Will not register a stream with an unknown protobuf version.`, func() {
						req.ProtoVersion = "unknown"
						_, err := svr.RegisterStream(c, &req)
						So(err, ShouldBeRPCInvalidArgument, "Unrecognized protobuf version")
					})

					Convey(`Will not register with an empty descriptor.`, func() {
						req.Desc = nil

						_, err := svr.RegisterStream(c, &req)
						So(err, ShouldBeRPCInvalidArgument, "Invalid log stream descriptor")
					})

					Convey(`Will not register if the descriptor doesn't validate.`, func() {
						tls.Desc.ContentType = ""
						So(tls.Desc.Validate(true), ShouldNotBeNil)
						req.Desc = tls.DescBytes()

						_, err := svr.RegisterStream(c, &req)
						So(err, ShouldBeRPCInvalidArgument, "Invalid log stream descriptor")
					})
				})

				Convey(`The registerStreamMutation`, func() {
					svcs := coordinator.GetServices(c)
					cfg, err := svcs.Config(c)
					if err != nil {
						panic(err)
					}
					pcfg, err := svcs.ProjectConfig(c, "proj-foo")
					if err != nil {
						panic(err)
					}

					rsm := registerStreamMutation{
						RegisterStreamRequest: &req,

						cfg:  cfg,
						pcfg: pcfg,

						desc: tls.Desc,
						pfx:  tls.Prefix,
						ls:   tls.Stream,
						lst:  tls.State,
					}

					Convey(`Can RollForward.`, func() {
						_, err := rsm.RollForward(c)
						So(err, ShouldBeNil)
					})

					Convey(`Returns internal server error if the stream Put() fails (tumble).`, func() {
						c, fb := featureBreaker.FilterRDS(c, nil)
						fb.BreakFeatures(errors.New("test error"), "PutMulti")

						_, err := rsm.RollForward(c)
						So(err, ShouldBeRPCInternal)
					})
				})
			})
		})
	})
}
