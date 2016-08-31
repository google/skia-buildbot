// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package auth

import (
	"net/http"
	"time"

	"golang.org/x/net/context"

	"github.com/luci/luci-go/common/auth"
	"github.com/luci/luci-go/server/auth/authdb"
	"github.com/luci/luci-go/server/auth/signing"
)

// Config contains global configuration of the auth library.
//
// It lives in the context and must be installed there by some root middleware.
//
// It contains concrete implementations of various interfaces used by
// the library.
type Config struct {
	// DBProvider is a callback that returns most recent DB instance.
	//
	// DB represents a snapshot of user groups used for authorization checks.
	DBProvider func(c context.Context) (authdb.DB, error)

	// Signer possesses the service's private key and can sign blobs with it.
	//
	// Signer interface also responsible for providing identity of the currently
	// running service (since it's related to the private key used).
	Signer signing.Signer

	// AccessTokenProvider knows how to generate OAuth2 access token for the
	// service account belonging to the server itself.
	//
	// Should implement caching itself, if appropriate. Returned tokens are
	// expected to live for at least 1-2 mins.
	AccessTokenProvider func(c context.Context, scopes []string) (auth.Token, error)

	// AnonymousTransport returns http.RoundTriper that can make unauthenticated
	// HTTP requests.
	//
	// The returned round tripper is assumed to be bound to the context and won't
	// outlive it.
	AnonymousTransport func(c context.Context) http.RoundTripper

	// GlobalCache implements service-global strongly consistent cache.
	//
	// Usually backed by memcache. Should do namespacing itself (i.e. the auth
	// library assumes full ownership of the keyspace).
	GlobalCache GlobalCache
}

// GlobalCache implements service-global strongly consistent cache.
type GlobalCache interface {
	// Get returns a cached item or (nil, nil) if it's not in the cache.
	//
	// Any returned error is transient error.
	Get(c context.Context, key string) ([]byte, error)

	// Set unconditionally overwrites an item in the cache.
	//
	// If 'exp' is zero, the item will have no expiration time.
	//
	// Any returned error is transient error.
	Set(c context.Context, key string, value []byte, exp time.Duration) error
}

type cfxContextKey int

// SetConfig replaces the configuration in the context.
func SetConfig(c context.Context, cfg Config) context.Context {
	return context.WithValue(c, cfxContextKey(0), &cfg)
}

// GetConfig returns the config stored in the context (or nil if not there).
//
// Do not modify it. If you want to change the config, make a copy and install
// the copy in the context (or use ModifyConfig that does the same thing).
func GetConfig(c context.Context) *Config {
	val, _ := c.Value(cfxContextKey(0)).(*Config)
	return val
}

// ModifyConfig is a shortcut for making a context with a derived configuration.
//
// It grabs current configuration from the context, makes its copy, calls the
// callback to modify the copy, and puts it into a new context.
func ModifyConfig(c context.Context, cb func(cfg *Config)) context.Context {
	var cfg Config
	if cur := GetConfig(c); cur != nil {
		cfg = *cur
	}
	cb(&cfg)
	return SetConfig(c, cfg)
}

////////////////////////////////////////////////////////////////////////////////
// Helpers that extract stuff from the config.

// GetDB returns most recent snapshot of authorization database using DBProvider
// installed in the context via 'SetConfig'
//
// If no factory is installed, returns DB that forbids everything and logs
// errors. It is often good enough for unit tests that do not care about
// authorization, and still not horribly bad if accidentally used in production.
func GetDB(c context.Context) (authdb.DB, error) {
	if cfg := GetConfig(c); cfg != nil && cfg.DBProvider != nil {
		return cfg.DBProvider(c)
	}
	return authdb.ErroringDB{Error: ErrNotConfigured}, nil
}

// GetSigner extracts Signer from the context. Returns nil if no Signer is set.
func GetSigner(c context.Context) signing.Signer {
	if cfg := GetConfig(c); cfg != nil {
		return cfg.Signer
	}
	return nil
}
