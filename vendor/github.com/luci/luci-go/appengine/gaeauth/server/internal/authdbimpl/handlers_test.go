// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package authdbimpl

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"golang.org/x/net/context"

	"github.com/luci/luci-go/appengine/gaetesting"
	"github.com/luci/luci-go/server/auth/service"
	"github.com/luci/luci-go/server/router"

	. "github.com/smartystreets/goconvey/convey"
)

func TestPubSubHandlers(t *testing.T) {
	Convey("pubSubPush skips when not configured", t, func() {
		c := gaetesting.TestingContext()
		rec := httptest.NewRecorder()
		pubSubPush(&router.Context{
			Context: c,
			Writer:  rec,
			Request: makePostRequest(),
		})
		So(rec.Code, ShouldEqual, 200)
		So(rec.Body.String(), ShouldEqual, "Auth Service URL is not configured, skipping the message")
	})

	Convey("pubSubPush works when no notification", t, func() {
		c, _ := setupCtx()
		rec := httptest.NewRecorder()
		pubSubPush(&router.Context{
			Context: c,
			Writer:  rec,
			Request: makePostRequest(),
		})
		So(rec.Code, ShouldEqual, 200)
		So(rec.Body.String(), ShouldEqual, "No new valid AuthDB change notifications")
	})

	Convey("pubSubPush old notification", t, func() {
		c, srv := setupCtx()
		srv.Notification = &service.Notification{Revision: 122} // older than 123
		rec := httptest.NewRecorder()
		pubSubPush(&router.Context{
			Context: c,
			Writer:  rec,
			Request: makePostRequest(),
		})
		So(rec.Code, ShouldEqual, 200)
		So(rec.Body.String(), ShouldEqual, "Processed PubSub notification for rev 122: 123 -> 123")
	})

	Convey("pubSubPush fresh notification", t, func() {
		c, srv := setupCtx()
		srv.LatestRev = 130
		srv.Notification = &service.Notification{Revision: 124}
		rec := httptest.NewRecorder()
		pubSubPush(&router.Context{
			Context: c,
			Writer:  rec,
			Request: makePostRequest(),
		})
		So(rec.Code, ShouldEqual, 200)
		So(rec.Body.String(), ShouldEqual, "Processed PubSub notification for rev 124: 123 -> 130")
	})
}

func setupCtx() (context.Context, *fakeAuthService) {
	srv := &fakeAuthService{LatestRev: 123}
	c := setAuthService(gaetesting.TestingContext(), srv)
	So(ConfigureAuthService(c, "http://base_url", "http://auth-service"), ShouldBeNil)
	return c, srv
}

func makePostRequest() *http.Request {
	req, _ := http.NewRequest("POST", "/doesntmatter", bytes.NewReader(nil))
	return req
}
