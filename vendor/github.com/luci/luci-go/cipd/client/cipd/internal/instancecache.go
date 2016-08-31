// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package internal

import (
	"bytes"
	"container/heap"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"os"
	"sync"
	"time"

	"golang.org/x/net/context"

	"github.com/luci/luci-go/common/data/stringset"
	"github.com/luci/luci-go/common/logging"
	"github.com/luci/luci-go/common/proto/google"

	"github.com/luci/luci-go/cipd/client/cipd/common"
	"github.com/luci/luci-go/cipd/client/cipd/internal/messages"
	"github.com/luci/luci-go/cipd/client/cipd/local"
)

const (
	instanceCacheMaxSize = 300
	// instanceCacheSyncInterval determines the frequency of
	// synchronization of state.db with instance files in the cache dir.
	instanceCacheSyncInterval  = 8 * time.Hour
	instanceCacheStateFilename = "state.db"
)

// InstanceCache is a file-system-based, thread-safe, LRU cache of instances.
//
// Does not validate instance hashes; it is caller's responsibility.
type InstanceCache struct {
	fs        local.FileSystem
	stateLock sync.Mutex // synchronizes access to the state file.
}

// NewInstanceCache initializes InstanceCache.
//
// fs will be the root of the cache.
func NewInstanceCache(fs local.FileSystem) *InstanceCache {
	return &InstanceCache{fs: fs}
}

// Get searches for the instance in the cache and writes its contents to output.
//
// If the instance is not found, returns an os.IsNotExists error without writing
// to output.
func (c *InstanceCache) Get(ctx context.Context, pin common.Pin, output io.Writer, now time.Time) error {
	if err := common.ValidatePin(pin); err != nil {
		return err
	}

	path, err := c.fs.RootRelToAbs(pin.InstanceID)
	if err != nil {
		return fmt.Errorf("invalid instance ID %q", pin.InstanceID)
	}

	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	c.withState(ctx, now, func(s *messages.InstanceCache) {
		touch(s, pin.InstanceID, now)
	})

	_, err = io.Copy(output, f)
	return err
}

// Put caches an instance.
//
// write must write the instance contents. May remove some instances from the
// cache that were not accessed for a long time.
func (c *InstanceCache) Put(ctx context.Context, pin common.Pin, now time.Time, write func(*os.File) error) error {
	if err := common.ValidatePin(pin); err != nil {
		return err
	}
	path, err := c.fs.RootRelToAbs(pin.InstanceID)
	if err != nil {
		return fmt.Errorf("invalid instance ID %q", pin.InstanceID)
	}

	if err := c.fs.EnsureFile(ctx, path, write); err != nil {
		return err
	}

	c.withState(ctx, now, func(s *messages.InstanceCache) {
		touch(s, pin.InstanceID, now)
		c.gc(ctx, s)
	})
	return nil
}

type timeHeap []time.Time

func (h timeHeap) Len() int           { return len(h) }
func (h timeHeap) Less(i, j int) bool { return h[i].Before(h[j]) }
func (h timeHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }
func (h *timeHeap) Push(x interface{}) {
	*h = append(*h, x.(time.Time))
}
func (h *timeHeap) Pop() interface{} {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[0 : n-1]
	return x
}

// gc checks if the number of instances in the state is greater than maximum.
// If yes, purges excessive oldest instances.
func (c *InstanceCache) gc(ctx context.Context, state *messages.InstanceCache) {
	garbageSize := len(state.Entries) - instanceCacheMaxSize
	if garbageSize <= 0 {
		return
	}

	// Compute cutoff date by putting all access times to a heap
	// and pop from it garbageSize times.
	lastAccessTimes := make(timeHeap, 0, len(state.Entries))
	for _, s := range state.Entries {
		lastAccessTimes = append(lastAccessTimes, s.LastAccess.Time())
	}
	heap.Init(&lastAccessTimes)
	for i := 0; i < garbageSize-1; i++ {
		heap.Pop(&lastAccessTimes)
	}
	cutOff := heap.Pop(&lastAccessTimes).(time.Time)

	// First garbageSize instances that were last accessed on or before cutOff are garbage.
	garbage := make([]string, 0, garbageSize)
	// Map iteration is not deterministic, but it is fine.
	for id, e := range state.Entries {
		if !e.LastAccess.Time().After(cutOff) {
			garbage = append(garbage, id)
			if len(garbage) == cap(garbage) {
				break
			}
		}
	}

	collected := 0
	for _, id := range garbage {
		path, err := c.fs.RootRelToAbs(id)
		if err != nil {
			panic("impossible")
		}
		if c.fs.EnsureFileGone(ctx, path) != nil {
			// EnsureFileGone logs errors.
			continue
		}
		delete(state.Entries, id)
		collected++
	}
	logging.Infof(ctx, "cipd: instance cache collected %d instances", collected)
}

// readState loads cache state from the state file.
// If the file does not exist, corrupted or its state was not synchronized
// with the instance files for a long time, synchronizes it.
// Newly discovered files are considered last accessed at zero time.
// If synchronization fails, then the state is considered empty.
func (c *InstanceCache) readState(ctx context.Context, state *messages.InstanceCache, now time.Time) {
	statePath, err := c.fs.RootRelToAbs(instanceCacheStateFilename)
	if err != nil {
		panic("impossible")
	}

	stateBytes, err := ioutil.ReadFile(statePath)
	sync := false
	switch {
	case os.IsNotExist(err):
		sync = true

	case err != nil:
		logging.Warningf(ctx, "cipd: could not read instance cache - %s", err)
		sync = true

	default:
		if err := UnmarshalWithSHA1(stateBytes, state); err != nil {
			logging.Warningf(ctx, "cipd: instance cache file is corrupted - %s", err)
			*state = messages.InstanceCache{}
			sync = true
		} else {
			cutOff := now.
				Add(-instanceCacheSyncInterval).
				Add(time.Duration(rand.Int63n(int64(5 * time.Minute))))
			sync = state.LastSynced.Time().Before(cutOff)
		}
	}

	if sync {
		if err := c.syncState(ctx, state, now); err != nil {
			logging.Warningf(ctx, "cipd: failed to sync instance cache - %s", err)
		}
		c.gc(ctx, state)
	}
}

// syncState synchronizes the list of instances in the state file with instance files.
// Preserves lastAccess of existing instances. Newly discovered files are
// considered last accessed at zero time.
func (c *InstanceCache) syncState(ctx context.Context, state *messages.InstanceCache, now time.Time) error {
	root, err := os.Open(c.fs.Root())
	switch {

	case os.IsNotExist(err):
		state.Entries = nil

	case err != nil:
		return err

	default:
		instanceIDs, err := root.Readdirnames(0)
		if err != nil {
			return err
		}
		existingIDs := stringset.New(len(instanceIDs))
		for _, id := range instanceIDs {
			if common.ValidateInstanceID(id) != nil {
				continue
			}
			existingIDs.Add(id)

			if _, ok := state.Entries[id]; !ok {
				if state.Entries == nil {
					state.Entries = map[string]*messages.InstanceCache_Entry{}
				}
				state.Entries[id] = &messages.InstanceCache_Entry{}
			}
		}

		for id := range state.Entries {
			if !existingIDs.Has(id) {
				delete(state.Entries, id)
			}
		}
	}

	state.LastSynced = google.NewTimestamp(now)
	logging.Infof(ctx, "cipd: synchronized instance cache with instance files")
	return nil
}

// saveState persists the cache state.
func (c *InstanceCache) saveState(ctx context.Context, state *messages.InstanceCache) error {
	stateBytes, err := MarshalWithSHA1(state)
	if err != nil {
		return err
	}

	statePath, err := c.fs.RootRelToAbs(instanceCacheStateFilename)
	if err != nil {
		panic("impossible")
	}

	return local.EnsureFile(ctx, c.fs, statePath, bytes.NewReader(stateBytes))
}

// withState loads cache state from the state file, calls f and saves it back.
// See also readState.
func (c *InstanceCache) withState(ctx context.Context, now time.Time, f func(*messages.InstanceCache)) {
	c.stateLock.Lock()
	defer c.stateLock.Unlock()

	state := &messages.InstanceCache{}

	start := time.Now()
	c.readState(ctx, state, now)
	loadSaveTime := time.Since(start)

	f(state)

	start = time.Now()
	if err := c.saveState(ctx, state); err != nil {
		logging.Warningf(ctx, "cipd: could not save instance cache - %s", err)
	}
	loadSaveTime += time.Since(start)

	if loadSaveTime > time.Second {
		logging.Warningf(
			ctx, "cipd: loading and saving instance cache with %d entries took %s",
			len(state.Entries), loadSaveTime)
	}
}

// getAccessTime returns last access time of an instance.
// Used for testing.
func (c *InstanceCache) getAccessTime(ctx context.Context, now time.Time, pin common.Pin) (lastAccess time.Time, ok bool) {
	c.withState(ctx, now, func(s *messages.InstanceCache) {
		var entry *messages.InstanceCache_Entry
		if entry, ok = s.Entries[pin.InstanceID]; ok {
			lastAccess = entry.LastAccess.Time()
		}
	})
	return
}

// touch updates/adds last access time for an instance.
func touch(state *messages.InstanceCache, instanceID string, now time.Time) {
	entry := state.Entries[instanceID]
	if entry == nil {
		entry = &messages.InstanceCache_Entry{}
		if state.Entries == nil {
			state.Entries = map[string]*messages.InstanceCache_Entry{}
		}
		state.Entries[instanceID] = entry
	}
	entry.LastAccess = google.NewTimestamp(now)
}
