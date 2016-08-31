// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package cache

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/luci/luci-go/common/data/text/units"
	"github.com/luci/luci-go/common/isolated"
)

// Cache is a cache of objects.
//
// All implementations must be thread-safe.
type Cache interface {
	io.Closer

	// Keys returns the list of all cached digests in LRU order.
	Keys() isolated.HexDigests

	// Touch updates the LRU position of an item to ensure it is kept in the
	// cache.
	//
	// Returns true if item is in cache.
	Touch(digest isolated.HexDigest) bool

	// Evict removes item from cache if it's there.
	Evict(digest isolated.HexDigest)

	// Add reads data from src and stores it in cache.
	Add(digest isolated.HexDigest, src io.Reader) error

	// Read returns contents of the cached item.
	Read(digest isolated.HexDigest) (io.ReadCloser, error)

	// Hardlink ensures file at |dest| has the same content as cached |digest|.
	//
	// Note that the behavior when dest already exists is undefined. It will work
	// on all POSIX and may or may not fail on Windows depending on the
	// implementation used. Do not rely on this behavior.
	Hardlink(digest isolated.HexDigest, dest string, perm os.FileMode) error
}

// Policies is the policies to use on a cache to limit it's footprint.
//
// It's a cache, not a leak.
type Policies struct {
	// MaxSize trims if the cache gets larger than this value. If 0, the cache is
	// effectively a leak.
	MaxSize units.Size
	// MaxItems is the maximum number of items to keep in the cache. If 0, do not
	// enforce a limit.
	MaxItems int
	// MinFreeSpace trims if disk free space becomes lower than this value. If 0,
	// it unconditionally fills the disk. Only makes sense when using disk based
	// cache.
	//
	// BUG: Implement Policies.MinFreeSpace.
	MinFreeSpace units.Size
}

// NewMemory creates a purely in-memory cache.
func NewMemory(policies Policies) Cache {
	return &memory{
		policies: policies,
		data:     map[isolated.HexDigest][]byte{},
		lru:      makeLRUDict(),
	}
}

// NewDisk creates a disk based cache.
//
// It may return both a valid Cache and an error if it failed to load the
// previous cache metadata. It is safe to ignore this error.
func NewDisk(policies Policies, path string) (Cache, error) {
	if !filepath.IsAbs(path) {
		return nil, errors.New("must use absolute path")
	}
	d := &disk{
		policies: policies,
		path:     path,
		lru:      makeLRUDict(),
	}
	p := d.statePath()
	f, err := os.Open(p)
	if err == nil {
		defer f.Close()
		err = json.NewDecoder(f).Decode(&d.lru)
	} else if os.IsNotExist(err) {
		// The fact that the cache is new is not an error.
		err = nil
	}
	return d, err
}

// Private details.

type memory struct {
	// Immutable.
	policies Policies

	// Lock protected.
	lock sync.Mutex
	data map[isolated.HexDigest][]byte // Contains the actual content.
	lru  lruDict                       // Implements LRU based eviction.
}

func (m *memory) Close() error {
	m.lock.Lock()
	defer m.lock.Unlock()
	return nil
}

func (m *memory) Keys() isolated.HexDigests {
	m.lock.Lock()
	defer m.lock.Unlock()
	return m.lru.keys()
}

func (m *memory) Touch(digest isolated.HexDigest) bool {
	if !digest.Validate() {
		return false
	}
	m.lock.Lock()
	defer m.lock.Unlock()
	if _, ok := m.data[digest]; !ok {
		return false
	}
	m.lru.touch(digest)
	return true
}

func (m *memory) Evict(digest isolated.HexDigest) {
	if !digest.Validate() {
		return
	}
	m.lock.Lock()
	defer m.lock.Unlock()
	delete(m.data, digest)
	m.lru.pop(digest)
}

func (m *memory) Read(digest isolated.HexDigest) (io.ReadCloser, error) {
	if !digest.Validate() {
		return nil, os.ErrInvalid
	}
	m.lock.Lock()
	defer m.lock.Unlock()
	content, ok := m.data[digest]
	if !ok {
		return nil, os.ErrNotExist
	}
	return ioutil.NopCloser(bytes.NewBuffer(content)), nil
}

func (m *memory) Add(digest isolated.HexDigest, src io.Reader) error {
	if !digest.Validate() {
		return os.ErrInvalid
	}
	// TODO(maruel): Use a LimitedReader flavor that fails when reaching limit.
	content, err := ioutil.ReadAll(src)
	if err != nil {
		return err
	}
	if isolated.HashBytes(content) != digest {
		return errors.New("invalid hash")
	}
	if units.Size(len(content)) > m.policies.MaxSize {
		return errors.New("item too large")
	}
	m.lock.Lock()
	defer m.lock.Unlock()
	m.data[digest] = content
	m.lru.pushFront(digest, units.Size(len(content)))
	m.respectPolicies()
	return nil
}

func (m *memory) Hardlink(digest isolated.HexDigest, dest string, perm os.FileMode) error {
	if !digest.Validate() {
		return os.ErrInvalid
	}
	m.lock.Lock()
	content, ok := m.data[digest]
	m.lock.Unlock()
	if !ok {
		return os.ErrNotExist
	}
	return ioutil.WriteFile(dest, content, perm)
}

func (m *memory) respectPolicies() {
	for m.lru.length() > m.policies.MaxItems || m.lru.sum > m.policies.MaxSize {
		k, _ := m.lru.popOldest()
		delete(m.data, k)
	}
}

type disk struct {
	// Immutable.
	policies Policies
	path     string

	// Lock protected.
	lock sync.Mutex
	lru  lruDict // Implements LRU based eviction.
	// TODO(maruel): Add stats about: # added, # removed.
	// TODO(maruel): stateFile
}

func (d *disk) Close() error {
	d.lock.Lock()
	defer d.lock.Unlock()
	if !d.lru.IsDirty() {
		return nil
	}
	f, err := os.Create(d.statePath())
	if err == nil {
		defer f.Close()
		err = json.NewEncoder(f).Encode(&d.lru)
	}
	return err
}

func (d *disk) Keys() isolated.HexDigests {
	d.lock.Lock()
	defer d.lock.Unlock()
	return d.lru.keys()
}

func (d *disk) Touch(digest isolated.HexDigest) bool {
	if !digest.Validate() {
		return false
	}
	d.lock.Lock()
	defer d.lock.Unlock()
	mtime := time.Now()
	if err := os.Chtimes(d.itemPath(digest), mtime, mtime); err != nil {
		return false
	}
	d.lru.touch(digest)
	return true
}

func (d *disk) Evict(digest isolated.HexDigest) {
	if !digest.Validate() {
		return
	}
	d.lock.Lock()
	defer d.lock.Unlock()
	d.lru.pop(digest)
	_ = os.Remove(d.itemPath(digest))
}

func (d *disk) Read(digest isolated.HexDigest) (io.ReadCloser, error) {
	if !digest.Validate() {
		return nil, os.ErrInvalid
	}
	f, err := os.Open(d.itemPath(digest))
	if err != nil {
		return nil, err
	}
	return f, nil
}

func (d *disk) Add(digest isolated.HexDigest, src io.Reader) error {
	if !digest.Validate() {
		return os.ErrInvalid
	}
	p := d.itemPath(digest)
	dst, err := os.Create(p)
	if err != nil {
		return err
	}
	h := isolated.GetHash()
	// TODO(maruel): Use a LimitedReader flavor that fails when reaching limit.
	size, err := io.Copy(dst, io.TeeReader(src, h))
	if err2 := dst.Close(); err == nil {
		err = err2
	}
	if err != nil {
		_ = os.Remove(p)
		return err
	}
	if isolated.Sum(h) != digest {
		_ = os.Remove(p)
		return errors.New("invalid hash")
	}
	if units.Size(size) > d.policies.MaxSize {
		_ = os.Remove(p)
		return errors.New("item too large")
	}

	d.lock.Lock()
	defer d.lock.Unlock()
	d.lru.pushFront(digest, units.Size(size))
	d.respectPolicies()
	return nil
}

func (d *disk) Hardlink(digest isolated.HexDigest, dest string, perm os.FileMode) error {
	if !digest.Validate() {
		return os.ErrInvalid
	}
	src := d.itemPath(digest)
	// - Windows, if dest exists, the call fails. In particular, trying to
	//   os.Remove() will fail if the file's ReadOnly bit is set. What's worse is
	//   that the ReadOnly bit is set on the file inode, shared on all hardlinks
	//   to this inode. This means that in the case of a file with the ReadOnly
	//   bit set, it would have to do:
	//   - If dest exists:
	//    - If dest has ReadOnly bit:
	//      - If file has any other inode:
	//        - Remove the ReadOnly bit.
	//        - Remove dest.
	//        - Set the ReadOnly bit on one of the inode found.
	//   - Call os.Link()
	//  In short, nobody ain't got time for that.
	//
	// - On any other (sane) OS, if dest exists, it is silently overwritten.
	return os.Link(src, dest)
}

func (d *disk) itemPath(digest isolated.HexDigest) string {
	return filepath.Join(d.path, string(digest))
}

func (d *disk) statePath() string {
	return filepath.Join(d.path, "state.json")
}

func (d *disk) respectPolicies() {
	for d.lru.length() > d.policies.MaxItems || d.lru.sum > d.policies.MaxSize {
		k, _ := d.lru.popOldest()
		_ = os.Remove(d.itemPath(k))
	}
}
