// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

// Package cacheContext implements a context.Context wrapper which caches the
// results of Value calls, speeding up subsequent calls for the same key.
//
// Context values are stored as immutable layers in a backwards-referenced list.
// Each lookup traverses the list from tail to head looking for a layer with the
// requested value. As a Context retains more values, this traversal will get
// more expensive. This class can be used for large Contexts to avoid repeatedly
// paying the cost of traversal for the same read-only value.
package cacheContext

import (
	"sync"

	"golang.org/x/net/context"
)

// cacheContext implements a caching context.
type cacheContext struct {
	// Context is the underlying wrapped Context.
	context.Context

	// RWMutex protects access to the cache member.
	sync.RWMutex

	// cache is a map of Value retrievals.
	//
	// Read access requires a read lock, write access requires a write lock.
	cache map[interface{}]interface{}
}

// Wrap wraps the supplied Context in a caching Context. All Value lookups will
// be cached at this level, avoiding the expense of future Context traversals
// for that same key.
func Wrap(c context.Context) context.Context {
	if _, ok := c.(*cacheContext); ok {
		return c
	}
	return &cacheContext{Context: c}
}

func (c *cacheContext) Value(key interface{}) interface{} {
	// Optimistic: the value is cached.
	c.RLock()
	if v, ok := c.cache[key]; ok {
		c.RUnlock()
		return v
	}
	c.RUnlock()

	// Pessimistic: not in cache, load from Context and cache it.
	v := c.Context.Value(key)

	c.Lock()
	defer c.Unlock()
	if c.cache == nil {
		c.cache = make(map[interface{}]interface{})
	}
	c.cache[key] = v
	return v
}
