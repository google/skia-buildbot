// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package secrets

import (
	"errors"

	"golang.org/x/net/context"
)

var (
	// ErrNoStoreConfigured is returned by GetSecret if secret store is not in
	// the context.
	ErrNoStoreConfigured = errors.New("secrets.Store is not in the context")
)

// Factory knows how to make a new Store.
type Factory func(context.Context) Store

type contextKey int

// Get grabs a Store by calling Factory stored in the context. If one hasn't
// been set, it returns nil.
func Get(c context.Context) Store {
	if f, ok := c.Value(contextKey(0)).(Factory); ok && f != nil {
		return f(c)
	}
	return nil
}

// SetFactory sets the function to produce Store instances when Get(c) is used.
func SetFactory(c context.Context, f Factory) context.Context {
	return context.WithValue(c, contextKey(0), f)
}

// Set injects the Store object in the context to be returned by Get as is.
func Set(c context.Context, s Store) context.Context {
	if s == nil {
		return SetFactory(c, nil)
	}
	return SetFactory(c, func(context.Context) Store { return s })
}

// GetSecret is shortcut for grabbing a Store from the context and using its
// GetSecret method. If the context doesn't have Store set,
// returns ErrNoStoreConfigured.
func GetSecret(c context.Context, k Key) (Secret, error) {
	if s := Get(c); s != nil {
		return s.GetSecret(k)
	}
	return Secret{}, ErrNoStoreConfigured
}
