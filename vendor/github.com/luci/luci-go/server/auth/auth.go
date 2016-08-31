// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package auth

import (
	"fmt"
	"net/http"

	"golang.org/x/net/context"

	"github.com/luci/luci-go/common/errors"
	"github.com/luci/luci-go/common/logging"

	"github.com/luci/luci-go/server/auth/authdb"
	"github.com/luci/luci-go/server/auth/delegation"
	"github.com/luci/luci-go/server/auth/identity"
	"github.com/luci/luci-go/server/auth/signing"
)

var (
	// ErrNotConfigured is returned by Authenticate if auth library wasn't
	// properly initialized (see SetConfig).
	ErrNotConfigured = errors.New("auth: the library is not properly configured")

	// ErrNoUsersAPI is returned by LoginURL and LogoutURL if none of
	// the authentication methods support UsersAPI.
	ErrNoUsersAPI = errors.New("auth: methods do not support login or logout URL")

	// ErrBadClientID is returned by Authenticate if caller is using
	// non-whitelisted OAuth2 client. More info is in the log.
	ErrBadClientID = errors.New("auth: OAuth client_id is not whitelisted")

	// ErrIPNotWhitelisted is returned when an account is restricted by an IP
	// whitelist and request's remote_addr is not in it.
	ErrIPNotWhitelisted = errors.New("auth: IP is not whitelisted")
)

// Method implements particular kind of low level authentication mechanism for
// incoming requests. It may also optionally implement UsersAPI (if the method
// support login and logout URLs). Use type sniffing to figure out.
type Method interface {
	// Authenticate extracts user information from the incoming request.
	// It returns:
	//   * (*User, nil) on success.
	//   * (nil, nil) if the method is not applicable.
	//   * (nil, error) if the method is applicable, but credentials are invalid.
	Authenticate(context.Context, *http.Request) (*User, error)
}

// UsersAPI may be additionally implemented by Method if it supports login and
// logout URLs.
type UsersAPI interface {
	// LoginURL returns a URL that, when visited, prompts the user to sign in,
	// then redirects the user to the URL specified by dest.
	LoginURL(c context.Context, dest string) (string, error)

	// LogoutURL returns a URL that, when visited, signs the user out,
	// then redirects the user to the URL specified by dest.
	LogoutURL(c context.Context, dest string) (string, error)
}

// User represents identity and profile of a user.
type User struct {
	// Identity is identity string of the user (may be AnonymousIdentity).
	// If User is returned by Authenticate(...), Identity string is always present
	// and valid.
	Identity identity.Identity

	// Superuser is true if the user is site-level administrator. For example, on
	// GAE this bit is set for GAE-level administrators. Optional, default false.
	Superuser bool

	// Email is email of the user. Optional, default "". Don't use it as a key
	// in various structures. Prefer to use Identity() instead (it is always
	// available).
	Email string

	// Name is full name of the user. Optional, default "".
	Name string

	// Picture is URL of the user avatar. Optional, default "".
	Picture string

	// ClientID is the ID of the pre-registered OAuth2 client so its identity can
	// be verified. Used only by authentication methods based on OAuth2.
	// See https://developers.google.com/console/help/#generatingoauth2 for more.
	ClientID string
}

// Authenticator perform authentication of incoming requests. It is stateless
// object that just describes what methods to try when authenticating current
// request. It is fine to create it on per-request basis.
type Authenticator []Method

// Authenticate authenticates incoming requests and returns new context.Context
// with State stored into it. Returns error if credentials are provided, but
// invalid. If no credentials are provided (i.e. the request is anonymous),
// finishes successfully, but in that case State.Identity() will return
// AnonymousIdentity.
func (a Authenticator) Authenticate(c context.Context, r *http.Request) (context.Context, error) {
	// We will need working DB factory below to check IP whitelist.
	cfg := GetConfig(c)
	if cfg == nil || cfg.DBProvider == nil {
		return nil, ErrNotConfigured
	}

	// Pick first authentication method that applies.
	var s state
	for _, m := range a {
		var err error
		s.user, err = m.Authenticate(c, r)
		if err != nil {
			return nil, err
		}
		if s.user != nil {
			if err = s.user.Identity.Validate(); err != nil {
				return nil, err
			}
			s.method = m
			break
		}
	}

	// If no authentication method is applicable, default to anonymous identity.
	if s.method == nil {
		s.user = &User{Identity: identity.AnonymousIdentity}
	}

	var err error
	s.peerIP, err = parseRemoteIP(r.RemoteAddr)
	if err != nil {
		panic(fmt.Errorf("auth: bad remote_addr: %v", err))
	}

	// Grab a snapshot of auth DB to use consistently for the duration of this
	// request.
	s.db, err = cfg.DBProvider(c)
	if err != nil {
		return nil, ErrNotConfigured
	}

	// If using OAuth2, make sure ClientID is whitelisted.
	if s.user.ClientID != "" {
		valid, err := s.db.IsAllowedOAuthClientID(c, s.user.Email, s.user.ClientID)
		if err != nil {
			return nil, err
		}
		if !valid {
			logging.Warningf(
				c, "auth: %q is using client_id %q not in the whitelist",
				s.user.Email, s.user.ClientID)
			return nil, ErrBadClientID
		}
	}

	// Some callers may be constrained by an IP whitelist.
	switch ipWhitelist, err := s.db.GetWhitelistForIdentity(c, s.user.Identity); {
	case err != nil:
		return nil, err
	case ipWhitelist != "":
		switch whitelisted, err := s.db.IsInWhitelist(c, s.peerIP, ipWhitelist); {
		case err != nil:
			return nil, err
		case !whitelisted:
			return nil, ErrIPNotWhitelisted
		}
	}

	// peerIdent always matches the identity of a remote peer. It may be different
	// from s.user.Identity if the delegation is used (see below).
	s.peerIdent = s.user.Identity

	// Check the delegation token. This is LUCI-specific authentication protocol.
	// Delegation tokens are generated by the central auth service (see luci-py's
	// auth_service) and validated by checking their RSA signature using auth
	// server's public keys.
	delegationTok := r.Header.Get(delegation.HTTPHeaderName)
	if delegationTok != "" {
		// Need to grab our own identity to verify that the delegation token is
		// minted for consumption by us and not some other service.
		ownServiceIdentity, err := getOwnServiceIdentity(c, cfg.Signer)
		if err != nil {
			return nil, err
		}
		delegatedIdentity, err := delegation.CheckToken(c, delegation.CheckTokenParams{
			Token:                delegationTok,
			PeerID:               s.peerIdent,
			CertificatesProvider: certsProvider{s.db},
			GroupsChecker:        s.db,
			OwnServiceIdentity:   ownServiceIdentity,
		})
		if err != nil {
			return nil, err
		}
		// User profile information is not available when using delegation, so just
		// wipe it.
		s.user = &User{Identity: delegatedIdentity}
	}

	// Inject auth state.
	return WithState(c, &s), nil
}

// usersAPI returns implementation of UsersAPI by examining Methods. Returns nil
// if none of Methods implement UsersAPI.
func (a Authenticator) usersAPI() UsersAPI {
	for _, m := range a {
		if api, ok := m.(UsersAPI); ok {
			return api
		}
	}
	return nil
}

// LoginURL returns a URL that, when visited, prompts the user to sign in,
// then redirects the user to the URL specified by dest.
func (a Authenticator) LoginURL(c context.Context, dest string) (string, error) {
	if api := a.usersAPI(); api != nil {
		return api.LoginURL(c, dest)
	}
	return "", ErrNoUsersAPI
}

// LogoutURL returns a URL that, when visited, signs the user out,
// then redirects the user to the URL specified by dest.
func (a Authenticator) LogoutURL(c context.Context, dest string) (string, error) {
	if api := a.usersAPI(); api != nil {
		return api.LogoutURL(c, dest)
	}
	return "", ErrNoUsersAPI
}

////

// certsProvider implements delegation.CertificatesProvider.
type certsProvider struct {
	db authdb.DB
}

func (p certsProvider) GetAuthServiceCertificates(c context.Context) (*signing.PublicCertificates, error) {
	serviceURL, err := p.db.GetAuthServiceURL(c)
	if err != nil {
		return nil, err
	}
	return signing.FetchCertificatesFromLUCIService(c, serviceURL)
}

// getOwnServiceIdentity returns 'service:<appID>' identity of the current
// service.
func getOwnServiceIdentity(c context.Context, signer signing.Signer) (identity.Identity, error) {
	if signer == nil {
		return "", ErrNotConfigured
	}
	serviceInfo, err := signer.ServiceInfo(c)
	if err != nil {
		return "", err
	}
	return identity.MakeIdentity("service:" + serviceInfo.AppID)
}
