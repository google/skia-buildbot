// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package prpc

import (
	"sync"

	"github.com/luci/luci-go/server/auth"
)

var defaultAuth = struct {
	sync.RWMutex
	Authenticator auth.Authenticator
}{}

// RegisterDefaultAuth sets a default authenticator that is used unless
// Server.Authenticator is provided.
// Panics if a is nil or called twice.
func RegisterDefaultAuth(a auth.Authenticator) {
	if a == nil {
		panic("a is nil")
	}
	defaultAuth.Lock()
	defer defaultAuth.Unlock()
	if defaultAuth.Authenticator != nil {
		panic("default prpc authenticator is already set")
	}
	defaultAuth.Authenticator = a
}

// GetDefaultAuth returns the default authenticator set by RegisterDefaultAuth
// or nil if not registered.
func GetDefaultAuth() auth.Authenticator {
	defaultAuth.RLock()
	defer defaultAuth.RUnlock()
	return defaultAuth.Authenticator
}
