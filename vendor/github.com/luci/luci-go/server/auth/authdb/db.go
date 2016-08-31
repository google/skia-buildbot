// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package authdb

import (
	"net"

	"golang.org/x/net/context"

	"github.com/luci/luci-go/server/auth/identity"
	"github.com/luci/luci-go/server/secrets"
)

// DB is interface to access a database of authorization related information.
//
// It is static read only object that represent snapshot of auth data at some
// moment in time.
type DB interface {
	// IsAllowedOAuthClientID returns true if given OAuth2 client_id can be used
	// to authenticate access for given email.
	IsAllowedOAuthClientID(c context.Context, email, clientID string) (bool, error)

	// IsMember returns true if the given identity belongs to the given group.
	//
	// Unknown groups are considered empty. May return errors if underlying
	// datastore has issues.
	IsMember(c context.Context, id identity.Identity, group string) (bool, error)

	// SharedSecrets is secrets.Store with secrets in Auth DB.
	//
	// Such secrets are usually generated on central Auth Service and are known
	// to all trusted services (so that they can use them to exchange data).
	SharedSecrets(c context.Context) (secrets.Store, error)

	// GetWhitelistForIdentity returns name of the IP whitelist to use to check
	// IP of requests from given `ident`.
	//
	// It's used to restrict access for certain account to certain IP subnets.
	//
	// Returns ("", nil) if `ident` is not IP restricted.
	GetWhitelistForIdentity(c context.Context, ident identity.Identity) (string, error)

	// IsInWhitelist returns true if IP address belongs to given named
	// IP whitelist.
	//
	// IP whitelist is a set of IP subnets. Unknown IP whitelists are considered
	// empty. May return errors if underlying datastore has issues.
	IsInWhitelist(c context.Context, ip net.IP, whitelist string) (bool, error)

	// GetAuthServiceURL returns root URL ("https://<host>") of the auth service.
	//
	// Returns an error if the DB implementation is not using an auth service. May
	// also return transient errors.
	GetAuthServiceURL(c context.Context) (string, error)
}
