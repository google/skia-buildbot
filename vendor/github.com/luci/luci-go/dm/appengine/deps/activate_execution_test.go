// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package deps

import (
	"testing"

	. "github.com/luci/luci-go/common/testing/assertions"
	dm "github.com/luci/luci-go/dm/api/service/v1"
	"github.com/luci/luci-go/dm/appengine/distributor/fake"
	. "github.com/smartystreets/goconvey/convey"
)

func TestActivateExecution(t *testing.T) {
	t.Parallel()

	Convey("Test ActivateExecution", t, func() {
		ttest, c, dist, s := testSetup()

		qid := s.ensureQuest(c, "foo", 1)
		ttest.Drain(c)

		eid := dm.NewExecutionID(qid, 1, 1)
		dist.RunTask(c, eid, func(tsk *fake.Task) error {
			req := &dm.ActivateExecutionReq{
				Auth:           &dm.Execution_Auth{Id: eid},
				ExecutionToken: []byte("sufficiently long new 'random' token"),
			}

			Convey("bad", func() {
				Convey("wrong token", func() {
					_, err := s.ActivateExecution(c, req)
					So(err, ShouldBeRPCPermissionDenied, "failed to activate")
				})

				Convey("wrong token (already activated)", func() {
					req.Auth.Token = tsk.Auth.Token
					_, err := s.ActivateExecution(c, req)
					So(err, ShouldBeNil)

					req.Auth.Token = []byte("bad sekret")
					req.ExecutionToken = []byte("random other super duper long token")
					_, err = s.ActivateExecution(c, req)
					So(err, ShouldBeRPCPermissionDenied, "failed to activate")
				})

				Convey("concurrent activation", func() {
					req.Auth.Token = tsk.Auth.Token
					_, err := s.ActivateExecution(c, req)
					So(err, ShouldBeNil)

					req.ExecutionToken = append(req.ExecutionToken, []byte(" (but incorrect)")...)
					_, err = s.ActivateExecution(c, req)
					So(err, ShouldBeRPCPermissionDenied, "failed to activate")
				})
			})

			Convey("good", func() {
				req.Auth.Token = tsk.Auth.Token

				Convey("normal activation", func() {
					_, err := s.ActivateExecution(c, req)
					So(err, ShouldBeNil)
				})

				Convey("repeated activation", func() {
					_, err := s.ActivateExecution(c, req)
					So(err, ShouldBeNil)
					_, err = s.ActivateExecution(c, req)
					So(err, ShouldBeNil)
				})
			})
			return nil
		})
	})
}
