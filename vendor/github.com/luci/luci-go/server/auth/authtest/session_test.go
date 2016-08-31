// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package authtest

import (
	"testing"
	"time"

	"github.com/luci/luci-go/common/clock"
	"github.com/luci/luci-go/common/clock/testclock"
	"github.com/luci/luci-go/server/auth"
	"golang.org/x/net/context"

	. "github.com/smartystreets/goconvey/convey"
)

func TestSession(t *testing.T) {
	Convey("Works", t, func() {
		c, _ := testclock.UseTime(context.Background(), time.Unix(1442540000, 0))
		s := MemorySessionStore{}

		ss, err := s.GetSession(c, "missing")
		So(err, ShouldBeNil)
		So(ss, ShouldBeNil)

		sid, err := s.OpenSession(c, "uid", &auth.User{Name: "dude"}, clock.Now(c).Add(1*time.Hour))
		So(err, ShouldBeNil)
		So(sid, ShouldEqual, "uid/1")

		ss, err = s.GetSession(c, "uid/1")
		So(err, ShouldBeNil)
		So(ss, ShouldResemble, &auth.Session{
			SessionID: "uid/1",
			UserID:    "uid",
			User:      auth.User{Name: "dude"},
			Exp:       clock.Now(c).Add(1 * time.Hour),
		})

		So(s.CloseSession(c, "uid/1"), ShouldBeNil)

		ss, err = s.GetSession(c, "missing")
		So(err, ShouldBeNil)
		So(ss, ShouldBeNil)
	})
}
