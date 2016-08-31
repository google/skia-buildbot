// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package server

import (
	"net/http"

	"golang.org/x/net/context"

	"github.com/luci/luci-go/server/auth"
	"github.com/luci/luci-go/server/auth/identity"
)

// InboundAppIDAuthMethod implements auth.Method by checking special HTTP
// header (X-Appengine-Inbound-Appid), that is set iff one GAE app talks to
// another.
type InboundAppIDAuthMethod struct{}

// Authenticate extracts peer's identity from the incoming request.
func (m InboundAppIDAuthMethod) Authenticate(c context.Context, r *http.Request) (*auth.User, error) {
	appID := r.Header.Get("X-Appengine-Inbound-Appid")
	if appID == "" {
		return nil, nil
	}
	id, err := identity.MakeIdentity("service:" + appID)
	if err != nil {
		return nil, err
	}
	return &auth.User{Identity: id}, nil
}
