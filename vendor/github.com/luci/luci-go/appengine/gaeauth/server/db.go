// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package server

import (
	"errors"
	"net"

	"golang.org/x/net/context"

	"github.com/luci/gae/service/info"
	"github.com/luci/luci-go/appengine/gaeauth/server/internal/authdbimpl"
	"github.com/luci/luci-go/common/clock"
	"github.com/luci/luci-go/common/logging"
	"github.com/luci/luci-go/server/auth/authdb"
	"github.com/luci/luci-go/server/auth/identity"
	"github.com/luci/luci-go/server/secrets"
)

// errNotConfigured is returned on real GAE if auth service URL is not set.
var errNotConfigured = errors.New(
	"Auth Service URL is not configured, you MUST configure it for apps used " +
		"in production, visit /admin/settings/auth_service to do so.")

// GetAuthDB fetches AuthDB snapshot from the datastore and returns authdb.DB
// interface wrapping it.
//
// It may reuse existing one (`prev`), if no changes were made. If `prev` is
// nil, always fetches a new copy from the datastore.
//
// If auth_service URL is not configured, returns special kind of authdb.DB that
// implements some default authorization rules (allow everything on dev server,
// forbid everything and emit errors on real GAE).
func GetAuthDB(c context.Context, prev authdb.DB) (authdb.DB, error) {
	// Grab revision number of most recent snapshot.
	latest, err := authdbimpl.GetLatestSnapshotInfo(c)
	if err != nil {
		return nil, err
	}

	// If auth_service URL is not configured, use default db implementation.
	if latest == nil {
		if info.Get(c).IsDevAppServer() {
			return devServerDB{}, nil
		}
		return authdb.ErroringDB{Error: errNotConfigured}, nil
	}

	// No newer version in the datastore? Reuse what we have in memory. `prev` may
	// be an instance of ErroringDB or devServerDB, so use non-panicking type
	// assertion.
	if prevDB, _ := prev.(*authdb.SnapshotDB); prevDB != nil {
		if prevDB.AuthServiceURL == latest.AuthServiceURL && prevDB.Rev == latest.Rev {
			return prevDB, nil
		}
	}

	// Fetch new snapshot from the datastore. Log how long it takes to keep an
	// eye on performance here, since it has potential to become slow.
	start := clock.Now(c)
	proto, err := authdbimpl.GetAuthDBSnapshot(c, latest.GetSnapshotID())
	if err != nil {
		return nil, err
	}
	db, err := authdb.NewSnapshotDB(proto, latest.AuthServiceURL, latest.Rev)
	logging.Infof(c, "auth: AuthDB at rev %d fetched in %s", latest.Rev, clock.Now(c).Sub(start))
	if err != nil {
		logging.Errorf(c, "auth: AuthDB is invalid - %s", err)
		return nil, err
	}

	return db, nil
}

// devServerDB implements authdb.DB by allowing everything.
//
// It is used on dev server when auth_service URL is not configured. Must not be
// used for real production GAE applications.
type devServerDB struct{}

func (devServerDB) IsAllowedOAuthClientID(c context.Context, email, clientID string) (bool, error) {
	if !info.Get(c).IsDevAppServer() {
		return false, errNotConfigured
	}
	return true, nil
}

func (devServerDB) IsMember(c context.Context, id identity.Identity, group string) (bool, error) {
	if !info.Get(c).IsDevAppServer() {
		return false, errNotConfigured
	}
	return id.Kind() != identity.Anonymous, nil
}

func (devServerDB) SharedSecrets(c context.Context) (secrets.Store, error) {
	return nil, errNotConfigured
}

func (devServerDB) GetWhitelistForIdentity(c context.Context, ident identity.Identity) (string, error) {
	return "", nil
}

func (devServerDB) IsInWhitelist(c context.Context, ip net.IP, whitelist string) (bool, error) {
	return false, nil
}

func (devServerDB) GetAuthServiceURL(c context.Context) (string, error) {
	return "", errNotConfigured
}
