// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package coordinator

import (
	"sync"
	"time"

	"github.com/luci/luci-go/common/clock"
	"github.com/luci/luci-go/common/config"
	"github.com/luci/luci-go/common/data/caching/lru"
	"github.com/luci/luci-go/common/errors"
	log "github.com/luci/luci-go/common/logging"
	"github.com/luci/luci-go/common/sync/promise"
	"github.com/luci/luci-go/common/tsmon/field"
	"github.com/luci/luci-go/common/tsmon/metric"
	tsmon_types "github.com/luci/luci-go/common/tsmon/types"
	"github.com/luci/luci-go/logdog/common/types"
	"golang.org/x/net/context"
)

const (
	// DefaultSize is the default (maximum) size of the LRU cache.
	DefaultSize = 1024 * 1024

	// DefaultExpiration is the default expiration value.
	DefaultExpiration = 10 * time.Minute
)

var (
	tsCache = metric.NewCounter("logdog/collector/coordinator/cache",
		"Metrics for cache uses, tracking hits and misses.",
		tsmon_types.MetricMetadata{},
		field.Bool("hit"))
)

// cache is a Coordinator interface implementation for the Collector service
// that caches remote results locally.
type cache struct {
	Coordinator

	// Size is the number of stream states to hold in the cache.
	size int

	// expiration is the maximum lifespan of a cache entry. If an entry is older
	// than this, it will be discarded.
	expiration time.Duration

	// cache is the LRU state cache.
	lru *lru.Cache
}

// NewCache creates a new Coordinator instance that wraps another Coordinator
// instance with a cache that retains the latest remote Coordinator state in a
// client-side LRU cache.
//
// If size is <= 0, DefaultSize will be used.
// If expiration is <= 0, DefaultExpiration will be used.
func NewCache(c Coordinator, size int, expiration time.Duration) Coordinator {
	if size <= 0 {
		size = DefaultSize
	}
	if expiration <= 0 {
		expiration = DefaultExpiration
	}

	return &cache{
		Coordinator: c,
		expiration:  expiration,
		lru:         lru.New(size),
	}
}

func (c *cache) getCacheEntry(ctx context.Context, k cacheEntryKey) (*cacheEntry, bool) {
	now := clock.Now(ctx)

	// Get the cacheEntry from our cache. If it is expired or doesn't exist,
	// generate a new cache entry for this key.
	created := false
	entry := c.lru.Mutate(k, func(current interface{}) interface{} {
		// Don't replace an existing entry, unless it has an error or has expired.
		if current != nil {
			curEntry := current.(*cacheEntry)
			if now.Before(curEntry.expiresAt) {
				return current
			}
		}

		created = true
		return &cacheEntry{
			cacheEntryKey: k,
			terminalIndex: -1,
			expiresAt:     now.Add(c.expiration),
		}
	}).(*cacheEntry)
	return entry, created
}

// RegisterStream invokes the wrapped Coordinator's RegisterStream method and
// caches the result. It uses a Promise to cause all simultaneous identical
// RegisterStream requests to block on a single RPC.
func (c *cache) RegisterStream(ctx context.Context, st *LogStreamState, desc []byte) (*LogStreamState, error) {
	entry, created := c.getCacheEntry(ctx, cacheEntryKey{
		project: st.Project,
		path:    st.Path,
	})
	tsCache.Add(ctx, 1, !created)

	st, err := entry.registerStream(ctx, c.Coordinator, *st, desc)
	if err != nil {
		log.WithError(err).Errorf(ctx, "Error retrieving stream state.")
		return nil, err
	}

	return st, nil
}

func (c *cache) TerminateStream(ctx context.Context, r *TerminateRequest) error {
	entry, _ := c.getCacheEntry(ctx, cacheEntryKey{
		project: r.Project,
		path:    r.Path,
	})
	return entry.terminateStream(ctx, c.Coordinator, *r)
}

// cacheEntryKey is the LRU key for a cacheEntry.
type cacheEntryKey struct {
	project config.ProjectName
	path    types.StreamPath
}

// cacheEntry is a cached state for a specific log stream.
//
// It contains promises for each singleton operation: one for stream
// registration (registerP), and one for stream termination (terminateP).
//
// There are three states to promise evaluation:
//	- If the promise is nil, it will be populated. Any concurrent requests
//	  will block pending population (via lock) and will obtain a reference to
//	  the populated promise.
//	- If the promise succeeded, or failed non-transiently, its result will be
//	  retained and all subsequent calls will see this result.
//	- If the promise failed transiently, it will be set to nil. This will cause
//	  the next caller to generate a new promise (retry). Concurrent users of the
//	  transiently-failing Promise will all receive a transient error.
type cacheEntry struct {
	sync.Mutex
	cacheEntryKey

	// expiresAt is the time when this cache entry expires. If this is in the
	// past, this entry can be discarded.
	expiresAt time.Time

	// terminalIndex is the cached terminal index. Valid if >= 0.
	//
	// If a TerminateStream RPC succeeds, we will use this value in our returned
	// RegisterStream state.
	terminalIndex types.MessageIndex

	// registerP is a Promise that is blocking pending stream registration.
	// Upon successful resolution, it will contain a *LogStreamState.
	registerP *promise.Promise
	// terminateP is a Promise that is blocking pending stream termination.
	// Upon successful resolution, it will contain a nil result with no error.
	terminateP *promise.Promise
}

// registerStream performs a RegisterStream Coordinator RPC.
func (ce *cacheEntry) registerStream(ctx context.Context, coord Coordinator, st LogStreamState, desc []byte) (*LogStreamState, error) {
	// Initialize the registration Promise, if one is not defined.
	//
	// While locked, load the current registration promise and the local
	// terminal index value.
	var (
		p    *promise.Promise
		tidx types.MessageIndex
	)
	ce.withLock(func() {
		if ce.registerP == nil {
			ce.registerP = promise.NewDeferred(func(ctx context.Context) (interface{}, error) {
				st, err := coord.RegisterStream(ctx, &st, desc)
				if err == nil {
					// If the remote state has a terminal index, retain it locally.
					ce.loadRemoteTerminalIndex(st.TerminalIndex)
					return st, nil
				}

				return nil, err
			})
		}

		p, tidx = ce.registerP, ce.terminalIndex
	})

	// Resolve our registration Promise.
	remoteStateIface, err := p.Get(ctx)
	if err != nil {
		// If the promise failed transiently, clear it so that subsequent callers
		// will regenerate a new promise. ONLY clear it if it it is the same
		// promise, as different callers may have already cleared/rengerated it.
		if errors.IsTransient(err) {
			ce.withLock(func() {
				if ce.registerP == p {
					ce.registerP = nil
				}
			})
		}
		return nil, err
	}
	remoteState := remoteStateIface.(*LogStreamState)

	// If our remote state doesn't include a terminal index and our local state
	// has recorded a successful remote terminal index, return a copy of the
	// remote state with the remote terminal index added.
	if remoteState.TerminalIndex < 0 && tidx >= 0 {
		remoteStateCopy := *remoteState
		remoteStateCopy.TerminalIndex = tidx
		remoteState = &remoteStateCopy
	}
	return remoteState, nil
}

// terminateStream performs a TerminateStream Coordinator RPC.
func (ce *cacheEntry) terminateStream(ctx context.Context, coord Coordinator, tr TerminateRequest) error {
	// Initialize the termination Promise if one is not defined. Also, grab our
	// cached remote terminal index.
	var (
		p    *promise.Promise
		tidx types.MessageIndex = -1
	)
	ce.withLock(func() {
		if ce.terminateP == nil {
			// We're creating a new promise, so our tr's TerminalIndex will be set.
			ce.terminateP = promise.NewDeferred(func(ctx context.Context) (interface{}, error) {
				// Execute our TerminateStream RPC. If successful, retain the successful
				// terminal index locally.
				err := coord.TerminateStream(ctx, &tr)
				if err == nil {
					// Note that this happens within the Promise body, so this will not
					// conflict with our outer lock.
					ce.loadRemoteTerminalIndex(tr.TerminalIndex)
				}
				return nil, err
			})
		}

		p, tidx = ce.terminateP, ce.terminalIndex
	})

	// If the stream is known to be terminated on the Coordinator side, we don't
	// need to issue another request.
	if tidx >= 0 {
		if tr.TerminalIndex != tidx {
			// Not much we can do here, and this probably will never happen, but let's
			// log it if it does.
			log.Fields{
				"requestIndex": tr.TerminalIndex,
				"cachedIndex":  tidx,
			}.Warningf(ctx, "Request terminal index doesn't match cached value.")
		}
		return nil
	}

	// Resolve our termination Promise.
	if _, err := p.Get(ctx); err != nil {
		// If this is a transient error, delete this Promise so future termination
		// attempts will retry for this stream.
		if errors.IsTransient(err) {
			ce.withLock(func() {
				if ce.terminateP == p {
					ce.terminateP = nil
				}
			})
		}
		return err
	}
	return nil
}

// loadRemoteTerminalIndex updates our cached remote terminal index with tidx,
// if tidx >= 0 and a remote terminal index is not already cached.
//
// This is executed in the bodies of the register and terminate Promises if they
// receive a terminal index remotely.
func (ce *cacheEntry) loadRemoteTerminalIndex(tidx types.MessageIndex) {
	// Never load an invalid remote terminal index.
	if tidx < 0 {
		return
	}

	// Load the remote terminal index if one isn't already loaded.
	ce.withLock(func() {
		if ce.terminalIndex < 0 {
			ce.terminalIndex = tidx
		}
	})
}

func (ce *cacheEntry) withLock(f func()) {
	ce.Lock()
	defer ce.Unlock()
	f()
}
