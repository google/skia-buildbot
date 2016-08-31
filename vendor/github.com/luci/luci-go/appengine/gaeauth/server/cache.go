// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package server

import (
	"time"

	"golang.org/x/net/context"

	"github.com/luci/gae/service/info"
	"github.com/luci/gae/service/memcache"

	"github.com/luci/luci-go/common/errors"
	"github.com/luci/luci-go/server/auth"
)

// Memcache implements auth.GlobalCache on top of GAE memcache.
type Memcache struct {
	Namespace string
}

var _ auth.GlobalCache = (*Memcache)(nil)

// Get returns a cached item or (nil, nil) if it's not in the cache.
//
// Any returned error is transient error.
func (m *Memcache) Get(c context.Context, key string) ([]byte, error) {
	switch itm, err := m.cache(c).Get(key); {
	case err == memcache.ErrCacheMiss:
		return nil, nil
	case err != nil:
		return nil, errors.WrapTransient(err)
	default:
		return itm.Value(), nil
	}
}

// Set unconditionally overwrites an item in the cache.
//
// If 'exp' is zero, the item will have no expiration time.
//
// Any returned error is transient error.
func (m *Memcache) Set(c context.Context, key string, value []byte, exp time.Duration) error {
	cache := m.cache(c)
	item := cache.NewItem(key).SetValue(value).SetExpiration(exp)
	if err := cache.Set(item); err != nil {
		return errors.WrapTransient(err)
	}
	return nil
}

// cache returns properly namespaced memcache.Interface.
func (m *Memcache) cache(c context.Context) memcache.Interface {
	c = info.Get(c).MustNamespace(m.Namespace)
	return memcache.Get(c)
}
