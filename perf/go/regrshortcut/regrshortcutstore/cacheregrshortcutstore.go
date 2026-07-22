package regrshortcutstore

import (
	"bytes"
	"context"
	"crypto/md5"
	"database/sql"
	"encoding/json"
	"slices"
	"strings"

	"go.opencensus.io/trace"
	"go.skia.org/infra/go/cache"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/perf/go/types"
)

// cacheRegressionsShortcutStore provides an implementation of regrshortcut.Store
// which stores these shortcuts in a cache instead of the database.
// The primary use case is when we connect to prod database on a local instance,
// using multigraph needs write access to the database in order to write the
// regrshortcuts. This store prevents the need to elevate to breakglass by
// using a local cache to store this data.
type cacheRegressionsShortcutStore struct {
	cacheClient cache.Cache
}

// NewCacheRegressionsShortcutStore returns a new instance of cacheRegressionsShortcutStore.
func NewCacheRegressionsShortcutStore(cacheClient cache.Cache) *cacheRegressionsShortcutStore {
	return &cacheRegressionsShortcutStore{
		cacheClient: cacheClient,
	}
}

type cacheKey struct {
	RegrIdList []string
	IsLegacy   bool
}

// Create implements the regrshortcut.Store interface.
func (c *cacheRegressionsShortcutStore) Create(ctx context.Context, regrIdList []string) (string, error) {
	ctx, span := trace.StartSpan(ctx, "cacheRegressionsShortcutStore.Create")
	defer span.End()

	if len(regrIdList) == 0 {
		return "", skerr.Fmt("regression id list cannot be empty")
	}

	slices.Sort(regrIdList)
	shortcut := c.calcHash(regrIdList)

	var buff bytes.Buffer
	ck := cacheKey{RegrIdList: regrIdList, IsLegacy: false}
	err := json.NewEncoder(&buff).Encode(ck)
	if err != nil {
		return "", skerr.Wrapf(err, "Failed to encode regression id list")
	}

	err = c.cacheClient.SetValue(ctx, shortcut, buff.String())
	if err != nil {
		return "", skerr.Wrapf(err, "Failed to set value in cache")
	}

	return shortcut, nil
}

// Get implements the regrshortcut.Store interface.
func (c *cacheRegressionsShortcutStore) Get(ctx context.Context, shortcut string) (sql.NullBool, []string, error) {
	ctx, span := trace.StartSpan(ctx, "cacheRegressionsShortcutStore.Get")
	defer span.End()

	if !strings.HasPrefix(shortcut, "\\x") {
		shortcut = "\\x" + shortcut
	}
	if !c.cacheClient.Exists(shortcut) {
		return sql.NullBool{}, []string{}, nil
	}
	value, err := c.cacheClient.GetValue(ctx, shortcut)
	if err != nil {
		return sql.NullBool{}, nil, skerr.Wrapf(err, "Failed to get value from cache")
	}

	var ck cacheKey
	if err := json.Unmarshal([]byte(value), &ck); err != nil {
		return sql.NullBool{}, nil, skerr.Wrapf(err, "Failed to decode regression id list")
	}

	return sql.NullBool{Valid: true, Bool: ck.IsLegacy}, ck.RegrIdList, nil
}

func (c *cacheRegressionsShortcutStore) calcHash(regrIdList []string) string {
	hash := md5.Sum([]byte(strings.Join(regrIdList, ",")))
	return string(types.TraceIDForSQLFromTraceIDAsBytes(hash[:]))
}
