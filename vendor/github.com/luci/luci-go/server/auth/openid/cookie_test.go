// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package openid

import (
	"net/http"
	"testing"
	"time"

	"golang.org/x/net/context"

	"github.com/luci/luci-go/common/clock"
	"github.com/luci/luci-go/common/clock/testclock"
	"github.com/luci/luci-go/server/secrets/testsecrets"

	. "github.com/smartystreets/goconvey/convey"
)

func TestCookie(t *testing.T) {
	Convey("With context", t, func() {
		c := context.Background()
		c, _ = testclock.UseTime(c, time.Unix(1442540000, 0))
		c = testsecrets.Use(c)

		Convey("Encode and decode works", func() {
			cookie, err := makeSessionCookie(c, "sid", true)
			So(err, ShouldBeNil)
			So(cookie, ShouldResemble, &http.Cookie{
				Name: "oid_session",
				Value: "AXsiX2kiOiIxNDQyNTQwMDAwMDAwIiwic2lkIjoic2lkIn1NXPzKTFXWhzt" +
					"tmqW2uODV4f1Nvt1zLxAnWTtjqkhGEQ",
				Path:     "/",
				Expires:  clock.Now(c).Add(2591100 * time.Second),
				MaxAge:   2591100,
				Secure:   true,
				HttpOnly: true,
			})

			r, err := http.NewRequest("GET", "http://example.com", nil)
			So(err, ShouldBeNil)
			r.AddCookie(cookie)

			sid, err := decodeSessionCookie(c, r)
			So(err, ShouldBeNil)
			So(sid, ShouldEqual, "sid")
		})

		Convey("Bad cookie is ignored", func() {
			r, err := http.NewRequest("GET", "http://example.com", nil)
			So(err, ShouldBeNil)
			r.AddCookie(&http.Cookie{
				Name:     "oid_session",
				Value:    "garbage",
				Path:     "/",
				Expires:  clock.Now(c).Add(2591100 * time.Second),
				MaxAge:   2591100,
				Secure:   true,
				HttpOnly: true,
			})
			sid, err := decodeSessionCookie(c, r)
			So(err, ShouldBeNil)
			So(sid, ShouldEqual, "")
		})

		Convey("Expired session token is ignored", func() {
			r, err := http.NewRequest("GET", "http://example.com", nil)
			So(err, ShouldBeNil)
			r.AddCookie(&http.Cookie{
				Name: "oid_session",
				Value: "AXsiX2kiOiIxNDQyNTQwMDAwMDAwIiwic2lkIjoic2lkIn1NXPzKTFXWhzt" +
					"tmqW2uODV4f1Nvt1zLxAnWTtjqkhGEQ",
				Path:     "/",
				Expires:  clock.Now(c).Add(2591100 * time.Second),
				MaxAge:   2591100,
				Secure:   true,
				HttpOnly: true,
			})

			// Works now.
			sid, err := decodeSessionCookie(c, r)
			So(err, ShouldBeNil)
			So(sid, ShouldEqual, "sid")

			// Doesn't work after expiration.
			clock.Get(c).(testclock.TestClock).Add(2600000 * time.Second)
			sid, err = decodeSessionCookie(c, r)
			So(err, ShouldBeNil)
			So(sid, ShouldEqual, "")
		})
	})
}
