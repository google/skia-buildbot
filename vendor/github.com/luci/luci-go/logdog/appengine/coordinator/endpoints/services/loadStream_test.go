// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package services

import (
	"errors"
	"testing"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/luci/gae/filter/featureBreaker"
	"github.com/luci/luci-go/common/proto/google"
	"github.com/luci/luci-go/logdog/api/endpoints/coordinator/services/v1"
	"github.com/luci/luci-go/logdog/appengine/coordinator"
	ct "github.com/luci/luci-go/logdog/appengine/coordinator/coordinatorTest"

	. "github.com/luci/luci-go/common/testing/assertions"
	. "github.com/smartystreets/goconvey/convey"
)

func TestLoadStream(t *testing.T) {
	t.Parallel()

	Convey(`With a testing configuration`, t, func() {
		c, env := ct.Install()

		svr := New()

		// Register a test stream.
		tls := ct.MakeStream(c, "proj-foo", "testing/+/foo/bar")
		if err := tls.Put(c); err != nil {
			panic(err)
		}

		// Prepare a request to load the test stream.
		req := &logdog.LoadStreamRequest{
			Project: string(tls.Project),
			Id:      string(tls.Stream.ID),
		}

		Convey(`Returns Forbidden error if not a service.`, func() {
			_, err := svr.LoadStream(c, &logdog.LoadStreamRequest{})
			So(err, ShouldBeRPCPermissionDenied)
		})

		Convey(`When logged in as a service`, func() {
			env.JoinGroup("services")

			Convey(`Will succeed.`, func() {
				resp, err := svr.LoadStream(c, req)
				So(err, ShouldBeNil)
				So(resp, ShouldResemble, &logdog.LoadStreamResponse{
					State: &logdog.LogStreamState{
						ProtoVersion:  "1",
						TerminalIndex: -1,
						Secret:        tls.State.Secret,
					},
				})
			})

			Convey(`Will return archival properties.`, func() {
				// Add an hour to the clock. Created is +0, Updated is +1hr.
				env.Clock.Add(1 * time.Hour)
				tls.State.ArchivalKey = []byte("archival key")
				tls.Reload(c)
				if err := tls.Put(c); err != nil {
					panic(err)
				}

				// Set time to +2hr, age should now be 1hr.
				env.Clock.Add(1 * time.Hour)
				resp, err := svr.LoadStream(c, req)
				So(err, ShouldBeNil)
				So(resp, ShouldResemble, &logdog.LoadStreamResponse{
					State: &logdog.LogStreamState{
						ProtoVersion:  "1",
						TerminalIndex: -1,
						Secret:        tls.State.Secret,
					},
					ArchivalKey: []byte("archival key"),
					Age:         google.NewDuration(1 * time.Hour),
				})
			})

			Convey(`Will succeed, and return the descriptor when requested.`, func() {
				req.Desc = true

				d, err := proto.Marshal(tls.Desc)
				if err != nil {
					panic(err)
				}

				resp, err := svr.LoadStream(c, req)
				So(err, ShouldBeNil)
				So(resp, ShouldResemble, &logdog.LoadStreamResponse{
					State: &logdog.LogStreamState{
						ProtoVersion:  "1",
						TerminalIndex: -1,
						Secret:        tls.State.Secret,
					},
					Desc: d,
				})
			})

			Convey(`Will return InvalidArgument if the stream hash is not valid.`, func() {
				req.Id = string("!!! not a hash !!!")

				_, err := svr.LoadStream(c, req)
				So(err, ShouldBeRPCInvalidArgument, "Invalid ID")
			})

			Convey(`Will return NotFound for non-existent streams.`, func() {
				req.Id = string(coordinator.LogStreamID("this/stream/+/does/not/exist"))

				_, err := svr.LoadStream(c, req)
				So(err, ShouldBeRPCNotFound)
			})

			Convey(`Will return Internal for random datastore failures.`, func() {
				c, fb := featureBreaker.FilterRDS(c, nil)
				fb.BreakFeatures(errors.New("test error"), "GetMulti")

				_, err := svr.LoadStream(c, req)
				So(err, ShouldBeRPCInternal)
			})
		})
	})
}
