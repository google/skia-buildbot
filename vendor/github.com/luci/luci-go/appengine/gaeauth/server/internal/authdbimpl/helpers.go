// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package authdbimpl

import (
	"golang.org/x/net/context"

	"github.com/luci/gae/filter/txnBuf"
	"github.com/luci/gae/service/datastore"
	"github.com/luci/gae/service/info"
	"github.com/luci/luci-go/server/auth/service"
)

// authService is interface for service.AuthService.
//
// Unit tests inject fake implementation into the testing context.
type authService interface {
	EnsureSubscription(c context.Context, subscription, pushURL string) error
	DeleteSubscription(c context.Context, subscription string) error
	PullPubSub(c context.Context, subscription string) (*service.Notification, error)
	ProcessPubSubPush(c context.Context, body []byte) (*service.Notification, error)
	GetLatestSnapshotRevision(c context.Context) (int64, error)
	GetSnapshot(c context.Context, rev int64) (*service.Snapshot, error)
}

type contextKey int

// setAuthService injects authService implementation into the context.
//
// Used in unit tests.
func setAuthService(c context.Context, s authService) context.Context {
	return context.WithValue(c, contextKey(0), s)
}

// getAuthService returns authService implementation injected into the context
// via setAuthService or *service.AuthService otherwise.
func getAuthService(c context.Context, url string) authService {
	if s, _ := c.Value(contextKey(0)).(authService); s != nil {
		return s
	}
	return &service.AuthService{URL: url}
}

// defaultNS returns GAE context configured to use default namespace.
//
// All publicly callable functions must use it to switch to default namespace.
// All internal functions expect the context to be in the default namespace.
//
// Idempotent.
func defaultNS(c context.Context) context.Context {
	return info.Get(c).MustNamespace("")
}

// noTxnDS returns datastore interface configured to escape any current
// transaction.
func noTxnDS(c context.Context) datastore.Interface {
	return txnBuf.GetNoTxn(c)
}
