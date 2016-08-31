// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package server

import (
	"net/http"

	"golang.org/x/net/context"

	"github.com/luci/gae/service/user"
	"github.com/luci/luci-go/server/auth"
	"github.com/luci/luci-go/server/auth/identity"
)

// UsersAPIAuthMethod implements auth.Method and auth.UsersAPI interfaces on top
// of GAE Users API (that uses HTTP cookies internally to track user sessions).
type UsersAPIAuthMethod struct{}

// Authenticate extracts peer's identity from the incoming request.
func (m UsersAPIAuthMethod) Authenticate(c context.Context, r *http.Request) (*auth.User, error) {
	u := user.Get(c).Current()
	if u == nil {
		return nil, nil
	}
	id, err := identity.MakeIdentity("user:" + u.Email)
	if err != nil {
		return nil, err
	}
	return &auth.User{
		Identity:  id,
		Superuser: u.Admin,
		Email:     u.Email,
	}, nil
}

// LoginURL returns a URL that, when visited, prompts the user to sign in,
// then redirects the user to the URL specified by dest.
func (m UsersAPIAuthMethod) LoginURL(c context.Context, dest string) (string, error) {
	return user.Get(c).LoginURL(dest)
}

// LogoutURL returns a URL that, when visited, signs the user out,
// then redirects the user to the URL specified by dest.
func (m UsersAPIAuthMethod) LogoutURL(c context.Context, dest string) (string, error) {
	return user.Get(c).LogoutURL(dest)
}
