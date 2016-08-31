// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

// Package erroring implements config.Interface that simply returns an error.
//
// May be handy as a placeholder in case some more useful implementation is not
// available.
package erroring

import (
	"net/url"

	"golang.org/x/net/context"

	"github.com/luci/luci-go/common/config"
)

// New produces config.Interface instance that returns the given error for all
// calls.
//
// Panics if given err is nil.
func New(err error) config.Interface {
	if err == nil {
		panic("the error must not be nil")
	}
	return erroringImpl{err}
}

type erroringImpl struct {
	err error
}

func (e erroringImpl) ServiceURL(ctx context.Context) url.URL {
	return url.URL{
		Scheme: "error",
	}
}

func (e erroringImpl) GetConfig(ctx context.Context, configSet, path string, hashOnly bool) (*config.Config, error) {
	return nil, e.err
}

func (e erroringImpl) GetConfigByHash(ctx context.Context, contentHash string) (string, error) {
	return "", e.err
}

func (e erroringImpl) GetConfigSetLocation(ctx context.Context, configSet string) (*url.URL, error) {
	return nil, e.err
}

func (e erroringImpl) GetProjectConfigs(ctx context.Context, path string, hashesOnly bool) ([]config.Config, error) {
	return nil, e.err
}

func (e erroringImpl) GetProjects(ctx context.Context) ([]config.Project, error) {
	return nil, e.err
}

func (e erroringImpl) GetRefConfigs(ctx context.Context, path string, hashesOnly bool) ([]config.Config, error) {
	return nil, e.err
}

func (e erroringImpl) GetRefs(ctx context.Context, projectID string) ([]string, error) {
	return nil, e.err
}
