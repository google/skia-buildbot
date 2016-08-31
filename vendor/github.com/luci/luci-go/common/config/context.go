// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package config

import (
	"errors"
	"net/url"

	"golang.org/x/net/context"
)

var errNoImpl = errors.New("config: the context doesn't have Interface installed, use SetImplementation")

type contextKey int

// SetImplementation sets the current Interface implementation in the context.
//
// By keeping Interface in the context we allow high level libraries to use
// a config client implementation without explicitly passing it through all
// method calls.
//
// This implementation is also used by package level functions declared below
// (see GetConfig, etc).
func SetImplementation(c context.Context, ri Interface) context.Context {
	return context.WithValue(c, contextKey(0), ri)
}

// GetImplementation returns the Interface object from context.
//
// It should be installed there by 'SetImplementation' already.
//
// Returns nil if it's not there.
func GetImplementation(c context.Context) Interface {
	ret, _ := c.Value(contextKey(0)).(Interface)
	return ret
}

////////////////////////////////////////////////////////////////////////////////
// Functions below mimic Interface API.
//
// They forward calls to an Interface implementation in the context or panic if
// it isn't installed.

// ServiceURL returns the URL of the config service.
//
// The function just forwards the call to corresponding method of Interface
// implementation installed in the context with SetImplementation.
func ServiceURL(ctx context.Context) url.URL {
	impl := GetImplementation(ctx)
	if impl == nil {
		panic(errNoImpl)
	}
	return impl.ServiceURL(ctx)
}

// GetConfig returns a config at a path in a config set or ErrNoConfig
// if missing. If hashOnly is true, returned Config struct has Content set
// to "" (and the call is faster).
//
// The function just forwards the call to corresponding method of Interface
// implementation installed in the context with SetImplementation.
func GetConfig(ctx context.Context, configSet, path string, hashOnly bool) (*Config, error) {
	impl := GetImplementation(ctx)
	if impl == nil {
		panic(errNoImpl)
	}
	return impl.GetConfig(ctx, configSet, path, hashOnly)
}

// GetConfigByHash returns the contents of a config, as identified by its
// content hash, or ErrNoConfig if missing.
//
// The function just forwards the call to corresponding method of Interface
// implementation installed in the context with SetImplementation.
func GetConfigByHash(ctx context.Context, contentHash string) (string, error) {
	impl := GetImplementation(ctx)
	if impl == nil {
		panic(errNoImpl)
	}
	return impl.GetConfigByHash(ctx, contentHash)
}

// GetConfigSetLocation returns the URL location of a config set.
//
// The function just forwards the call to corresponding method of Interface
// implementation installed in the context with SetImplementation.
func GetConfigSetLocation(ctx context.Context, configSet string) (*url.URL, error) {
	impl := GetImplementation(ctx)
	if impl == nil {
		panic(errNoImpl)
	}
	return impl.GetConfigSetLocation(ctx, configSet)
}

// GetProjectConfigs returns all the configs at the given path in all
// projects that have such config. If hashesOnly is true, returned Config
// structs have Content set to "" (and the call is faster).
//
// The function just forwards the call to corresponding method of Interface
// implementation installed in the context with SetImplementation.
func GetProjectConfigs(ctx context.Context, path string, hashesOnly bool) ([]Config, error) {
	impl := GetImplementation(ctx)
	if impl == nil {
		panic(errNoImpl)
	}
	return impl.GetProjectConfigs(ctx, path, hashesOnly)
}

// GetProjects returns all the registered projects in the configuration
// service.
//
// The function just forwards the call to corresponding method of Interface
// implementation installed in the context with SetImplementation.
func GetProjects(ctx context.Context) ([]Project, error) {
	impl := GetImplementation(ctx)
	if impl == nil {
		panic(errNoImpl)
	}
	return impl.GetProjects(ctx)
}

// GetRefConfigs returns the config at the given path in all refs of all
// projects that have such config. If hashesOnly is true, returned Config
// structs have Content set to "" (and the call is faster).
//
// The function just forwards the call to corresponding method of Interface
// implementation installed in the context with SetImplementation.
func GetRefConfigs(ctx context.Context, path string, hashesOnly bool) ([]Config, error) {
	impl := GetImplementation(ctx)
	if impl == nil {
		panic(errNoImpl)
	}
	return impl.GetRefConfigs(ctx, path, hashesOnly)
}

// GetRefs returns the list of refs for a project.
//
// The function just forwards the call to corresponding method of Interface
// implementation installed in the context with SetImplementation.
func GetRefs(ctx context.Context, projectID string) ([]string, error) {
	impl := GetImplementation(ctx)
	if impl == nil {
		panic(errNoImpl)
	}
	return impl.GetRefs(ctx, projectID)
}
