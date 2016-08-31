// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package auth

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"golang.org/x/net/context"

	"github.com/luci/luci-go/common/errors"
	"github.com/luci/luci-go/server/auth/identity"
	"github.com/luci/luci-go/server/router"
	. "github.com/smartystreets/goconvey/convey"
)

func TestContext(t *testing.T) {
	Convey("Works", t, func() {
		c := context.Background()

		So(getAuthenticator(c), ShouldBeNil)
		_, err := LoginURL(c, "dest")
		So(err, ShouldEqual, ErrNoUsersAPI)
		_, err = LogoutURL(c, "dest")
		So(err, ShouldEqual, ErrNoUsersAPI)

		// Authenticator without UsersAPI.
		c = SetAuthenticator(c, Authenticator{noUserAPI{}})

		So(getAuthenticator(c), ShouldNotBeNil)
		_, err = LoginURL(c, "dest")
		So(err, ShouldEqual, ErrNoUsersAPI)
		_, err = LogoutURL(c, "dest")
		So(err, ShouldEqual, ErrNoUsersAPI)

		// Authenticator with UsersAPI.
		c = SetAuthenticator(c, Authenticator{fakeMethod{}})

		So(getAuthenticator(c), ShouldNotBeNil)
		dest, err := LoginURL(c, "dest")
		So(err, ShouldBeNil)
		So(dest, ShouldEqual, "http://login_url?r=dest")
		dest, err = LogoutURL(c, "dest")
		So(err, ShouldBeNil)
		So(dest, ShouldEqual, "http://logout_url?r=dest")
	})

}

func TestContextAuthenticate(t *testing.T) {
	call := func(c context.Context, m router.MiddlewareChain, h router.Handler) *httptest.ResponseRecorder {
		req, err := http.NewRequest("GET", "http://example.com/foo", nil)
		So(err, ShouldBeNil)
		w := httptest.NewRecorder()
		router.RunMiddleware(&router.Context{
			Context: c,
			Writer:  w,
			Request: req,
		}, m, h)
		return w
	}

	handler := func(c *router.Context) {
		fmt.Fprintf(c.Writer, "%s", CurrentIdentity(c.Context))
	}

	Convey("Not configured", t, func() {
		rr := call(context.Background(), router.NewMiddlewareChain(Authenticate), handler)
		So(rr.Code, ShouldEqual, 500)
		So(rr.Body.String(), ShouldEqual, "Authentication middleware is not configured\n")
	})

	Convey("Transient error", t, func() {
		c := prepareCtx(fakeMethod{authError: errors.WrapTransient(errors.New("boo"))})
		rr := call(c, router.NewMiddlewareChain(Authenticate), handler)
		So(rr.Code, ShouldEqual, 500)
		So(rr.Body.String(), ShouldEqual, "Transient error during authentication - boo\n")
	})

	Convey("Fatal error", t, func() {
		c := prepareCtx(fakeMethod{authError: errors.New("boo")})
		rr := call(c, router.NewMiddlewareChain(Authenticate), handler)
		So(rr.Code, ShouldEqual, 401)
		So(rr.Body.String(), ShouldEqual, "Authentication error - boo\n")
	})

	Convey("Works", t, func() {
		c := prepareCtx(fakeMethod{userID: "user:abc@example.com"})
		rr := call(c, router.NewMiddlewareChain(Authenticate), handler)
		So(rr.Code, ShouldEqual, 200)
		So(rr.Body.String(), ShouldEqual, "user:abc@example.com")
	})

	Convey("Anonymous works", t, func() {
		c := prepareCtx(fakeMethod{anon: true})
		rr := call(c, router.NewMiddlewareChain(Authenticate), handler)
		So(rr.Code, ShouldEqual, 200)
		So(rr.Body.String(), ShouldEqual, "anonymous:anonymous")
	})

	Convey("Broken ID is rejected", t, func() {
		c := prepareCtx(fakeMethod{userID: "???"})
		rr := call(c, router.NewMiddlewareChain(Authenticate), handler)
		So(rr.Code, ShouldEqual, 401)
		So(rr.Body.String(), ShouldEqual, "Authentication error - auth: bad identity string \"???\"\n")
	})
}

func TestAutologin(t *testing.T) {
	call := func(c context.Context, m router.MiddlewareChain, h router.Handler) *httptest.ResponseRecorder {
		req, err := http.NewRequest("GET", "http://example.com/foo", nil)
		So(err, ShouldBeNil)
		w := httptest.NewRecorder()
		router.RunMiddleware(&router.Context{
			Context: c,
			Writer:  w,
			Request: req,
		}, m, h)
		return w
	}

	handler := func(c *router.Context) {
		fmt.Fprintf(c.Writer, "%s", CurrentIdentity(c.Context))
	}

	Convey("Not configured", t, func() {
		rr := call(context.Background(), router.NewMiddlewareChain(Autologin), handler)
		So(rr.Code, ShouldEqual, 500)
		So(rr.Body.String(), ShouldEqual, "Authentication middleware is not configured\n")
	})

	Convey("Transient error", t, func() {
		c := prepareCtx(fakeMethod{authError: errors.WrapTransient(errors.New("boo"))})
		rr := call(c, router.NewMiddlewareChain(Autologin), handler)
		So(rr.Code, ShouldEqual, 500)
		So(rr.Body.String(), ShouldEqual, "Transient error during authentication - boo\n")
	})

	Convey("Fatal error", t, func() {
		c := prepareCtx(fakeMethod{authError: errors.New("boo")})
		rr := call(c, router.NewMiddlewareChain(Autologin), handler)
		So(rr.Code, ShouldEqual, 401)
	})

	Convey("Anonymous is redirected to login if has UsersAPI", t, func() {
		c := prepareCtx(fakeMethod{anon: true})
		rr := call(c, router.NewMiddlewareChain(Autologin), handler)
		So(rr.Code, ShouldEqual, 302)
		So(rr.Header().Get("Location"), ShouldEqual, "http://login_url?r=%2Ffoo")
	})

	Convey("Anonymous is rejected if no UsersAPI", t, func() {
		c := prepareCtx(noUserAPI{})
		rr := call(c, router.NewMiddlewareChain(Autologin), handler)
		So(rr.Code, ShouldEqual, 401)
		So(rr.Body.String(), ShouldEqual, "Authentication error - auth: methods do not support login or logout URL\n")
	})

	Convey("Handles transient error in LoginURL", t, func() {
		c := prepareCtx(fakeMethod{anon: true, loginURLError: errors.WrapTransient(errors.New("boo"))})
		rr := call(c, router.NewMiddlewareChain(Autologin), handler)
		So(rr.Code, ShouldEqual, 500)
		So(rr.Body.String(), ShouldEqual, "Transient error during authentication - boo\n")
	})

	Convey("Passes authenticated user through", t, func() {
		c := prepareCtx(fakeMethod{userID: "user:abc@example.com"})
		rr := call(c, router.NewMiddlewareChain(Autologin), handler)
		So(rr.Code, ShouldEqual, 200)
		So(rr.Body.String(), ShouldEqual, "user:abc@example.com")
	})
}

func prepareCtx(m ...Method) context.Context {
	c := injectTestDB(context.Background(), &fakeDB{})
	return SetAuthenticator(c, Authenticator(m))
}

type noUserAPI struct{}

func (noUserAPI) Authenticate(context.Context, *http.Request) (*User, error) {
	return nil, nil
}

type fakeMethod struct {
	authError     error
	loginURLError error
	userID        identity.Identity
	anon          bool
}

func (m fakeMethod) Authenticate(context.Context, *http.Request) (*User, error) {
	if m.anon {
		return nil, nil
	}
	if m.authError != nil {
		return nil, m.authError
	}
	return &User{Identity: m.userID}, nil
}

func (m fakeMethod) LoginURL(c context.Context, dest string) (string, error) {
	if m.loginURLError != nil {
		return "", m.loginURLError
	}
	v := url.Values{}
	v.Set("r", dest)
	return "http://login_url?" + v.Encode(), nil
}

func (m fakeMethod) LogoutURL(c context.Context, dest string) (string, error) {
	v := url.Values{}
	v.Set("r", dest)
	return "http://logout_url?" + v.Encode(), nil
}
