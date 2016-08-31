// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package authtest

import (
	"net"

	"github.com/luci/luci-go/server/auth"
	"github.com/luci/luci-go/server/auth/authdb"
	"github.com/luci/luci-go/server/auth/identity"
)

// FakeState implements auth.State returning predefined values.
//
// Inject it into the context when testing handlers that expect auth state:
//
//   ctx = auth.WithState(ctx, &authtest.FakeState{
//     Identity: "user:user@example.com",
//     IdentityGroups: []string{"admins"},
//   })
//   auth.IsMember(ctx, "admins") -> returns true.
type FakeState struct {
	// Identity is main identity associated with the request.
	//
	// identity.AnonymousIdentity if not set.
	Identity identity.Identity

	// IdentityGroups is list of groups the calling identity belongs to.
	IdentityGroups []string

	// Error if not nil is returned by IsMember checks.
	Error error

	// PeerIdentityOverride may be set for PeerIdentity() to return custom value.
	//
	// By default PeerIdentity() returns Identity (i.e. no delegation is
	// happening).
	PeerIdentityOverride identity.Identity

	// PeerIPOverride may be set for PeerIP() to return custom value.
	//
	// By default PeerIP() returns "127.0.0.1".
	PeerIPOverride net.IP
}

var _ auth.State = (*FakeState)(nil)

// DB is part of State interface.
func (s *FakeState) DB() authdb.DB {
	return &FakeErroringDB{
		FakeDB: FakeDB{s.User().Identity: s.IdentityGroups},
		Error:  s.Error,
	}
}

// Method is part of State interface.
func (s *FakeState) Method() auth.Method {
	return &FakeAuth{
		User: s.User(),
	}
}

// User is part of State interface.
func (s *FakeState) User() *auth.User {
	ident := identity.AnonymousIdentity
	if s.Identity != "" {
		ident = s.Identity
	}
	return &auth.User{
		Identity: ident,
		Email:    ident.Email(),
	}
}

// PeerIdentity is part of State interface.
func (s *FakeState) PeerIdentity() identity.Identity {
	if s.PeerIdentityOverride == "" {
		return s.User().Identity
	}
	return s.PeerIdentityOverride
}

// PeerIP is part of State interface.
func (s *FakeState) PeerIP() net.IP {
	if s.PeerIPOverride == nil {
		return net.ParseIP("127.0.0.1")
	}
	return s.PeerIPOverride
}
