// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package gaeconfig

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"golang.org/x/net/context"

	"github.com/luci/gae/service/info"
	"github.com/luci/luci-go/common/config"
	"github.com/luci/luci-go/common/config/impl/erroring"
	"github.com/luci/luci-go/common/config/impl/filesystem"
	"github.com/luci/luci-go/common/config/impl/remote"
	"github.com/luci/luci-go/server/auth"
)

// ErrNotConfigured is returned by methods of config.Interface object returned
// by New if config service URL is not set. Usually happens for new apps.
var ErrNotConfigured = errors.New("config service URL is not set in settings")

// devCfgDir is a name of the directory with config files when running in
// local dev appserver model. See New for details.
const devCfgDir = "devcfg"

// implCache is used to avoid reallocating config.Interface implementation
// during each request.
//
// config.Interface instances don't actually depend on the context and thus we
// can share them between requests.
//
// Cache key is the current value of Settings struct. We don't bother to clean
// up old entries, since in most cases there'll be none (the settings are mostly
// static).
var implCache struct {
	lock  sync.RWMutex
	cache map[Settings]config.Interface
}

// New constructs default luci-config client.
//
// The client is configured to use luci-config URL specified in the settings,
// using GAE app service account for authentication.
//
// If running in prod, and the settings don't specify luci-config URL, produces
// an implementation of config.Interface that returns ErrNotConfigured from all
// methods.
//
// If running on devserver, and the settings don't specify luci-config URL,
// returns a filesystem-based implementation that reads configs from a directory
// (or a symlink) named 'devcfg' located in the GAE module directory (where
// app.yaml is) or its immediate parent directory.
//
// If such directory can not be located, produces an implementation of
// config.Interface that returns errors from all methods.
//
// Panics if it can't load the settings (should not happen since they are in
// the local memory cache usually).
func New(c context.Context) config.Interface {
	settings, err := FetchCachedSettings(c)
	if err != nil {
		panic(err)
	}

	if settings.ConfigServiceURL == "" {
		if inf := info.Get(c); inf.IsDevAppServer() {
			return devServerConfig()
		}
		return erroring.New(ErrNotConfigured)
	}

	return configImplForSettings(settings)
}

// configImplForSettings returns config.Interface based on given settings.
//
// Split out from New to enforce that returned config.Interface doesn't depend
// on the context (since this function doesn't accept a context).
func configImplForSettings(settings Settings) config.Interface {
	implCache.lock.RLock()
	impl, ok := implCache.cache[settings]
	implCache.lock.RUnlock()
	if ok {
		return impl
	}

	implCache.lock.Lock()
	defer implCache.lock.Unlock()

	if impl, ok := implCache.cache[settings]; ok {
		return impl
	}

	impl = remote.New(settings.ConfigServiceURL+"/_ah/api/config/v1/", authenticatedClient)
	if settings.CacheExpirationSec != 0 {
		impl = WrapWithCache(impl, time.Duration(settings.CacheExpirationSec)*time.Second)
	}

	if implCache.cache == nil {
		implCache.cache = make(map[Settings]config.Interface, 1)
	}
	implCache.cache[settings] = impl

	return impl
}

// devServerConfig returns config.Interface to use on a devserver.
//
// See New for details.
func devServerConfig() config.Interface {
	pwd := os.Getenv("PWD") // os.Getwd works funny with symlinks, use PWD
	candidates := []string{
		filepath.Join(pwd, devCfgDir),
		filepath.Join(filepath.Dir(pwd), devCfgDir),
	}
	for _, dir := range candidates {
		if _, err := os.Stat(dir); err == nil {
			fs, err := filesystem.New(dir)
			if err != nil {
				return erroring.New(err)
			}
			return fs
		}
	}
	return erroring.New(fmt.Errorf("luci-config: could not find local configs in any of %s", candidates))
}

// authenticatedClient returns http.Client to use for making authenticated
// request to the config service.
//
// The returned client uses GAE app's service account for authentication.
func authenticatedClient(ctx context.Context) (*http.Client, error) {
	t, err := auth.GetRPCTransport(ctx, auth.AsSelf)
	if err != nil {
		return nil, err
	}
	return &http.Client{Transport: t}, nil
}
