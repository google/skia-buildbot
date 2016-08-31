// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package authtest

import (
	"errors"
	"net/http"
	"net/url"

	"golang.org/x/net/context"

	"github.com/luci/luci-go/server/auth"
)

// ErrAuthenticationError is returned by FakeAuth.Authenticate.
var ErrAuthenticationError = errors.New("authtest: fake Authenticate error")

// FakeAuth implements auth.Method's Authenticate by returning predefined
// user.
type FakeAuth struct {
	User *auth.User // user to return in Authenticate or nil for error
}

// Authenticate returns predefined User object (if it is not nil) or error.
func (m FakeAuth) Authenticate(context.Context, *http.Request) (*auth.User, error) {
	if m.User == nil {
		return nil, ErrAuthenticationError
	}
	return m.User, nil
}

// LoginURL returns fake login URL.
func (m FakeAuth) LoginURL(c context.Context, dest string) (string, error) {
	return "http://fake/login?dest=" + url.QueryEscape(dest), nil
}

// LogoutURL returns fake logout URL.
func (m FakeAuth) LogoutURL(c context.Context, dest string) (string, error) {
	return "http://fake/logout?dest=" + url.QueryEscape(dest), nil
}
