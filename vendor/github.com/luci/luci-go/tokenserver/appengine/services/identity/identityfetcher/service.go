// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

// Package identityfetcher implements IdentityFetcher API.
package identityfetcher

import (
	"golang.org/x/net/context"

	"github.com/luci/luci-go/common/proto/google"
	"github.com/luci/luci-go/server/auth"

	"github.com/luci/luci-go/tokenserver/api/identity/v1"
)

// Server implements identity.IdentityFetcher RPC interface.
type Server struct{}

// GetCallerIdentity returns caller identity as understood by the auth layer.
func (s *Server) GetCallerIdentity(c context.Context, _ *google.Empty) (*identity.CallerIdentity, error) {
	state := auth.GetState(c)
	if state == nil {
		panic("impossible, auth middleware must be configured")
	}
	user := state.User()
	return &identity.CallerIdentity{
		Identity:       string(user.Identity),
		PeerIdentity:   string(state.PeerIdentity()),
		PeerIp:         state.PeerIP().String(),
		Oauth2ClientId: user.ClientID,
	}, nil
}
