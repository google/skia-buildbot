package isolate_cache

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"cloud.google.com/go/bigtable"
	"go.chromium.org/luci/common/isolated"
	"go.skia.org/infra/go/atomic_miss_cache"
	"go.skia.org/infra/go/isolate"
	"go.skia.org/infra/task_scheduler/go/types"
	"golang.org/x/oauth2"
	"google.golang.org/api/option"
)

const (
	BT_TABLE         = "isolated_cache"
	BT_COLUMN_FAMILY = "ISOLATE"
	BT_COLUMN        = "ISO"
)

var (
	// Fully-qualified BigTable column name.
	BT_COLUMN_FULL = fmt.Sprintf("%s:%s", BT_COLUMN_FAMILY, BT_COLUMN)
)

// CachedValue maps isolate filenames to isolated hashes.
type CachedValue struct {
	// Isolated maps isolate filenames to IsolatedFiles.
	Isolated map[string]*isolated.Isolated

	// Error stores a permanent error. Mutually-exclusive with Isolated.
	Error string
}

// backingCache provides lookups for CachedValues in BigTable.
type backingCache struct {
	table *bigtable.Table
}

// Get retrieves the given entry from BigTable
func (c *backingCache) Get(ctx context.Context, key string) (atomic_miss_cache.Value, error) {
	var rv *CachedValue
	var rvErr error
	if err := c.table.ReadRows(ctx, bigtable.PrefixRange(key), func(row bigtable.Row) bool {
		for _, ri := range row[BT_COLUMN_FAMILY] {
			if ri.Column == BT_COLUMN_FULL {
				// We'd like to use gob encoding since it's faster, but we can't
				// because of https://github.com/golang/go/issues/11119.
				var cv CachedValue
				rvErr = json.Unmarshal(ri.Value, &cv)
				if rvErr == nil {
					rv = &cv
				}
				return false
			}
		}
		return true
	}); err != nil {
		return nil, err
	}
	if rvErr != nil {
		return nil, rvErr
	}
	if rv == nil {
		return nil, atomic_miss_cache.ErrNoSuchEntry
	}
	return rv, nil
}

// Set writes the given entry to BigTable.
func (c *backingCache) Set(ctx context.Context, key string, value atomic_miss_cache.Value) error {
	// We'd like to use gob encoding since it's faster, but we can't
	// because of https://github.com/golang/go/issues/11119.
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(value.(*CachedValue)); err != nil {
		return err
	}
	mut := bigtable.NewMutation()
	mut.Set(BT_COLUMN_FAMILY, BT_COLUMN, bigtable.ServerTime, buf.Bytes())
	return c.table.Apply(ctx, key, mut)
}

// Delete is a no-op, since we don't delete from BigTable.
func (c *backingCache) Delete(ctx context.Context, key string) error {
	return nil
}

// Cache maintains a cache of IsolatedFiles which is backed by BigTable.
type Cache struct {
	cache  *atomic_miss_cache.AtomicMissCache
	client *bigtable.Client
}

// New returns an isolated cache.
func New(ctx context.Context, btProject, btInstance string, ts oauth2.TokenSource) (*Cache, error) {
	client, err := bigtable.NewClient(ctx, btProject, btInstance, option.WithTokenSource(ts))
	if err != nil {
		return nil, fmt.Errorf("Failed to create BigTable client: %s", err)
	}
	table := client.Open(BT_TABLE)
	return &Cache{
		cache: atomic_miss_cache.New(&backingCache{
			table: table,
		}),
		client: client,
	}, nil
}

// Get returns the cached IsolatedFile for the given RepoState and isolated
// file, if it exists, or ErrNoSuchEntry otherwise.
func (c *Cache) Get(ctx context.Context, rs types.RepoState, isolateFile string) (*isolated.Isolated, error) {
	val, err := c.cache.Get(ctx, rs.RowKey())
	if err != nil {
		return nil, err
	}
	cv := val.(*CachedValue)
	if cv.Error != "" {
		return nil, errors.New(cv.Error)
	}
	rv, ok := cv.Isolated[isolateFile]
	if !ok {
		return nil, atomic_miss_cache.ErrNoSuchEntry
	}
	return isolate.CopyIsolated(rv), nil
}

// Set sets the cached IsolatedFiles. Currently only used for testing since we
// don't generally want to overwrite existing entries.
func (c *Cache) Set(ctx context.Context, rs types.RepoState, cv *CachedValue) error {
	return c.cache.Set(ctx, rs.RowKey(), cv)
}

// SetIfUnset sets the cached IsolatedFiles by calling the given function if
// they do not yet exist in the cache.
func (c *Cache) SetIfUnset(ctx context.Context, rs types.RepoState, fn func(context.Context) (*CachedValue, error)) error {
	_, err := c.cache.SetIfUnset(ctx, rs.RowKey(), func(ctx context.Context) (atomic_miss_cache.Value, error) {
		cv, err := fn(ctx)
		return cv, err
	})
	return err
}

// Close cleans up the resources used by the Cache.
func (c *Cache) Close() error {
	return c.client.Close()
}
