// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/luci/gae/impl/memory"
	"github.com/luci/gae/service/urlfetch"
	"github.com/luci/luci-go/server/auth"
	"golang.org/x/net/context"

	. "github.com/luci/luci-go/common/testing/assertions"
	. "github.com/smartystreets/goconvey/convey"
)

type tokenInfo struct {
	Audience      string `json:"aud"`
	Email         string `json:"email"`
	EmailVerified string `json:"email_verified"`
	Error         string `json:"error_description"`
	ExpiresIn     string `json:"expires_in"`
	Scope         string `json:"scope"`
}

func TestOAuth2MethodDevServer(t *testing.T) {
	Convey("with mock backend", t, func(c C) {
		ctx := memory.Use(context.Background())
		ctx = urlfetch.Set(ctx, http.DefaultTransport)

		info := tokenInfo{
			Audience:      "client_id",
			Email:         "abc@example.com",
			EmailVerified: "true",
			ExpiresIn:     "3600",
			Scope:         EmailScope + " other stuff",
		}
		status := http.StatusOK
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(status)
			c.So(json.NewEncoder(w).Encode(&info), ShouldBeNil)
		}))

		call := func(header string) (*auth.User, error) {
			m := OAuth2Method{
				Scopes:            []string{EmailScope},
				tokenInfoEndpoint: ts.URL,
			}
			req, err := http.NewRequest("GET", "http://fake", nil)
			So(err, ShouldBeNil)
			req.Header.Set("Authorization", header)
			return m.Authenticate(ctx, req)
		}

		Convey("Works", func() {
			u, err := call("Bearer access_token")
			So(err, ShouldBeNil)
			So(u, ShouldResemble, &auth.User{
				Identity: "user:abc@example.com",
				Email:    "abc@example.com",
				ClientID: "client_id",
			})
		})

		Convey("Bad header", func() {
			_, err := call("broken")
			So(err, ShouldErrLike, "oauth: bad Authorization header")
		})

		Convey("HTTP 500", func() {
			status = http.StatusInternalServerError
			_, err := call("Bearer access_token")
			So(err, ShouldErrLike, "transient error")
		})

		Convey("Error response", func() {
			status = http.StatusBadRequest
			info.Error = "OMG, error"
			_, err := call("Bearer access_token")
			So(err, ShouldErrLike, "OMG, error")
		})

		Convey("No email", func() {
			info.Email = ""
			_, err := call("Bearer access_token")
			So(err, ShouldErrLike, "not associated with an email")
		})

		Convey("Email not verified", func() {
			info.EmailVerified = "false"
			_, err := call("Bearer access_token")
			So(err, ShouldErrLike, "not verified")
		})

		Convey("Bad expires_in", func() {
			info.ExpiresIn = "not a number"
			_, err := call("Bearer access_token")
			So(err, ShouldErrLike, "not a positive integer")
		})

		Convey("Zero expires_in", func() {
			info.ExpiresIn = "0"
			_, err := call("Bearer access_token")
			So(err, ShouldErrLike, "not a positive integer")
		})

		Convey("Missing scope", func() {
			info.Scope = "some other scopes"
			_, err := call("Bearer access_token")
			So(err, ShouldErrLike, "doesn't have scope")
		})

		Convey("Bad email", func() {
			info.Email = "@@@@"
			_, err := call("Bearer access_token")
			So(err, ShouldErrLike, "bad value")
		})
	})
}
