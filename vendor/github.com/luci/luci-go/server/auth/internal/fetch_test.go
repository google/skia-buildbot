// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package internal

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"golang.org/x/net/context"

	. "github.com/luci/luci-go/common/testing/assertions"
	. "github.com/smartystreets/goconvey/convey"
)

func init() {
	RegisterClientFactory(func(c context.Context, scopes []string) (*http.Client, error) {
		return http.DefaultClient, nil
	})
}

func TestFetch(t *testing.T) {
	Convey("with test context", t, func(c C) {
		body := ""
		status := http.StatusOK
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(status)
			w.Write([]byte(body))
		}))
		ctx := context.Background()

		Convey("fetch works", func() {
			var val struct {
				A string `json:"a"`
			}
			body = `{"a": "hello"}`
			req := Request{
				Method: "GET",
				URL:    ts.URL,
				Out:    &val,
			}
			So(req.Do(ctx), ShouldBeNil)
			So(val.A, ShouldEqual, "hello")
		})

		Convey("handles bad status code", func() {
			var val struct{}
			status = http.StatusNotFound
			req := Request{
				Method: "GET",
				URL:    ts.URL,
				Out:    &val,
			}
			So(req.Do(ctx), ShouldErrLike, "HTTP code (404)")
		})

		Convey("handles bad body", func() {
			var val struct{}
			body = "not json"
			req := Request{
				Method: "GET",
				URL:    ts.URL,
				Out:    &val,
			}
			So(req.Do(ctx), ShouldErrLike, "can't deserialize JSON")
		})

		Convey("handles connection error", func() {
			var val struct{}
			req := Request{
				Method: "GET",
				URL:    "http://localhost:???",
				Out:    &val,
			}
			So(req.Do(ctx), ShouldErrLike, "dial tcp")
		})
	})
}
