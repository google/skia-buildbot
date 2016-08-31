// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package authdb

import (
	"time"

	"golang.org/x/net/context"

	"github.com/luci/luci-go/common/clock"
	"github.com/luci/luci-go/common/data/caching/lazyslot"
	"github.com/luci/luci-go/common/data/rand/mathrand"
)

// DBCacheUpdater knows how to update local in-memory copy of DB.
//
// Used by NewDBCache.
type DBCacheUpdater func(c context.Context, prev DB) (DB, error)

// NewDBCache returns a provider of DB instances that uses local memory to
// cache DB instances for 5-10 seconds. It uses supplied callback to refetch DB
// from some permanent storage when cache expires.
//
// Even though the return value is technically a function, treat it as a heavy
// stateful object, since it has the cache of DB in its closure.
func NewDBCache(updater DBCacheUpdater) func(c context.Context) (DB, error) {
	cacheSlot := lazyslot.Slot{
		Fetcher: func(c context.Context, prev lazyslot.Value) (lazyslot.Value, error) {
			var prevDB DB
			if prev.Value != nil {
				prevDB = prev.Value.(DB)
			}
			newDB, err := updater(c, prevDB)
			if err != nil {
				return lazyslot.Value{}, err
			}
			expTime := 5*time.Second + time.Duration(mathrand.Get(c).Intn(5000))*time.Millisecond
			return lazyslot.Value{
				Value:      newDB,
				Expiration: clock.Now(c).Add(expTime),
			}, nil
		},
	}

	// Actual factory that just grabs DB from the cache (triggering lazy refetch).
	return func(c context.Context) (DB, error) {
		val, err := cacheSlot.Get(c)
		if err != nil {
			return nil, err
		}
		return val.Value.(DB), nil
	}
}
