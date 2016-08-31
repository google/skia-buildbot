// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package gaemiddleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/luci/luci-go/appengine/gaetesting"
	"github.com/luci/luci-go/server/router"
	. "github.com/smartystreets/goconvey/convey"
	"golang.org/x/net/context"
)

func init() {
	// disable this so that we can actually check the logic in these middlewares
	devAppserverBypassFn = func(context.Context) bool { return false }
}

func TestRequireCron(t *testing.T) {
	t.Parallel()

	Convey("Test RequireCron", t, func() {
		hit := false
		f := func(c *router.Context) {
			hit = true
			c.Writer.Write([]byte("ok"))
		}

		Convey("from non-cron fails", func() {
			rec := httptest.NewRecorder()
			c := &router.Context{
				Context: gaetesting.TestingContext(),
				Writer:  rec,
				Request: &http.Request{},
			}
			router.RunMiddleware(c, router.NewMiddlewareChain(RequireCron), f)
			So(hit, ShouldBeFalse)
			So(rec.Body.String(), ShouldEqual, "error: must be run from cron")
			So(rec.Code, ShouldEqual, http.StatusForbidden)
		})

		Convey("from cron succeeds", func() {
			rec := httptest.NewRecorder()
			c := &router.Context{
				Context: gaetesting.TestingContext(),
				Writer:  rec,
				Request: &http.Request{
					Header: http.Header{
						http.CanonicalHeaderKey("x-appengine-cron"): []string{"true"},
					},
				},
			}
			router.RunMiddleware(c, router.NewMiddlewareChain(RequireCron), f)
			So(hit, ShouldBeTrue)
			So(rec.Body.String(), ShouldEqual, "ok")
			So(rec.Code, ShouldEqual, http.StatusOK)
		})
	})
}

func TestRequireTQ(t *testing.T) {
	t.Parallel()

	Convey("Test RequireTQ", t, func() {
		hit := false
		f := func(c *router.Context) {
			hit = true
			c.Writer.Write([]byte("ok"))
		}

		Convey("from non-tq fails (wat)", func() {
			rec := httptest.NewRecorder()
			c := &router.Context{
				Context: gaetesting.TestingContext(),
				Writer:  rec,
				Request: &http.Request{},
			}
			router.RunMiddleware(c, router.NewMiddlewareChain(RequireTaskQueue("wat")), f)
			So(hit, ShouldBeFalse)
			So(rec.Body.String(), ShouldEqual, "error: must be run from the correct taskqueue")
			So(rec.Code, ShouldEqual, http.StatusForbidden)
		})

		Convey("from non-tq fails", func() {
			rec := httptest.NewRecorder()
			c := &router.Context{
				Context: gaetesting.TestingContext(),
				Writer:  rec,
				Request: &http.Request{},
			}
			router.RunMiddleware(c, router.NewMiddlewareChain(RequireTaskQueue("")), f)
			So(hit, ShouldBeFalse)
			So(rec.Body.String(), ShouldEqual, "error: must be run from the correct taskqueue")
			So(rec.Code, ShouldEqual, http.StatusForbidden)
		})

		Convey("from wrong tq fails (wat)", func() {
			rec := httptest.NewRecorder()
			c := &router.Context{
				Context: gaetesting.TestingContext(),
				Writer:  rec,
				Request: &http.Request{
					Header: http.Header{
						http.CanonicalHeaderKey("x-appengine-queuename"): []string{"else"},
					},
				},
			}
			router.RunMiddleware(c, router.NewMiddlewareChain(RequireTaskQueue("wat")), f)
			So(hit, ShouldBeFalse)
			So(rec.Body.String(), ShouldEqual, "error: must be run from the correct taskqueue")
			So(rec.Code, ShouldEqual, http.StatusForbidden)
		})

		Convey("from right tq succeeds (wat)", func() {
			rec := httptest.NewRecorder()
			c := &router.Context{
				Context: gaetesting.TestingContext(),
				Writer:  rec,
				Request: &http.Request{
					Header: http.Header{
						http.CanonicalHeaderKey("x-appengine-queuename"): []string{"wat"},
					},
				},
			}
			router.RunMiddleware(c, router.NewMiddlewareChain(RequireTaskQueue("wat")), f)
			So(hit, ShouldBeTrue)
			So(rec.Body.String(), ShouldEqual, "ok")
			So(rec.Code, ShouldEqual, http.StatusOK)
		})

		Convey("from any tq succeeds", func() {
			rec := httptest.NewRecorder()
			c := &router.Context{
				Context: gaetesting.TestingContext(),
				Writer:  rec,
				Request: &http.Request{
					Header: http.Header{
						http.CanonicalHeaderKey("x-appengine-queuename"): []string{"wat"},
					},
				},
			}
			router.RunMiddleware(c, router.NewMiddlewareChain(RequireTaskQueue("")), f)
			So(hit, ShouldBeTrue)
			So(rec.Body.String(), ShouldEqual, "ok")
			So(rec.Code, ShouldEqual, http.StatusOK)
		})
	})
}
